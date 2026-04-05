package com.aiops.model;

import lombok.Data;
import lombok.Builder;
import lombok.AllArgsConstructor;
import lombok.NoArgsConstructor;

import java.time.Instant;
import java.util.HashMap;
import java.util.Map;
import java.util.UUID;

/**
 * 故障事件全局状态 — 在 Agent 间流转
 */
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class IncidentState {

    @Builder.Default
    private String incidentId = UUID.randomUUID().toString();

    @Builder.Default
    private Instant createdAt = Instant.now();

    @Builder.Default
    private Instant updatedAt = Instant.now();

    @Builder.Default
    private String status = "open";

    private AlertEvent alertEvent;
    private RCAResult rcaResult;
    private HealAction healAction;
    private ChangeDecision changeDecision;

    @Builder.Default
    private String currentAgent = "monitor";

    @Builder.Default
    private int retryCount = 0;

    private String errorMessage;

    @Builder.Default
    private Map<String, Object> metadata = new HashMap<>();
}
