package com.aiops.model;

import lombok.Data;
import lombok.Builder;
import lombok.AllArgsConstructor;
import lombok.NoArgsConstructor;

@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class ChangeDecision {

    private String healEventId;
    private double riskScore;
    private String approvalStatus;
    private String approver;
    private String reason;
}
