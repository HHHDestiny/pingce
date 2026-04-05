package com.aiops.core;

import com.aiops.model.IncidentState;
import org.springframework.web.bind.annotation.*;

import java.time.Instant;
import java.util.*;
import java.util.concurrent.CopyOnWriteArrayList;

/**
 * REST API 控制器
 */
@RestController
@RequestMapping("/api/v1")
public class AiOpsController {

    private final Orchestrator orchestrator;
    private final List<Map<String, Object>> incidentHistory = new CopyOnWriteArrayList<>();

    public AiOpsController(Orchestrator orchestrator) {
        this.orchestrator = orchestrator;
    }

    @GetMapping("/")
    public Map<String, Object> root() {
        return Map.of(
            "service", "Multi-Agent AIOps System (Java)",
            "version", "1.0.0",
            "agents", List.of("monitor", "rca", "heal", "change"),
            "status", "running"
        );
    }

    @PostMapping("/incidents/trigger")
    public Map<String, Object> triggerIncident(@RequestBody(required = false) Map<String, Object> request) {
        Map<String, Object> metadata = new HashMap<>();
        if (request != null && request.containsKey("metric_name")) {
            metadata.put("metric_data", Map.of(
                "metric_name", request.get("metric_name"),
                "value", request.getOrDefault("metric_value", 0.0),
                "service", request.getOrDefault("target_service", "unknown"),
                "labels", request.getOrDefault("labels", Map.of())
            ));
        }

        IncidentState state = orchestrator.run(metadata);
        Map<String, Object> result = serializeState(state);
        incidentHistory.add(result);
        return result;
    }

    @GetMapping("/incidents")
    public Map<String, Object> listIncidents() {
        int size = incidentHistory.size();
        return Map.of(
            "total", size,
            "incidents", incidentHistory.subList(Math.max(0, size - 50), size)
        );
    }

    @GetMapping("/agents/status")
    public Map<String, Object> getAgentStatus() {
        return Map.of("workflow_nodes", orchestrator.getWorkflowStatus());
    }

    @GetMapping("/health")
    public Map<String, Object> health() {
        return Map.of("status", "healthy", "timestamp", Instant.now().toString());
    }

    private Map<String, Object> serializeState(IncidentState state) {
        Map<String, Object> result = new HashMap<>();
        result.put("incident_id", state.getIncidentId());
        result.put("status", state.getStatus());

        if (state.getAlertEvent() != null) {
            var alert = state.getAlertEvent();
            result.put("alert", Map.of(
                "name", alert.getAlertName(),
                "severity", alert.getSeverity().name(),
                "service", alert.getTargetService(),
                "value", alert.getMetricValue()
            ));
        }

        if (state.getRcaResult() != null) {
            var rca = state.getRcaResult();
            result.put("rca", Map.of(
                "root_cause", rca.getRootCause(),
                "confidence", rca.getConfidence(),
                "suggested_actions", rca.getSuggestedActions()
            ));
        }

        if (state.getHealAction() != null) {
            var heal = state.getHealAction();
            result.put("heal", Map.of(
                "action", heal.getActionType(),
                "level", heal.getHealLevel().name(),
                "blast_radius", heal.getBlastRadius()
            ));
        }

        if (state.getChangeDecision() != null) {
            var change = state.getChangeDecision();
            result.put("change", Map.of(
                "status", change.getApprovalStatus(),
                "risk_score", change.getRiskScore(),
                "approver", change.getApprover(),
                "reason", change.getReason()
            ));
        }

        return result;
    }
}
