package core

import (
	"log"
	"sync"
	"time"

	"github.com/multi-agent-aiops/golang/internal/agent"
	"github.com/multi-agent-aiops/golang/internal/model"
)

/*
Agent 编排器 — Go 实现

面试要点（Go 特色）：
- 函数作为一等公民：condition 是 func(*IncidentState) bool
- goroutine + channel 可以实现 Agent 并行执行（当 Agent 间无依赖时）
- sync.Map 保证并发安全的检查点存储
- Go 接口隐式实现 vs Java 显式实现 vs Python 鸭子类型
*/

type workflowNode struct {
	Name       string
	Agent      agent.Agent
	Condition  func(*model.IncidentState) bool
	MaxRetries int
}

// Orchestrator 多 Agent 编排器
type Orchestrator struct {
	nodes       []workflowNode
	checkpoints sync.Map
}

func NewOrchestrator() *Orchestrator {
	monitor := agent.NewMonitorAgent()
	rca := agent.NewRCAAgent()
	heal := agent.NewHealAgent()
	change := agent.NewChangeAgent()

	return &Orchestrator{
		nodes: []workflowNode{
			{"monitor", monitor, func(s *model.IncidentState) bool { return true }, 3},
			{"rca", rca, func(s *model.IncidentState) bool { return s.AlertEvent != nil }, 3},
			{"heal", heal, func(s *model.IncidentState) bool {
				return s.RCAResult != nil && s.RCAResult.Confidence >= 0.3
			}, 3},
			{"change", change, func(s *model.IncidentState) bool { return s.HealAction != nil }, 3},
		},
	}
}

// Run 执行完整的工作流
func (o *Orchestrator) Run(metadata map[string]interface{}) *model.IncidentState {
	state := model.NewIncidentState()
	if metadata != nil {
		state.Metadata = metadata
	}

	log.Printf("[Orchestrator] Starting workflow: %s", state.IncidentID)

	for _, node := range o.nodes {
		if !node.Condition(state) {
			log.Printf("[Orchestrator] Skipping: %s", node.Name)
			continue
		}

		success := false
		for retry := 0; retry <= node.MaxRetries; retry++ {
			state = node.Agent.Handle(state)
			if state.ErrorMessage == "" {
				success = true
				break
			}
			log.Printf("[Orchestrator] Retry %d/%d: %s", retry+1, node.MaxRetries, node.Name)
			state.ErrorMessage = ""
		}

		if !success {
			log.Printf("[Orchestrator] %s failed after retries", node.Name)
		}

		o.checkpoints.Store(state.IncidentID, state)
	}

	state.UpdatedAt = time.Now()
	log.Printf("[Orchestrator] Workflow complete: %s status=%s", state.IncidentID, state.Status)
	return state
}

// GetCheckpoint 获取检查点
func (o *Orchestrator) GetCheckpoint(incidentID string) (*model.IncidentState, bool) {
	v, ok := o.checkpoints.Load(incidentID)
	if !ok {
		return nil, false
	}
	return v.(*model.IncidentState), true
}

// GetWorkflowStatus 获取工作流状态
func (o *Orchestrator) GetWorkflowStatus() []map[string]interface{} {
	var result []map[string]interface{}
	for _, node := range o.nodes {
		result = append(result, map[string]interface{}{
			"name":    node.Name,
			"agent":   node.Agent.Name(),
			"metrics": node.Agent.Metrics(),
		})
	}
	return result
}
