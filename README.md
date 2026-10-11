# 🤖 Multi-Agent AIOps — 多 Agent 智能运维系统

<div align="center">

**企业级多 Agent 智能运维平台 | Python · Java · Go 三语言实现**

**面向大厂面试的完整项目：代码 + 架构文档 + 八股文 + 简历模板 + STAR 话术**

[![Python](https://img.shields.io/badge/Python-3.11+-blue?logo=python)](python/)
[![Java](https://img.shields.io/badge/Java-21-orange?logo=openjdk)](java/)
[![Go](https://img.shields.io/badge/Go-1.23-cyan?logo=go)](golang/)
[![License](https://img.shields.io/badge/License-MIT-green)](LICENSE)

</div>

---

## 📖 这个项目是什么？

你是否遇到过这些问题：
- 运维团队每天被 **200+ 条告警**淹没，大部分是误报
- 服务出了问题，排查根因要花 **40 分钟**
- 凌晨 3 点被电话叫醒，半睡半醒地排查故障

本项目用 **5 个 AI Agent 协作**（1 个告警接入 + 检测 / 根因 / 自愈 / 审批 4 个处理 Agent），
自动完成从「告警接入降噪」到「故障修复」的全流程，目标是把 MTTR（平均修复时间）从 40 分钟压到 5 分钟。

```
告警来了
  ↓  Agent 1：这是真的异常吗？（时序分析，过滤误报）
  ↓  Agent 2：根因在哪里？（知识图谱推理）
  ↓  Agent 3：怎么修复？能自动执行吗？（安全护栏 + 分级策略）
  ↓  Agent 4：这个操作风险有多大？需要审批吗？（风险评分）
  ✅ 5 分钟内修复完成，全程自动
```

> **适用人群**：准备腾讯/美团/字节/阿里 AIOps 方向面试的同学、想学多 Agent 系统设计的开发者、对智能运维感兴趣的 SRE 工程师

---

## 🎯 面试价值

| 方向 | 对应招聘岗位 | 薪资范围 |
|------|------------|---------|
| Python + LLM Agent | 腾讯 AI Agent 开发工程师 | 30-60k · 15薪 |
| Java 后端 | 美团/阿里 智能运维平台后端 | 25-50k · 16薪 |
| Go 云原生 | 字节/腾讯 SRE/运维平台 | 25-45k · 15薪 |

> AI Agent 工程师岗位需求同比增长 **380%**（2026年数据）

---

## 🏗️ 系统架构

### 整体架构图

```
┌──────────────────────────────────────────────────────────────┐
│          已有监控系统（已存在，不是本项目做的事）               │
│   夜莺 / Prometheus + AlertManager：采集 · 存储 · 阈值判定     │
│   Loki（日志）  Jaeger（链路）  CMDB                          │
└───────────────────────────┬──────────────────────────────────┘
                            │ webhook POST（告警事件 JSON）
                   ┌────────▼─────────┐
                   │  ① 告警接入 Agent  │ ← 标准化 · 指纹去重
                   │   AlertIngest    │   时序二次确认 · 图谱富化
                   └────────┬─────────┘
                            │ 确认后的告警（未通过的在此抑制）
                   ┌────────▼─────────┐
                   │  事件总线 Kafka   │  ← 解耦所有 Agent
                   │  aiops.alerts    │
                   │  aiops.events    │
                   │  aiops.commands  │
                   └────────┬─────────┘
                            │
┌───────────────────────────▼──────────────────────────────────┐
│                   Agent 编排器 Orchestrator                    │
│                  （状态机 · 条件路由 · 检查点恢复）              │
│                                                              │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌──────────┐  │
│  │ 监控告警   │→ │ 根因分析   │→ │ 故障自愈   │→ │ 变更审批  │  │
│  │  Agent   │  │  Agent   │  │  Agent   │  │  Agent  │  │
│  │           │  │           │  │           │  │          │  │
│  │3-Sigma    │  │知识图谱   │  │Playbook   │  │风险评分  │  │
│  │EWMA       │  │BFS遍历   │  │dry-run    │  │L0/L1/L2  │  │
│  │IsoForest  │  │贝叶斯推理 │  │熔断器     │  │审计日志  │  │
│  └───────────┘  └───────────┘  └───────────┘  └──────────┘  │
└───────────────────────────┬──────────────────────────────────┘
                            │
┌───────────────────────────▼──────────────────────────────────┐
│                      知识层 / 数据源                          │
│      Neo4j 知识图谱    向量数据库 RAG    规则引擎              │
│   （服务拓扑 + 依赖关系 + 历史故障）                           │
└──────────────────────────────────────────────────────────────┘
```

### 一次故障的完整处理流程

```
1. 已有监控系统判定 order-service CPU 95%，webhook 推给本系统
2. 告警接入 Agent：标准化 payload → 指纹去重 → 反查时序做二次确认
                   三算法投票通过才放行，否则判定为抖动直接抑制
3. 根因分析 Agent：查知识图谱，发现 order-service 今天有新部署
                    贝叶斯推理：P(部署引起|CPU高) = 0.54
4. 故障自愈 Agent：匹配 rollback Playbook（L1级），dry-run 通过
5. 变更审批 Agent：风险评分 0.16，oncall 自动审批
6. 执行回滚，5 分钟内恢复正常 ✅
```

> **边界说明**：本项目不做采集。第 1 步的采集、存储、阈值判定全部由已有的
> 夜莺 / Prometheus + AlertManager 完成，本系统只负责告警进来之后的分析与处置。
> 外部告警进来时编排器会**跳过**监控告警 Agent（它只在本地 Demo 模式下生成模拟告警）。

---

## 🚀 快速开始（5 分钟跑起来）

### 方式一：最简运行（零外部依赖，推荐小白）

```bash
# 克隆项目
git clone https://github.com/bcefghj/multi-agent-aiops.git
cd multi-agent-aiops/python

# 安装依赖（只装必要的，不需要 Kafka/Neo4j）
pip install fastapi uvicorn pydantic pydantic-settings numpy scikit-learn structlog

# 启动服务
python -m uvicorn api.main:app --reload --port 8000
```

浏览器打开 → **http://localhost:8000/docs**，你会看到完整的 API 文档界面

### 方式二：完整运行（含 Kafka + Neo4j + Grafana）

```bash
cd python

# 一键启动所有基础设施
docker-compose up -d

# 等待约 30 秒服务就绪，然后启动主服务
pip install -r requirements.txt
python -m uvicorn api.main:app --reload --port 8000
```

各组件地址：
- **API 文档**: http://localhost:8000/docs
- **Neo4j 浏览器**: http://localhost:7474 （账号 neo4j / aiops_password）
- **Prometheus**: http://localhost:9090
- **Grafana**: http://localhost:3000 （账号 admin / aiops_admin）

### 方式三：一键 Demo 演示

```bash
cd python
python -c "import asyncio; from core.orchestrator import run_demo; asyncio.run(run_demo())"
```

输出示例：
```
============================================================
  故障处理结果
============================================================
  事件 ID:    a8f3c2d1-...
  状态:       resolved

  [告警]
    名称:     high_cpu_usage
    严重度:   high
    服务:     order-service
    指标值:   95.3

  [根因分析]
    根因:     近期代码部署引入性能退化
    置信度:   0.54
    影响链:   order-service → payment-service → mysql-primary
    建议动作: rollback, profiling

  [自愈]
    操作:     rollback
    级别:     L1（需 oncall 确认）
    爆炸半径: 0.15
    Dry-run:  DRY-RUN OK: kubectl rollout undo deployment/order-service

  [审批]
    状态:     approved
    风险分:   0.158
    审批人:   oncall-engineer
    原因:     L1 approved: risk=0.158

  [工作流节点状态]
    monitor      → completed
    rca          → completed
    heal         → completed
    change       → completed
============================================================
```

### 方式四：跑异常检测评测（只要 numpy）

```bash
cd python
pip install numpy            # scikit-learn 可选，缺失时自动降级
python benchmarks/anomaly_eval.py
```

不需要起服务、不需要 Kafka/Neo4j，直接输出四种检测策略的精确率 / 召回率 / 误报率 / F1，
用于验证「多算法投票 vs 静态阈值」到底提升多少（结果见[核心技术亮点 1](#1-时序异常检测--多算法投票)）。

---

## 📁 项目结构详解

```
multi-agent-aiops/
│
├── 📄 README.md                     ← 你现在看的这个文件
│
├── 🐍 python/                       ← Python 实现（推荐先看这个）
│   ├── agents/
│   │   ├── base_agent.py            ← Agent 抽象基类（模板方法模式）
│   │   ├── monitor_agent.py         ← 监控告警 Agent
│   │   ├── rca_agent.py             ← 根因分析 Agent
│   │   ├── heal_agent.py            ← 故障自愈 Agent
│   │   └── change_agent.py          ← 变更审批 Agent
│   ├── alerting/
│   │   └── alert_ingest.py          ← 告警接入 Agent（生产入口，降噪 + 二次确认）
│   ├── benchmarks/
│   │   └── anomaly_eval.py          ← 异常检测评测脚本（实测 P/R/FPR/F1）
│   ├── core/
│   │   ├── orchestrator.py          ← Agent 编排器（状态机）⭐ 最核心
│   │   ├── event_bus.py             ← 事件总线（Kafka + 内存双模）⭐
│   │   └── knowledge_graph.py       ← 知识图谱（Neo4j + 内存双模）⭐
│   ├── models/
│   │   ├── events.py                ← 所有事件的数据模型
│   │   └── time_series.py           ← 时序异常检测算法
│   ├── api/
│   │   └── main.py                  ← FastAPI 接口入口
│   ├── config/
│   │   └── settings.py              ← 全局配置
│   ├── docker-compose.yml           ← 一键启动基础设施
│   └── requirements.txt             ← Python 依赖
│
├── ☕ java/                          ← Java 实现（Spring Boot 3）
│   ├── src/main/java/com/aiops/
│   │   ├── agent/                   ← 4 个 Agent 实现
│   │   ├── core/                    ← 编排器 + REST Controller
│   │   └── model/                   ← 数据模型
│   ├── pom.xml                      ← Maven 依赖
│   └── src/main/resources/
│       └── application.yml          ← Spring 配置
│
├── 🐹 golang/                       ← Go 实现（Gin + goroutine）
│   ├── cmd/server/main.go           ← 程序入口
│   ├── internal/
│   │   ├── agent/                   ← 4 个 Agent 实现
│   │   ├── core/                    ← 编排器
│   │   └── model/                   ← 数据模型
│   └── go.mod                       ← Go 依赖
│
└── 📚 docs/                         ← 文档（面试必看）
    ├── architecture.md              ← 架构设计文档
    ├── project-plan.md              ← 项目规划
    ├── explainer/
    │   └── 实现原理详解.md           ← 按实际架构整理的原理说明与职责边界
    ├── interview/                   ← 🎯 面试材料
    │   ├── resume-template.md       ← 简历模板（3 个版本）
    │   ├── star-method.md           ← STAR 话术（含追问应对）
    │   ├── baguwen.md               ← 八股文 25 道
    │   └── interview-qa.md          ← 面试 Q&A 80+ 道
    └── tutorial/                   ← 📖 从 0 到部署教程
        ├── 01-getting-started.md    ← 入门：理解项目结构
        ├── 02-event-driven.md       ← 事件驱动架构详解
        ├── 03-agent-design.md       ← Agent 设计模式
        ├── 04-knowledge-graph.md    ← 知识图谱详解
        └── 05-deploy.md             ← 生产部署指南
```

---

## 🔌 API 接口说明

启动后访问 http://localhost:8000/docs 查看完整文档，核心接口：

### 告警 webhook 入口（生产环境真实入口）

```bash
# 夜莺 / AlertManager 把告警推到这里
curl -X POST http://localhost:8000/api/v1/alerts/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "rule_name": "cpu_usage_high",
    "severity": 1,
    "res_ident": "order-service",
    "trigger_value": "95.3",
    "trigger_time": 1730000000,
    "tags": ["__name__=cpu_usage_percent", "service=order-service"]
  }'
```

返回示例（二次确认未通过时会直接 `suppressed`，不进入后续流程）：
```json
{
  "incident_id": "b7f0e2a4-...",
  "status": "resolved",
  "ingest": {
    "action": "confirmed_by_vote",
    "votes": 3,
    "score": 5.412,
    "reason": "三算法投票通过 3/3"
  },
  "enrichment": {
    "dependencies": ["payment-service", "inventory-service"],
    "impact_score": 0.091,
    "recent_changes": [{"type": "deploy", "time": "2026-10-11T09:12:00"}]
  },
  "rca": {"root_cause": "近期代码部署引入性能退化", "confidence": 0.54},
  "heal": {"action": "rollback", "level": "L1", "dry_run": true}
}
```

`ingest.action` 取值：

| 取值 | 含义 | 后续动作 |
|------|------|---------|
| `confirmed_by_vote` | 三算法投票通过，确认为真异常 | 进入 RCA → Heal → Change |
| `confirmed_by_upstream` | 历史样本不足，或投票未通过但上游判 CRITICAL | 放行人工复核 |
| `suppressed` | 投票未通过判定为抖动，或指纹重复 | 直接抑制，不进入后续流程 |

### 触发故障处理流程（本地 Demo）

```bash
# 使用 Demo 数据（模拟 CPU 告警）
curl -X POST http://localhost:8000/api/v1/incidents/trigger

# 传入自定义指标数据
curl -X POST http://localhost:8000/api/v1/incidents/trigger \
  -H "Content-Type: application/json" \
  -d '{
    "metric_name": "cpu_usage_percent",
    "metric_value": 95.3,
    "target_service": "order-service"
  }'
```

### 查看服务拓扑

```bash
# 查看全部拓扑
curl http://localhost:8000/api/v1/topology

# 查看某服务的依赖链
curl http://localhost:8000/api/v1/topology/order-service/dependencies
```

返回示例：
```json
{
  "service": "order-service",
  "direct_dependencies": ["payment-service", "inventory-service", "user-service"],
  "depended_by": ["api-gateway"],
  "impact_score": 0.091,
  "dependency_paths": [
    ["order-service", "payment-service", "mysql-primary"],
    ["order-service", "inventory-service", "elasticsearch"]
  ]
}
```

### 运行异常检测 Demo

```bash
curl http://localhost:8000/api/v1/anomaly/demo
```

返回示例：
```json
{
  "total_points": 200,
  "anomalies_detected": 3,
  "anomalies": [
    {"index": 150, "value": 82.45, "score": 4.21, "actual_anomaly": true},
    {"index": 170, "value": 19.83, "score": 3.87, "actual_anomaly": true}
  ]
}
```

---

## 💡 核心技术亮点

### 1. 时序异常检测 — 多算法投票

```python
# 三种算法同时检测，至少 2 种认为异常才报警（减少误报）
3-Sigma    → 检测突变（CPU 突然飙高）
EWMA       → 检测趋势（CPU 缓慢上涨）
Isolation Forest → 检测多维异常（多个指标同时异常）

投票结果：2/3 投票 = 报警
```

#### 实测数据（不是估值）

跑 `python benchmarks/anomaly_eval.py` 可复现，数据集 600 点、注入 18 个异常、seed=42：

```
策略                      TP    FP    FN       精确率       召回率       误报率       F1
--------------------------------------------------------------------------
静态阈值 (>80)               8    59    10    11.94%    44.44%    11.09%    0.188
单算法 3-Sigma              6     0    12   100.00%    33.33%     0.00%    0.500
单算法 EWMA                17     2     1    89.47%    94.44%     0.38%    0.919
三算法投票 (>=2)             17     0     1   100.00%    94.44%     0.00%    0.971
投票·全量基线                 14     0     4   100.00%    77.78%     0.00%    0.875
```

相对静态阈值基线：**误报率 11.09% → 0.00%，F1 0.188 → 0.971**。

> 口径：精确率 = 报出的告警中真异常的比例；误报率 = 正常点被误报的比例。
> 评测用滚动窗口基线（最近 120 点），最后一行是全量历史基线对照组——用全量历史会把
> 趋势和季节性算进 std，阈值被拉宽导致召回掉到 77.78%，这也是选滚动基线的原因。
> 注：这是在合成数据集上的自测结果，不是生产环境指标。

> 📖 算法详解：[docs/tutorial/03-agent-design.md](docs/tutorial/03-agent-design.md)

### 2. 知识图谱根因分析

```python
# 服务拓扑（Neo4j 存储）
order-service
  ├── depends on → payment-service → mysql-primary
  ├── depends on → inventory-service（有近期变更！）
  └── depends on → user-service

# 贝叶斯推理
P(部署引起|CPU高) = P(CPU高|部署引起) × P(部署引起) / P(CPU高)
                  = 0.9 × 0.3 / 0.5 = 0.54 ← 54% 置信度
```

> 📖 知识图谱详解：[docs/tutorial/04-knowledge-graph.md](docs/tutorial/04-knowledge-graph.md)

### 3. 分级自愈 + 安全护栏

```
L0（全自动）：重启 Pod、扩容、限流  ← 爆炸半径 < 5%，直接执行
L1（半自动）：回滚版本、改配置      ← 需要 oncall 确认
L2（人工介入）：数据库操作、全量回滚 ← 需要 TL 审批

安全护栏链：
dry-run 模拟 → 爆炸半径 < 20%？→ 熔断器开着？→ 审批通过？→ 执行
```

> 📖 Agent 设计详解：[docs/tutorial/03-agent-design.md](docs/tutorial/03-agent-design.md)

### 4. 事件驱动架构

```python
# Kafka Topic 设计
aiops.alerts    ← MonitorAgent 发布告警
aiops.events    ← RCAAgent 发布分析结果
aiops.commands  ← HealAgent 发布修复命令
aiops.audit     ← ChangeAgent 发布审批记录
*.dlq           ← 死信队列（消费失败的消息）

# 双模支持：本地开发用内存队列，生产用 Kafka
event_bus = create_event_bus(use_kafka=False)  # 本地开发
event_bus = create_event_bus(use_kafka=True)   # 生产环境
```

> 📖 事件驱动详解：[docs/tutorial/02-event-driven.md](docs/tutorial/02-event-driven.md)

---

## 🌐 三语言实现对比

| 特性 | 🐍 Python | ☕ Java | 🐹 Go |
|------|-----------|---------|-------|
| Agent 抽象 | `ABC` 抽象基类 | `abstract class` | `interface` 隐式实现 |
| 异步模型 | `asyncio` 协程 | `CompletableFuture` | `goroutine + channel` |
| 类型安全 | 弱（运行时） | 强（编译时） | 强（编译时） |
| 并发模型 | 单线程事件循环 | 线程池 | M:N goroutine 调度 |
| 适用场景 | AI/ML 密集 | 企业后端 | 云原生/运维平台 |
| 部署体积 | ~200MB | ~150MB（含JRE） | ~15MB（静态链接） |
| 启动时间 | 2-3s | 5-10s | < 0.5s |

**选哪个版本？**
- 准备 **AI/大模型/Python 岗**：看 [python/](python/) 目录
- 准备 **Java 后端岗**：看 [java/](java/) 目录
- 准备 **Go/云原生/SRE 岗**：看 [golang/](golang/) 目录

---

## 🎓 面试准备材料（全套）

| 材料 | 内容 | 链接 |
|------|------|------|
| 简历模板 | Python/Java/Go 三套，含量化数据 | [resume-template.md](docs/interview/resume-template.md) |
| STAR 话术 | 1分钟版 + 3分钟版 + 追问应对 | [star-method.md](docs/interview/star-method.md) |
| 八股文 | 25 道核心考点，含标准答案 | [baguwen.md](docs/interview/baguwen.md) |
| 面试 Q&A | 80+ 道高频问题 + 回答思路 | [interview-qa.md](docs/interview/interview-qa.md) |
| 架构文档 | 系统设计 + 技术选型决策 | [architecture.md](docs/architecture.md) |

### 八股文核心考点预览

<details>
<summary>📋 点击展开 — 25 道核心八股文目录</summary>

**事件驱动架构（必考）**
- 事件驱动 vs RPC 的区别？
- 背压（Backpressure）怎么处理？
- 最终一致性怎么保证？

**Kafka（必考）**
- 如何保证消息不丢？（三个环节）
- 消费组的工作原理？
- Exactly-Once 语义怎么实现？
- 死信队列是什么？

**Agent 框架**
- Orchestrator vs Choreography 编排模式？
- LangGraph 的核心概念？
- ReAct 范式是什么？

**时序异常检测**
- 3-Sigma / EWMA / Isolation Forest 各自适用场景？
- 动态阈值 vs 静态阈值？
- 多算法投票机制的原理？

**知识图谱**
- 图数据库 vs 关系型数据库？
- BFS vs DFS 在根因分析中怎么选？
- 贝叶斯推理在根因分析中怎么用？

**安全与可靠性**
- 熔断器模式的三个状态？
- 爆炸半径怎么评估？
- dry-run 怎么实现？

**分布式系统（通用）**
- CAP 定理？
- Saga 模式？

**设计模式（Java 重点）**
- 项目用了哪些设计模式？（6 种）

**Go 特色**
- goroutine vs Java Thread？
- sync.Map vs map + Mutex？

</details>

### STAR 话术预览

<details>
<summary>💬 点击展开 — 1 分钟版面试话术</summary>

> "这是一个基于事件驱动的多 Agent AIOps 系统，目前处于 POC / 灰度验证阶段。
> 告警接入 Agent 负责对接已有的夜莺 / Prometheus，做标准化、指纹去重和时序二次确认，
> 后面四个 Agent 分别负责根因分析、故障自愈、变更审批和检测兜底，
> 通过 Kafka 事件总线异步协作。
> 技术亮点是用 Neo4j 知识图谱做根因推理，
> 多算法投票做异常检测，分级策略加安全护栏做自愈。
> 降噪效果我做了实测：在 600 点的标注数据集上，相对静态阈值，
> 误报率从 11.09% 降到 0，F1 从 0.188 提到 0.971——这是合成数据集的自测结果，
> 不是生产指标，我会如实说明口径。"

详细版（3 分钟）和追问应对脚本见 → [star-method.md](docs/interview/star-method.md)

</details>

---

## 📚 从零开始学习路径

推荐按以下顺序学习：

```
第 1 天：理解架构
  ├── 读 README（本文件）
  └── 读 docs/tutorial/01-getting-started.md

第 2 天：跑起来
  ├── 按快速开始运行 Python 版
  └── 用 curl 测试各接口，观察输出

第 3 天：看核心代码
  ├── python/models/events.py     ← 理解数据流
  ├── python/core/orchestrator.py ← 理解编排逻辑
  └── python/agents/rca_agent.py  ← 最复杂的 Agent

第 4 天：理解技术原理
  ├── docs/tutorial/02-event-driven.md
  ├── docs/tutorial/03-agent-design.md
  └── docs/tutorial/04-knowledge-graph.md

第 5-7 天：面试准备
  ├── docs/interview/baguwen.md   ← 八股文
  ├── docs/interview/star-method.md  ← 话术
  └── docs/interview/interview-qa.md ← Q&A
```

---

## ⚙️ 技术栈全览

| 层次 | 🐍 Python 版 | ☕ Java 版 | 🐹 Go 版 |
|------|-------------|-----------|---------|
| **Agent 框架** | LangGraph 状态机 | 自建状态机 + Spring | goroutine + channel |
| **API 层** | FastAPI + Pydantic | Spring Boot 3 | Gin |
| **事件总线** | confluent-kafka | Spring Cloud Stream | sarama |
| **知识图谱** | Neo4j + py2neo | Spring Data Neo4j | neo4j-go-driver |
| **时序分析** | Prophet + scikit-learn | Commons Math | gonum |
| **向量数据库** | ChromaDB（RAG） | — | — |
| **监控** | Prometheus Client | Micrometer | — |
| **容器化** | Docker Compose | Docker Compose | Docker Compose |
| **配置管理** | pydantic-settings | application.yml | 环境变量 |

---

## 🤝 参考的企业级开源项目

本项目设计时参考了以下真实的企业级项目：

| 项目 | 公司/组织 | 技术特点 |
|------|---------|---------|
| [HolmesGPT](https://github.com/holmesgpt/holmesgpt) | CNCF Sandbox | Agentic Loop + 多数据源根因分析 |
| [Aurora](https://github.com/arvo-ai/aurora) | Arvo AI | LangGraph + Memgraph 知识图谱 |
| [Microsoft AIOpsLab](https://github.com/microsoft/AIOpsLab) | Microsoft | AIOps Agent 评测框架 |
| [Auto-Agent-K8s](https://github.com/supersaiyane/auto-agent-k8s) | 社区 | K8s 故障自动修复 |
| [Self-Healing SRE](https://github.com/jalpatel11/Self-Healing-SRE-Agent) | 社区 | LangGraph 自愈 SRE |

---

## 📄 License

MIT License — 可自由用于学习、修改和面试展示

---

<div align="center">

**如果这个项目对你有帮助，欢迎 ⭐ Star 支持一下！**

</div>
