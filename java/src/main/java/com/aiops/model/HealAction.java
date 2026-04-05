package com.aiops.model;

import lombok.Data;
import lombok.Builder;
import lombok.AllArgsConstructor;
import lombok.NoArgsConstructor;

import java.util.Map;

@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class HealAction {

    private String rcaEventId;
    private HealLevel healLevel;
    private String actionType;
    private Map<String, Object> actionParams;
    private String targetService;
    private String estimatedImpact;
    private double blastRadius;
    private String dryRunResult;
    private String executionResult;
}
