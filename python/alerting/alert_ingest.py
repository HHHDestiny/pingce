"""告警接入与降噪 Agent —— 对接已有监控系统（夜莺 / Prometheus + AlertManager）

## 架构定位（重要）

采集、存储、阈值判定全部由**已有的监控系统**完成，本模块不做任何采集。

```
夜莺 / AlertManager（已有）
        │  webhook POST
        ▼
AlertIngestAgent（本模块）
  1. 标准化 —— 不同监控系统的 payload 统一成 AlertEvent
  2. 降噪   —— 指纹收敛（重复告警）+ 时序二次确认（三算法投票）
  3. 富化   —— 关联知识图谱，补上拓扑位置与近期变更，供 RCA 直接消费
        │  aiops.alerts
        ▼
  RCA → Heal → Change
```

## 为什么要"二次确认"

上游监控系统多用静态阈值（CPU > 80%），在业务波动期误报很多。
本模块在告警进来后，反查时序数据做一次复核：
- 三算法投票通过 → 确认是真异常，放行到 RCA
- 投票不通过   → 判定为瞬时抖动/周期波动，直接抑制
这样既不用替换现有监控系统，又能把噪音挡在进入根因分析之前。

## 依赖

- 必需：numpy
- 可选：httpx（真实拉取 Prometheus 时序；未安装时使用 mock 模式，本地可跑）
"""

import logging
import time
from typing import Any, Optional

from agents.base_agent import BaseAgent
from agents.monitor_agent import AlertFingerprint
from core.event_bus import EventBus
from models.events import (
    AgentType,
    AlertEvent,
    IncidentState,
    Severity,
)
from models.time_series import EnsembleDetector, generate_demo_metrics

logger = logging.getLogger(__name__)


# --------------------------------------------------------------------------
# 1. 告警标准化：兼容夜莺与 AlertManager 两种主流格式
# --------------------------------------------------------------------------

# 夜莺（Nightingale）severity：1=紧急 2=警告 3=提醒
NIGHTINGALE_SEVERITY = {1: Severity.CRITICAL, 2: Severity.HIGH, 3: Severity.MEDIUM}

# AlertManager / Prometheus 风格：字符串
GENERIC_SEVERITY = {
    "critical": Severity.CRITICAL,
    "fatal": Severity.CRITICAL,
    "p0": Severity.CRITICAL,
    "warning": Severity.HIGH,
    "warn": Severity.HIGH,
    "p1": Severity.HIGH,
    "info": Severity.LOW,
    "notice": Severity.LOW,
    "p2": Severity.LOW,
}


def _parse_tags(raw: dict) -> dict[str, str]:
    """夜莺的 tags 是 ['k=v', ...]，AlertManager 的 labels 是 {'k': 'v'}

    两种都支持，统一解析成 dict。
    """
    tags: dict[str, str] = {}

    labels = raw.get("labels") or raw.get("commonLabels") or {}
    if isinstance(labels, dict):
        tags.update({str(k): str(v) for k, v in labels.items()})

    for item in raw.get("tags") or []:
        if isinstance(item, str) and "=" in item:
            k, v = item.split("=", 1)
            tags[k.strip()] = v.strip()

    return tags


def parse_upstream_alert(raw: dict) -> dict[str, Any]:
    """把上游告警 payload 解析成统一结构

    兼容字段差异：夜莺用 rule_name / trigger_value / res_ident，
    AlertManager 用 alertname / value / labels。
    """
    tags = _parse_tags(raw)

    rule_name = (
        raw.get("rule_name")
        or raw.get("alertname")
        or tags.get("alertname")
        or "unknown_alert"
    )

    # 受影响服务：夜莺放 res_ident，AlertManager 在 labels 里
    service = (
        raw.get("res_ident")
        or tags.get("service")
        or tags.get("app")
        or tags.get("job")
        or "unknown"
    )

    # 触发值
    raw_value = raw.get("trigger_value", raw.get("value", 0))
    try:
        value = float(str(raw_value).strip().rstrip("%"))
    except (TypeError, ValueError):
        value = 0.0

    # 严重度
    sev_raw = raw.get("severity", tags.get("severity"))
    severity = Severity.MEDIUM
    if isinstance(sev_raw, int):
        severity = NIGHTINGALE_SEVERITY.get(sev_raw, Severity.MEDIUM)
    elif isinstance(sev_raw, str):
        severity = GENERIC_SEVERITY.get(sev_raw.lower(), Severity.MEDIUM)

    # 触发时间（秒级时间戳）
    trigger_time = int(raw.get("trigger_time") or raw.get("startsAt_ts") or time.time())

    # 用于反查时序的 PromQL：优先用上游带过来的，否则按指标名拼一个
    promql = (
        raw.get("promql")
        or raw.get("expr")
        or f'{tags.get("__name__", rule_name)}{{service="{service}"}}'
    )

    return {
        "rule_name": rule_name,
        "service": service,
        "value": value,
        "severity": severity,
        "trigger_time": trigger_time,
        "promql": promql,
        "labels": tags,
        "description": raw.get("description") or raw.get("annotations", {}).get("description", "")
        if isinstance(raw.get("annotations"), dict)
        else raw.get("description", ""),
        "source": raw.get("cate") or raw.get("source") or "nightingale",
    }


