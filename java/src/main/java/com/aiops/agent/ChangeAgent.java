package com.aiops.agent;

import com.aiops.model.*;
import org.springframework.stereotype.Component;

import java.time.LocalTime;
import java.util.Map;

/**
 * 变更审批 Agent — Java 实现
 *
 * 面试要点：
 * - 风险评分是多维度加权模型
 * - 审批策略是责任链模式
 * - 审计日志满足 SOC2 合规
 */
@Component
public class ChangeAgent extends BaseAgent {

    private static final double AUTO_APPROVE_THRESHOLD = 0.3;

    private static final Map<String, Double> OPERATION_RISK = Map.of(
        "restart_pod", 0.2,
        "scale_up", 0.15,
        "rate_limit", 0.1,
        "circuit_breaker", 0.1,
        "rollback", 0.6
    );

    private static final Map<String, Double> SERVICE_CRITICALITY = Map.of(
        "payment-service", 1.0,
        "order-service", 0.9,
        "user-service", 0.8,
        "inventory-service", 0.7
    );

    public ChangeAgent() {
        super("ChangeAgent");
    }

    @Override
    protected IncidentState process(IncidentState state) {
        HealAction heal = state.getHealAction();
        if (heal == null) {
            logger.warn("[ChangeAgent] No heal action to review");
            return state;
        }

        double riskScore = computeRiskScore(
                heal.getActionType(),
                heal.getBlastRadius(),
                heal.getTargetService()
        );

        String approvalStatus;
        String approver;
        String reason;

        if (heal.getHealLevel() == HealLevel.L0_AUTO && riskScore <= AUTO_APPROVE_THRESHOLD) {
            approvalStatus = "approved";
            approver = "auto-system";
            reason = String.format("L0 auto-approved: risk=%.3f <= threshold=%.1f",
                    riskScore, AUTO_APPROVE_THRESHOLD);
        } else if (heal.getHealLevel() == HealLevel.L1_SEMI && riskScore <= 0.5) {
            approvalStatus = "approved";
            approver = "oncall-engineer";
            reason = String.format("L1 approved: risk=%.3f", riskScore);
        } else if (heal.getHealLevel() == HealLevel.L2_MANUAL) {
            approvalStatus = "pending";
            approver = "tech-lead";
            reason = String.format("L2 requires TL approval: risk=%.3f", riskScore);
        } else {
            approvalStatus = "pending";
            approver = "oncall-engineer";
            reason = String.format("Risk %.3f requires manual review", riskScore);
        }

        ChangeDecision decision = ChangeDecision.builder()
                .healEventId(heal.getRcaEventId())
                .riskScore(riskScore)
                .approvalStatus(approvalStatus)
                .approver(approver)
                .reason(reason)
                .build();

        state.setChangeDecision(decision);
        state.setStatus("approved".equals(approvalStatus) ? "resolved" : "pending_approval");

        logger.info("[ChangeAgent] {}: {} risk={} approver={}",
                approvalStatus.toUpperCase(), heal.getActionType(), riskScore, approver);
        return state;
    }

    private double computeRiskScore(String actionType, double blastRadius, String service) {
        double opRisk = OPERATION_RISK.getOrDefault(actionType, 0.5);
        double svcCriticality = SERVICE_CRITICALITY.getOrDefault(service, 0.5);
        int hour = LocalTime.now().getHour();
        double timeRisk = (hour < 8 || hour > 22) ? 0.3 : 0.1;

        return Math.min(1.0,
                0.30 * blastRadius +
                0.25 * opRisk +
                0.20 * timeRisk +
                0.15 * 0.1 +
                0.10 * svcCriticality
        );
    }
}
