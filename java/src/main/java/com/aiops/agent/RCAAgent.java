package com.aiops.agent;

import com.aiops.model.*;
import org.springframework.stereotype.Component;

import java.util.*;

/**
 * 根因分析 Agent — Java 实现
 *
 * 面试要点（Java 特色）：
 * - 使用 Map 嵌套模拟知识图谱（生产环境用 Spring Data Neo4j）
 * - BFS 用 LinkedList 作为队列
 * - 贝叶斯推理：P(Cause|Evidence) = P(Evidence|Cause) * P(Cause) / P(Evidence)
 */
@Component
public class RCAAgent extends BaseAgent {

    private static final Map<String, Map<String, Object>> SERVICE_TOPOLOGY = Map.of(
        "order-service", Map.of(
            "depends_on", List.of("payment-service", "inventory-service", "user-service"),
            "recent_changes", List.of("deploy v2.3.1 at 2026-04-05 14:00")
        ),
        "payment-service", Map.of(
            "depends_on", List.of("mysql-primary", "redis-cache"),
            "recent_changes", List.of()
        ),
        "inventory-service", Map.of(
            "depends_on", List.of("mysql-primary", "elasticsearch"),
            "recent_changes", List.of("config change: max_connections 100→200")
        ),
        "user-service", Map.of(
            "depends_on", List.of("mysql-replica", "redis-cache"),
            "recent_changes", List.of()
        ),
        "mysql-primary", Map.of("depends_on", List.of(), "recent_changes", List.of()),
        "redis-cache", Map.of("depends_on", List.of(), "recent_changes", List.of())
    );

    public RCAAgent() {
        super("RCAAgent");
    }

    @Override
    @SuppressWarnings("unchecked")
    protected IncidentState process(IncidentState state) {
        AlertEvent alert = state.getAlertEvent();
        if (alert == null) {
            logger.warn("[RCAAgent] No alert to analyze");
            return state;
        }

        List<String> affectedChain = traceDependencyChain(alert.getTargetService(), 0, 5);
        List<Map<String, Object>> evidence = collectEvidence(alert, affectedChain);
        boolean hasRecentChange = evidence.stream()
                .anyMatch(e -> "recent_change".equals(e.get("type")));

        // 贝叶斯推理
        String bestCause = "上游服务请求量激增导致 CPU 过载";
        double bestConfidence = 0.35;
        List<String> bestActions = List.of("scale_up", "rate_limit");

        if (hasRecentChange) {
            bestCause = "近期代码部署引入性能退化";
            bestConfidence = 0.54;  // P(E|C)*P(C)/P(E) = 0.9*0.3/0.5
            bestActions = List.of("rollback", "profiling");
        }

        RCAResult rca = RCAResult.builder()
                .alertEventId(alert.getEventId())
                .rootCause(bestCause)
                .confidence(bestConfidence)
                .affectedServices(affectedChain)
                .evidence(evidence)
                .suggestedActions(bestActions)
                .build();

        state.setRcaResult(rca);
        logger.info("[RCAAgent] Root cause: {} (confidence: {})", bestCause, bestConfidence);
        return state;
    }

    @SuppressWarnings("unchecked")
    private List<String> traceDependencyChain(String service, int depth, int maxDepth) {
        if (depth >= maxDepth) return List.of();

        List<String> chain = new ArrayList<>();
        chain.add(service);

        Map<String, Object> topo = SERVICE_TOPOLOGY.getOrDefault(service, Map.of());
        List<String> deps = (List<String>) topo.getOrDefault("depends_on", List.of());

        for (String dep : deps) {
            chain.addAll(traceDependencyChain(dep, depth + 1, maxDepth));
        }
        return chain;
    }

    @SuppressWarnings("unchecked")
    private List<Map<String, Object>> collectEvidence(AlertEvent alert, List<String> chain) {
        List<Map<String, Object>> evidence = new ArrayList<>();

        evidence.add(Map.of(
            "type", "metric_anomaly",
            "source", "prometheus",
            "detail", String.format("%s=%.1f", alert.getMetricName(), alert.getMetricValue())
        ));

        for (String svc : chain) {
            Map<String, Object> topo = SERVICE_TOPOLOGY.getOrDefault(svc, Map.of());
            List<String> changes = (List<String>) topo.getOrDefault("recent_changes", List.of());
            for (String change : changes) {
                evidence.add(Map.of(
                    "type", "recent_change",
                    "source", "cmdb",
                    "detail", svc + ": " + change
                ));
            }
        }

        return evidence;
    }
}
