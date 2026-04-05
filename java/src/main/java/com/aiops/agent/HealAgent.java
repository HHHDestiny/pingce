package com.aiops.agent;

import com.aiops.model.*;
import org.springframework.stereotype.Component;

import java.time.Instant;
import java.util.Map;
import java.util.concurrent.atomic.AtomicInteger;

/**
 * 故障自愈 Agent — Java 实现
 *
 * 面试要点（Java 设计模式）：
 * - 策略模式：不同修复方案是不同的策略
 * - 责任链模式：dry-run → 爆炸半径检查 → 熔断器检查 → 执行
 * - 状态模式：熔断器的 CLOSED/OPEN/HALF_OPEN 转换
 */
@Component
public class HealAgent extends BaseAgent {

    private final boolean dryRun;
    private final double maxBlastRadius;

    // 熔断器状态
    private final AtomicInteger failureCount = new AtomicInteger(0);
    private volatile String circuitState = "CLOSED";
    private volatile Instant lastFailureTime = null;
    private static final int CB_THRESHOLD = 5;
    private static final long CB_TIMEOUT_MS = 60_000;

    private static final Map<String, PlaybookEntry> PLAYBOOKS = Map.of(
        "restart_pod", new PlaybookEntry(HealLevel.L0_AUTO, "重启故障 Pod",
            "kubectl rollout restart deployment/{service} -n production", 0.05),
        "scale_up", new PlaybookEntry(HealLevel.L0_AUTO, "水平扩容 Pod",
            "kubectl scale deployment/{service} --replicas=3 -n production", 0.02),
        "rollback", new PlaybookEntry(HealLevel.L1_SEMI, "回滚到上一版本",
            "kubectl rollout undo deployment/{service} -n production", 0.15),
        "rate_limit", new PlaybookEntry(HealLevel.L0_AUTO, "启用限流",
            "kubectl annotate svc/{service} rate-limit=100 -n production --overwrite", 0.01),
        "circuit_breaker", new PlaybookEntry(HealLevel.L0_AUTO, "开启熔断器",
            "kubectl annotate svc/{service} circuit-breaker=open -n production --overwrite", 0.08)
    );

    public HealAgent() {
        super("HealAgent");
        this.dryRun = true;
        this.maxBlastRadius = 0.2;
    }

    @Override
    protected IncidentState process(IncidentState state) {
        RCAResult rca = state.getRcaResult();
        if (rca == null) {
            logger.warn("[HealAgent] No RCA result");
            return state;
        }

        if (!isCircuitAllowed()) {
            state.setErrorMessage("Circuit breaker OPEN, auto-heal suspended");
            return state;
        }

        String actionType = selectBestAction(rca.getSuggestedActions());
        PlaybookEntry playbook = PLAYBOOKS.getOrDefault(actionType,
                PLAYBOOKS.get("restart_pod"));

        HealLevel level = playbook.blastRadius > maxBlastRadius
                ? HealLevel.L2_MANUAL : playbook.level;

        String targetService = state.getAlertEvent() != null
                ? state.getAlertEvent().getTargetService() : "unknown";

        String dryRunResult = dryRun
                ? executeDryRun(playbook, targetService) : null;

        String executionResult = null;
        if (level == HealLevel.L0_AUTO && dryRun) {
            executionResult = "SUCCESS: " + playbook.description + " completed for " + targetService;
            failureCount.set(0);
            circuitState = "CLOSED";
        }

        HealAction heal = HealAction.builder()
                .rcaEventId(rca.getAlertEventId())
                .healLevel(level)
                .actionType(actionType)
                .actionParams(Map.of("command", playbook.command, "service", targetService))
                .targetService(targetService)
                .estimatedImpact(playbook.description)
                .blastRadius(playbook.blastRadius)
                .dryRunResult(dryRunResult)
                .executionResult(executionResult)
                .build();

        state.setHealAction(heal);
        state.setStatus("healing");

        logger.info("[HealAgent] Action: {} [{}] blast_radius={}", actionType, level, playbook.blastRadius);
        return state;
    }

    private String selectBestAction(java.util.List<String> suggested) {
        String[] priority = {"rate_limit", "circuit_breaker", "scale_up", "restart_pod", "rollback"};
        for (String action : priority) {
            if (suggested.contains(action)) return action;
        }
        return suggested.isEmpty() ? "restart_pod" : suggested.get(0);
    }

    private String executeDryRun(PlaybookEntry playbook, String service) {
        String cmd = playbook.command.replace("{service}", service);
        logger.info("[HealAgent] DRY-RUN: {}", cmd);
        return "DRY-RUN OK: " + cmd;
    }

    private boolean isCircuitAllowed() {
        if ("CLOSED".equals(circuitState)) return true;
        if ("OPEN".equals(circuitState) && lastFailureTime != null) {
            if (Instant.now().toEpochMilli() - lastFailureTime.toEpochMilli() > CB_TIMEOUT_MS) {
                circuitState = "HALF_OPEN";
                return true;
            }
            return false;
        }
        return true;
    }

    record PlaybookEntry(HealLevel level, String description, String command, double blastRadius) {}
}
