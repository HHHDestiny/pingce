package com.aiops.agent;

import com.aiops.model.AlertEvent;
import com.aiops.model.IncidentState;
import com.aiops.model.Severity;
import org.springframework.stereotype.Component;

import java.util.*;
import java.util.concurrent.ConcurrentHashMap;

/**
 * 监控告警 Agent — Java 实现
 *
 * 面试要点（Java 特色）：
 * - ConcurrentHashMap 保证线程安全的历史数据存储
 * - DoubleSummaryStatistics 高效计算统计量
 * - 与 Python 版对比：Java 更适合高吞吐量场景
 */
@Component
public class MonitorAgent extends BaseAgent {

    private final Map<String, List<Double>> metricHistory = new ConcurrentHashMap<>();
    private static final int MAX_HISTORY = 1000;

    public MonitorAgent() {
        super("MonitorAgent");
    }

    @Override
    protected IncidentState process(IncidentState state) {
        @SuppressWarnings("unchecked")
        Map<String, Object> metricData = (Map<String, Object>) state.getMetadata().get("metric_data");

        AlertEvent alert;
        if (metricData != null) {
            alert = processMetricData(metricData);
        } else {
            alert = generateDemoAlert();
        }

        state.setAlertEvent(alert);
        state.setStatus("investigating");
        return state;
    }

    private AlertEvent processMetricData(Map<String, Object> metricData) {
        String metricName = (String) metricData.getOrDefault("metric_name", "unknown");
        double value = ((Number) metricData.getOrDefault("value", 0.0)).doubleValue();
        String service = (String) metricData.getOrDefault("service", "unknown");

        metricHistory.computeIfAbsent(metricName, k -> new ArrayList<>()).add(value);
        List<Double> history = metricHistory.get(metricName);
        if (history.size() > MAX_HISTORY) {
            metricHistory.put(metricName, new ArrayList<>(history.subList(history.size() - MAX_HISTORY, history.size())));
        }

        double zScore = computeZScore(history, value);
        boolean isAnomaly = zScore > 3.0;

        if (!isAnomaly) {
            return AlertEvent.builder()
                    .alertName("normal_" + metricName)
                    .severity(Severity.INFO)
                    .source("prometheus")
                    .targetService(service)
                    .metricName(metricName)
                    .metricValue(value)
                    .build();
        }

        return AlertEvent.builder()
                .alertName("anomaly_" + metricName)
                .severity(classifySeverity(zScore))
                .source("prometheus")
                .targetService(service)
                .metricName(metricName)
                .metricValue(value)
                .threshold(zScore)
                .description(String.format("Anomaly: %s=%.2f, z-score=%.2f", metricName, value, zScore))
                .build();
    }

    private AlertEvent generateDemoAlert() {
        return AlertEvent.builder()
                .alertName("high_cpu_usage")
                .severity(Severity.HIGH)
                .source("prometheus")
                .targetService("order-service")
                .metricName("cpu_usage_percent")
                .metricValue(95.3)
                .threshold(80.0)
                .description("CPU usage exceeded threshold: 95.3% > 80%")
                .labels(Map.of("pod", "order-service-7d8f9b6c4-x2k9m", "namespace", "production"))
                .build();
    }

    /**
     * 3-Sigma 异常检测
     */
    private double computeZScore(List<Double> values, double current) {
        if (values.size() < 10) return 0.0;

        DoubleSummaryStatistics stats = values.stream()
                .mapToDouble(Double::doubleValue)
                .summaryStatistics();

        double mean = stats.getAverage();
        double variance = values.stream()
                .mapToDouble(v -> Math.pow(v - mean, 2))
                .average()
                .orElse(0.0);
        double std = Math.sqrt(variance);

        if (std == 0) return 0.0;
        return Math.abs(current - mean) / std;
    }

    private Severity classifySeverity(double score) {
        if (score > 5.0) return Severity.CRITICAL;
        if (score > 4.0) return Severity.HIGH;
        if (score > 3.0) return Severity.MEDIUM;
        return Severity.LOW;
    }
}
