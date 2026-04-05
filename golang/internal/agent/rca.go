package agent

import (
	"fmt"
	"log"

	"github.com/multi-agent-aiops/golang/internal/model"
)

/*
根因分析 Agent — Go 实现

面试要点（Go 特色）：
- channel 可用于 Agent 间异步通信
- map 嵌套模拟知识图谱拓扑
- 递归 + slice 实现依赖链追踪
*/

var serviceTopo = map[string]struct {
	DependsOn     []string
	RecentChanges []string
}{
	"order-service":     {[]string{"payment-service", "inventory-service", "user-service"}, []string{"deploy v2.3.1"}},
	"payment-service":   {[]string{"mysql-primary", "redis-cache"}, nil},
	"inventory-service": {[]string{"mysql-primary", "elasticsearch"}, []string{"config: max_conn 100→200"}},
	"user-service":      {[]string{"mysql-replica", "redis-cache"}, nil},
	"mysql-primary":     {nil, nil},
	"redis-cache":       {nil, nil},
}

type RCAAgent struct {
	metrics BaseMetrics
}

func NewRCAAgent() *RCAAgent {
	return &RCAAgent{}
}

func (a *RCAAgent) Name() string { return "RCAAgent" }

func (a *RCAAgent) Handle(state *model.IncidentState) *model.IncidentState {
	return WrapHandle(a.Name(), &a.metrics, a.process, state)
}

func (a *RCAAgent) Metrics() map[string]interface{} {
	return map[string]interface{}{
		"agent":           a.Name(),
		"processed_count": a.metrics.ProcessedCount.Load(),
		"error_count":     a.metrics.ErrorCount.Load(),
	}
}

func (a *RCAAgent) process(state *model.IncidentState) *model.IncidentState {
	alert := state.AlertEvent
	if alert == nil {
		log.Println("[RCAAgent] No alert to analyze")
		return state
	}

	chain := traceDeps(alert.TargetService, 0, 5)

	evidence := collectEvidence(alert, chain)
	hasRecentChange := false
	for _, e := range evidence {
		if e["type"] == "recent_change" {
			hasRecentChange = true
			break
		}
	}

	rootCause := "上游服务请求量激增导致 CPU 过载"
	confidence := 0.35
	actions := []string{"scale_up", "rate_limit"}

	if hasRecentChange {
		rootCause = "近期代码部署引入性能退化"
		confidence = 0.54
		actions = []string{"rollback", "profiling"}
	}

	state.RCAResult = &model.RCAResult{
		AlertEventID:     alert.EventID,
		RootCause:        rootCause,
		Confidence:       confidence,
		AffectedServices: chain,
		Evidence:         evidence,
		SuggestedActions: actions,
	}

	log.Printf("[RCAAgent] Root cause: %s (confidence: %.2f)", rootCause, confidence)
	return state
}

func traceDeps(service string, depth, maxDepth int) []string {
	if depth >= maxDepth {
		return nil
	}
	chain := []string{service}
	if topo, ok := serviceTopo[service]; ok {
		for _, dep := range topo.DependsOn {
			chain = append(chain, traceDeps(dep, depth+1, maxDepth)...)
		}
	}
	return chain
}

func collectEvidence(alert *model.AlertEvent, chain []string) []map[string]interface{} {
	var evidence []map[string]interface{}

	evidence = append(evidence, map[string]interface{}{
		"type":   "metric_anomaly",
		"source": "prometheus",
		"detail": fmt.Sprintf("%s=%.1f", alert.MetricName, alert.MetricValue),
	})

	for _, svc := range chain {
		if topo, ok := serviceTopo[svc]; ok {
			for _, change := range topo.RecentChanges {
				evidence = append(evidence, map[string]interface{}{
					"type":   "recent_change",
					"source": "cmdb",
					"detail": svc + ": " + change,
				})
			}
		}
	}

	return evidence
}
