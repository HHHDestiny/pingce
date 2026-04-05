package com.aiops.model;

import lombok.Data;
import lombok.Builder;
import lombok.AllArgsConstructor;
import lombok.NoArgsConstructor;

import java.time.Instant;
import java.util.HashMap;
import java.util.Map;
import java.util.UUID;

@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class AlertEvent {

    @Builder.Default
    private String eventId = UUID.randomUUID().toString();

    @Builder.Default
    private Instant timestamp = Instant.now();

    private String alertName;
    private Severity severity;
    private String source;
    private String targetService;
    private String metricName;
    private Double metricValue;
    private Double threshold;
    private String description;

    @Builder.Default
    private Map<String, String> labels = new HashMap<>();
}
