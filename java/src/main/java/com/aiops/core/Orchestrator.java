package com.aiops.core;

import com.aiops.agent.*;
import com.aiops.model.IncidentState;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Service;

import java.time.Instant;
import java.util.*;
import java.util.concurrent.ConcurrentHashMap;
import java.util.function.Predicate;

/**
 * Agent 编排器 — 状态机模式编排多 Agent 协作
 *
 * 面试要点（对比 Python 版）：
 * - Java 用函数式接口 Predicate 实现条件路由
 * - ConcurrentHashMap 保证线程安全的检查点存储
 * - 与 Python LangGraph 的区别：Java 版更接近手写状态机
 */
@Service
public class Orchestrator {

    private static final Logger logger = LoggerFactory.getLogger(Orchestrator.class);

    private final List<WorkflowNode> nodes;
    private final Map<String, IncidentState> checkpoints = new ConcurrentHashMap<>();

    public Orchestrator(
            MonitorAgent monitorAgent,
            RCAAgent rcaAgent,
            HealAgent healAgent,
            ChangeAgent changeAgent
    ) {
        this.nodes = List.of(
            new WorkflowNode("monitor", monitorAgent, s -> true, 3),
            new WorkflowNode("rca", rcaAgent, s -> s.getAlertEvent() != null, 3),
            new WorkflowNode("heal", healAgent,
                s -> s.getRcaResult() != null && s.getRcaResult().getConfidence() >= 0.3, 3),
            new WorkflowNode("change", changeAgent, s -> s.getHealAction() != null, 3)
        );
    }

    public IncidentState run(Map<String, Object> metadata) {
        IncidentState state = IncidentState.builder()
                .metadata(metadata != null ? metadata : new HashMap<>())
                .build();

        logger.info("[Orchestrator] Starting workflow: {}", state.getIncidentId());

        for (WorkflowNode node : nodes) {
            if (!node.condition.test(state)) {
                logger.info("[Orchestrator] Skipping: {}", node.name);
                continue;
            }

            boolean success = false;
            for (int retry = 0; retry <= node.maxRetries; retry++) {
                try {
                    state = node.agent.handle(state);
                    if (state.getErrorMessage() == null) {
                        success = true;
                        break;
                    }
                    logger.warn("[Orchestrator] Retry {}/{}: {}", retry + 1, node.maxRetries, node.name);
                    state.setErrorMessage(null);
                } catch (Exception e) {
                    logger.error("[Orchestrator] {} failed: {}", node.name, e.getMessage());
                    if (retry >= node.maxRetries) break;
                }
            }

            if (!success) {
                logger.error("[Orchestrator] {} failed after retries", node.name);
            }

            checkpoints.put(state.getIncidentId(), state);
        }

        state.setUpdatedAt(Instant.now());
        logger.info("[Orchestrator] Workflow complete: {} status={}", state.getIncidentId(), state.getStatus());
        return state;
    }

    public Optional<IncidentState> getCheckpoint(String incidentId) {
        return Optional.ofNullable(checkpoints.get(incidentId));
    }

    public List<Map<String, Object>> getWorkflowStatus() {
        return nodes.stream()
                .map(n -> Map.<String, Object>of(
                    "name", n.name,
                    "agent", n.agent.getName(),
                    "metrics", n.agent.getMetrics()
                ))
                .toList();
    }

    record WorkflowNode(
        String name,
        BaseAgent agent,
        Predicate<IncidentState> condition,
        int maxRetries
    ) {}
}
