package agent

import (
	"fmt"
	"math"
	"sync"

	"github.com/google/uuid"
	"github.com/multi-agent-aiops/golang/internal/model"
)

/*
监控告警 Agent — Go 实现

面试要点（Go 特色）：
- sync.Map / sync.RWMutex 保证并发安全的历史数据存储
- goroutine 天然适合并发指标采集
- 与 Python/Java 对比：Go 版更轻量，适合高并发运维场景
*/

type MonitorAgent struct {
	metrics BaseMetrics
	history sync.Map // map[string][]float64
	mu      sync.RWMutex
}

func NewMonitorAgent() *MonitorAgent {
	return &MonitorAgent{}
}

func (a *MonitorAgent) Name() string { return "MonitorAgent" }

func (a *MonitorAgent) Handle(state *model.IncidentState) *model.IncidentState {
	return WrapHandle(a.Name(), &a.metrics, a.process, state)
}

func (a *MonitorAgent) Metrics() map[string]interface{} {
	return map[string]interface{}{
		"agent":           a.Name(),
		"processed_count": a.metrics.ProcessedCount.Load(),
		"error_count":     a.metrics.ErrorCount.Load(),
	}
}

func (a *MonitorAgent) process(state *model.IncidentState) *model.IncidentState {
	metricData, ok := state.Metadata["metric_data"].(map[string]interface{})
	if !ok {
		return a.generateDemoAlert(state)
	}

	metricName, _ := metricData["metric_name"].(string)
	value, _ := metricData["value"].(float64)
	service, _ := metricData["service"].(string)

	a.appendHistory(metricName, value)
	history := a.getHistory(metricName)

	zScore := computeZScore(history, value)
	if zScore <= 3.0 {
		return state
	}

	severity := classifySeverity(zScore)
	state.AlertEvent = &model.AlertEvent{
		EventID:       uuid.New().String(),
		AlertName:     fmt.Sprintf("anomaly_%s", metricName),
		Severity:      severity,
		Source:        "prometheus",
		TargetService: service,
		MetricName:    metricName,
		MetricValue:   value,
		Threshold:     zScore,
		Description:   fmt.Sprintf("Anomaly: %s=%.2f, z-score=%.2f", metricName, value, zScore),
		Labels:        map[string]string{},
	}
	state.Status = "investigating"
	return state
}

func (a *MonitorAgent) generateDemoAlert(state *model.IncidentState) *model.IncidentState {
	state.AlertEvent = &model.AlertEvent{
		EventID:       uuid.New().String(),
		AlertName:     "high_cpu_usage",
		Severity:      model.SeverityHigh,
		Source:        "prometheus",
		TargetService: "order-service",
		MetricName:    "cpu_usage_percent",
		MetricValue:   95.3,
		Threshold:     80.0,
		Description:   "CPU usage exceeded threshold: 95.3% > 80%",
		Labels:        map[string]string{"pod": "order-service-7d8f9b6c4-x2k9m", "namespace": "production"},
	}
	state.Status = "investigating"
	return state
}

func (a *MonitorAgent) appendHistory(name string, value float64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	var history []float64
	if v, ok := a.history.Load(name); ok {
		history = v.([]float64)
	}
	history = append(history, value)
	if len(history) > 1000 {
		history = history[len(history)-1000:]
	}
	a.history.Store(name, history)
}

func (a *MonitorAgent) getHistory(name string) []float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if v, ok := a.history.Load(name); ok {
		return v.([]float64)
	}
	return nil
}

func computeZScore(values []float64, current float64) float64 {
	n := len(values)
	if n < 10 {
		return 0.0
	}

	var sum float64
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(n)

	var variance float64
	for _, v := range values {
		variance += (v - mean) * (v - mean)
	}
	variance /= float64(n)
	std := math.Sqrt(variance)

	if std == 0 {
		return 0.0
	}
	return math.Abs(current-mean) / std
}

func classifySeverity(score float64) model.Severity {
	switch {
	case score > 5.0:
		return model.SeverityCritical
	case score > 4.0:
		return model.SeverityHigh
	case score > 3.0:
		return model.SeverityMedium
	default:
		return model.SeverityLow
	}
}
