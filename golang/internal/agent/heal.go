package agent

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/multi-agent-aiops/golang/internal/model"
)

/*
故障自愈 Agent — Go 实现

面试要点（Go 特色）：
- atomic 操作实现无锁熔断器计数
- sync.Mutex 保护熔断器状态转换
- channel 可用于通知修复完成
*/

type playbookEntry struct {
	Level       model.HealLevel
	Description string
	Command     string
	BlastRadius float64
}

var playbooks = map[string]playbookEntry{
	"restart_pod":     {model.HealLevelL0Auto, "重启故障 Pod", "kubectl rollout restart deployment/{service} -n production", 0.05},
	"scale_up":        {model.HealLevelL0Auto, "水平扩容", "kubectl scale deployment/{service} --replicas=3 -n production", 0.02},
	"rollback":        {model.HealLevelL1Semi, "回滚到上一版本", "kubectl rollout undo deployment/{service} -n production", 0.15},
	"rate_limit":      {model.HealLevelL0Auto, "启用限流", "kubectl annotate svc/{service} rate-limit=100 --overwrite", 0.01},
	"circuit_breaker": {model.HealLevelL0Auto, "开启熔断器", "kubectl annotate svc/{service} circuit-breaker=open --overwrite", 0.08},
}

type HealAgent struct {
	metrics         BaseMetrics
	dryRun          bool
	maxBlastRadius  float64
	failureCount    atomic.Int32
	circuitState    string
	lastFailureTime time.Time
	mu              sync.Mutex
}

func NewHealAgent() *HealAgent {
	return &HealAgent{
		dryRun:         true,
		maxBlastRadius: 0.2,
		circuitState:   "CLOSED",
	}
}

func (a *HealAgent) Name() string { return "HealAgent" }

func (a *HealAgent) Handle(state *model.IncidentState) *model.IncidentState {
	return WrapHandle(a.Name(), &a.metrics, a.process, state)
}

func (a *HealAgent) Metrics() map[string]interface{} {
	return map[string]interface{}{
		"agent":           a.Name(),
		"processed_count": a.metrics.ProcessedCount.Load(),
		"error_count":     a.metrics.ErrorCount.Load(),
		"circuit_state":   a.circuitState,
	}
}

func (a *HealAgent) process(state *model.IncidentState) *model.IncidentState {
	rca := state.RCAResult
	if rca == nil {
		log.Println("[HealAgent] No RCA result")
		return state
	}

	if !a.isCircuitAllowed() {
		state.ErrorMessage = "Circuit breaker OPEN"
		return state
	}

	actionType := selectAction(rca.SuggestedActions)
	pb, ok := playbooks[actionType]
	if !ok {
		pb = playbooks["restart_pod"]
	}

	level := pb.Level
	if pb.BlastRadius > a.maxBlastRadius {
		level = model.HealLevelL2Manual
	}

	targetService := "unknown"
	if state.AlertEvent != nil {
		targetService = state.AlertEvent.TargetService
	}

	var dryRunResult string
	if a.dryRun {
		cmd := strings.ReplaceAll(pb.Command, "{service}", targetService)
		dryRunResult = fmt.Sprintf("DRY-RUN OK: %s", cmd)
		log.Printf("[HealAgent] DRY-RUN: %s", cmd)
	}

	var execResult string
	if level == model.HealLevelL0Auto && a.dryRun {
		execResult = fmt.Sprintf("SUCCESS: %s completed for %s", pb.Description, targetService)
		a.failureCount.Store(0)
		a.mu.Lock()
		a.circuitState = "CLOSED"
		a.mu.Unlock()
	}

	state.HealAction = &model.HealAction{
		RCAEventID:      rca.AlertEventID,
		HealLevel:       level,
		ActionType:      actionType,
		ActionParams:    map[string]interface{}{"command": pb.Command, "service": targetService},
		TargetService:   targetService,
		EstimatedImpact: pb.Description,
		BlastRadius:     pb.BlastRadius,
		DryRunResult:    dryRunResult,
		ExecutionResult: execResult,
	}
	state.Status = "healing"

	log.Printf("[HealAgent] Action: %s [%s] blast_radius=%.2f", actionType, level, pb.BlastRadius)
	return state
}

func (a *HealAgent) isCircuitAllowed() bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.circuitState == "CLOSED" {
		return true
	}
	if a.circuitState == "OPEN" && time.Since(a.lastFailureTime) > 60*time.Second {
		a.circuitState = "HALF_OPEN"
		return true
	}
	return a.circuitState == "HALF_OPEN"
}

func selectAction(suggested []string) string {
	priority := []string{"rate_limit", "circuit_breaker", "scale_up", "restart_pod", "rollback"}
	for _, action := range priority {
		for _, s := range suggested {
			if action == s {
				return action
			}
		}
	}
	if len(suggested) > 0 {
		return suggested[0]
	}
	return "restart_pod"
}
