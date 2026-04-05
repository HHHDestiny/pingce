package main

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/multi-agent-aiops/golang/internal/core"
)

/*
Go 版 HTTP 服务入口

面试要点：
- Gin 高性能 HTTP 框架（对标 Python FastAPI / Java Spring MVC）
- goroutine 并发处理请求，天然支持高并发
- 适合 Kubernetes 运维平台场景（Go 是云原生第一语言）
*/

var (
	orchestrator    *core.Orchestrator
	incidentHistory []map[string]interface{}
	historyMu       sync.Mutex
)

func main() {
	orchestrator = core.NewOrchestrator()

	r := gin.Default()

	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service": "Multi-Agent AIOps System (Go)",
			"version": "1.0.0",
			"agents":  []string{"monitor", "rca", "heal", "change"},
			"status":  "running",
		})
	})

	api := r.Group("/api/v1")
	{
		api.POST("/incidents/trigger", triggerIncident)
		api.GET("/incidents", listIncidents)
		api.GET("/agents/status", agentStatus)
		api.GET("/health", healthCheck)
	}

	log.Println("Starting AIOps server on :8081")
	if err := r.Run(":8081"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func triggerIncident(c *gin.Context) {
	var req map[string]interface{}
	c.ShouldBindJSON(&req)

	metadata := make(map[string]interface{})
	if metricName, ok := req["metric_name"]; ok {
		metadata["metric_data"] = map[string]interface{}{
			"metric_name": metricName,
			"value":       req["metric_value"],
			"service":     req["target_service"],
			"labels":      req["labels"],
		}
	}

	state := orchestrator.Run(metadata)

	result := map[string]interface{}{
		"incident_id": state.IncidentID,
		"status":      state.Status,
	}

	if state.AlertEvent != nil {
		result["alert"] = map[string]interface{}{
			"name":     state.AlertEvent.AlertName,
			"severity": state.AlertEvent.Severity,
			"service":  state.AlertEvent.TargetService,
			"value":    state.AlertEvent.MetricValue,
		}
	}
	if state.RCAResult != nil {
		result["rca"] = map[string]interface{}{
			"root_cause":        state.RCAResult.RootCause,
			"confidence":        state.RCAResult.Confidence,
			"suggested_actions": state.RCAResult.SuggestedActions,
		}
	}
	if state.HealAction != nil {
		result["heal"] = map[string]interface{}{
			"action":       state.HealAction.ActionType,
			"level":        state.HealAction.HealLevel,
			"blast_radius": state.HealAction.BlastRadius,
		}
	}
	if state.ChangeDecision != nil {
		result["change"] = map[string]interface{}{
			"status":     state.ChangeDecision.ApprovalStatus,
			"risk_score": state.ChangeDecision.RiskScore,
			"approver":   state.ChangeDecision.Approver,
			"reason":     state.ChangeDecision.Reason,
		}
	}

	historyMu.Lock()
	incidentHistory = append(incidentHistory, result)
	historyMu.Unlock()

	c.JSON(http.StatusOK, result)
}

func listIncidents(c *gin.Context) {
	historyMu.Lock()
	defer historyMu.Unlock()

	start := 0
	if len(incidentHistory) > 50 {
		start = len(incidentHistory) - 50
	}

	c.JSON(http.StatusOK, gin.H{
		"total":     len(incidentHistory),
		"incidents": incidentHistory[start:],
	})
}

func agentStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"workflow_nodes": orchestrator.GetWorkflowStatus(),
	})
}

func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