# --------------------------------------------------------------------------
# 2. 时序拉取：二次确认的数据来源
# --------------------------------------------------------------------------

class MetricFetcher:
    """从已有监控系统反查时序数据

    生产：调 Prometheus / 夜莺的 query_range 兼容接口，取告警时刻前一段窗口的数据
    本地：mock 模式造一段带趋势+周期的基线数据，无需任何外部依赖
    """

    def __init__(
        self,
        prometheus_url: Optional[str] = None,
        mock: bool = True,
        timeout: float = 5.0,
    ):
        self._prometheus_url = prometheus_url
        self._mock = mock or not prometheus_url
        self._timeout = timeout

    async def fetch_series(
        self,
        promql: str,
        end_ts: int,
        window_sec: int = 1800,
        step: int = 30,
    ) -> list[float]:
        """拉取告警时刻前 window_sec 的时序点（不含当前点）

        返回按时间升序的 float 列表；失败或空返回 []，由调用方降级处理。
        """
        if self._mock:
            n_points = max(window_sec // step, 30)
            values, _ = generate_demo_metrics(n_points=n_points, inject_anomaly=False)
            return values

        try:
            import httpx
        except ImportError:
            logger.warning("[MetricFetcher] httpx 未安装，退回 mock 数据")
            values, _ = generate_demo_metrics(n_points=60, inject_anomaly=False)
            return values

        params = {
            "query": promql,
            "start": end_ts - window_sec,
            "end": end_ts,
            "step": f"{step}s",
        }
        try:
            async with httpx.AsyncClient(timeout=self._timeout) as client:
                resp = await client.get(
                    f"{self._prometheus_url}/api/v1/query_range", params=params
                )
                resp.raise_for_status()
                result = resp.json().get("data", {}).get("result", [])
                if not result:
                    return []
                return [float(point[1]) for point in result[0].get("values", [])]
        except Exception as e:
            logger.error(f"[MetricFetcher] 拉取时序失败: {e}")
            return []


# --------------------------------------------------------------------------
# 3. 告警接入 Agent
# --------------------------------------------------------------------------

class AlertIngestAgent(BaseAgent):
    """告警接入与降噪 Agent

    处理流程：解析 → 指纹收敛 → 时序二次确认 → 图谱富化 → 发布
    """

    def __init__(
        self,
        event_bus: EventBus,
        fetcher: Optional[MetricFetcher] = None,
        knowledge_graph=None,
        min_history: int = 10,
        window_sec: int = 1800,
        step: int = 30,
        fingerprint_window: int = 300,
    ):
        super().__init__(AgentType.MONITOR, event_bus, "AlertIngestAgent")
        self._fetcher = fetcher or MetricFetcher(mock=True)
        self._kg = knowledge_graph
        self._ensemble = EnsembleDetector(min_votes=2)
        self._fingerprint = AlertFingerprint(window_seconds=fingerprint_window)
        self._min_history = min_history
        self._window_sec = window_sec
        self._step = step

    async def process(self, state: IncidentState) -> IncidentState:
        raw = state.metadata.get("raw_alert")
        if not raw:
            logger.warning("[AlertIngestAgent] 没有收到上游告警 payload")
            return state

        parsed = parse_upstream_alert(raw)
        logger.info(
            f"[AlertIngestAgent] 收到告警: {parsed['rule_name']} "
            f"on {parsed['service']} value={parsed['value']}"
        )

        # ---- 步骤 1：指纹收敛，300s 内相同告警只放一条 ----
        fingerprint = self._fingerprint.compute(
            parsed["rule_name"], parsed["service"], parsed["labels"]
        )
        if self._fingerprint.is_duplicate(fingerprint):
            state.metadata["ingest_decision"] = {
                "action": "suppressed",
                "reason": "duplicate_fingerprint",
                "fingerprint": fingerprint,
            }
            logger.info("[AlertIngestAgent] 重复告警，已抑制")
            return state

        # ---- 步骤 2：时序二次确认（三算法投票）----
        history = await self._fetcher.fetch_series(
            parsed["promql"], parsed["trigger_time"], self._window_sec, self._step
        )
        decision = self._confirm(history, parsed["value"], parsed["severity"])

        # ---- 步骤 3：图谱富化 ----
        enrichment = self._enrich(parsed["service"])

        state.metadata["ingest_decision"] = decision
        state.metadata["enrichment"] = enrichment

        if decision["action"] == "suppressed":
            logger.info(
                f"[AlertIngestAgent] 二次确认未通过，判定为抖动: "
                f"votes={decision['votes']} score={decision['score']}"
            )
            return state

        # ---- 步骤 4：确认通过，生成标准告警事件 ----
        alert = AlertEvent(
            alert_name=parsed["rule_name"],
            severity=parsed["severity"],
            source=parsed["source"],
            target_service=parsed["service"],
            metric_name=parsed["labels"].get("__name__", parsed["rule_name"]),
            metric_value=parsed["value"],
            threshold=decision["score"],
            description=parsed["description"]
            or f"{parsed['rule_name']} on {parsed['service']}: {parsed['value']}",
            labels=parsed["labels"],
            correlation_id=state.incident_id,
        )
        alert.metadata["confirm"] = decision
        alert.metadata["enrichment"] = enrichment

        state.alert_event = alert
        state.status = "investigating"

        await self.event_bus.publish("aiops.alerts", alert)
        logger.info(
            f"[AlertIngestAgent] 告警已确认并发布: {alert.alert_name} "
            f"[{alert.severity.value}] votes={decision['votes']}"
        )
        return state

    def _confirm(
        self, history: list[float], current: float, upstream_severity: Severity
    ) -> dict[str, Any]:
        """时序二次确认

        三种结果：
        - confirmed_by_vote：三算法投票通过
        - confirmed_by_upstream：历史数据不足，降级采信上游告警
        - suppressed：投票未通过，判定为抖动
        """
        if len(history) < self._min_history:
            return {
                "action": "confirmed_by_upstream",
                "votes": None,
                "score": None,
                "reason": f"历史样本不足({len(history)}<{self._min_history})，采信上游判定",
            }

        is_anomaly, score, results = self._ensemble.detect(history, current)
        votes = sum(1 for r in results if r.is_anomaly)

        if is_anomaly:
            return {
                "action": "confirmed_by_vote",
                "votes": votes,
                "score": round(score, 3),
                "reason": f"三算法投票通过 {votes}/3",
                "detail": [
                    {"algorithm": r.algorithm, "hit": bool(r.is_anomaly), "score": float(r.score)}
                    for r in results
                ],
            }

        # 投票未通过，但上游判为紧急时仍然放行 —— 宁可多查一次，不可漏掉 P0
        if upstream_severity == Severity.CRITICAL:
            return {
                "action": "confirmed_by_upstream",
                "votes": votes,
                "score": round(score, 3),
                "reason": "投票未通过但上游判定为 CRITICAL，放行人工复核",
            }

        return {
            "action": "suppressed",
            "votes": votes,
            "score": round(score, 3),
            "reason": f"投票未通过 {votes}/3，判定为瞬时抖动",
            "detail": [
                {"algorithm": r.algorithm, "hit": bool(r.is_anomaly), "score": float(r.score)}
                for r in results
            ],
        }

    def _enrich(self, service: str) -> dict[str, Any]:
        """关联知识图谱，补出拓扑位置和近期变更"""
        if not self._kg:
            return {}
        try:
            return {
                "dependencies": self._kg.get_dependencies(service),
                "depended_by": self._kg.get_dependents(service),
                "impact_score": round(self._kg.compute_impact_score(service), 3),
                "recent_changes": self._kg.find_recent_changes(service),
            }
        except Exception as e:
            logger.warning(f"[AlertIngestAgent] 图谱富化失败: {e}")
            return {}


def create_ingest_agent(
    event_bus: EventBus,
    knowledge_graph=None,
    prometheus_url: Optional[str] = None,
    mock: bool = True,
) -> AlertIngestAgent:
    """工厂方法：创建告警接入 Agent"""
    fetcher = MetricFetcher(prometheus_url=prometheus_url, mock=mock)
    return AlertIngestAgent(event_bus, fetcher=fetcher, knowledge_graph=knowledge_graph)
