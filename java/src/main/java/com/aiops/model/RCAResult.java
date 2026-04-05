package com.aiops.model;

import lombok.Data;
import lombok.Builder;
import lombok.AllArgsConstructor;
import lombok.NoArgsConstructor;

import java.util.List;
import java.util.Map;

@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class RCAResult {

    private String alertEventId;
    private String rootCause;
    private double confidence;
    private List<String> affectedServices;
    private List<Map<String, Object>> evidence;
    private List<String> suggestedActions;
}
