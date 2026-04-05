package agent

import (
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/multi-agent-aiops/golang/internal/model"
)

// Agent 接口 — Go 的接口隐式实现（面试要点：鸭子类型）
type Agent interface {
	Name() string
	Handle(state *model.IncidentState) *model.IncidentState
	Metrics() map[string]interface{}
}

// BaseMetrics 基础指标（原子操作保证 goroutine 安全）
type BaseMetrics struct {
	ProcessedCount atomic.Int64
	ErrorCount     atomic.Int64
}

// WrapHandle 模板方法：统一的 Agent 执行包装
// Go 没有类继承，用组合 + 函数包装实现模板方法模式
func WrapHandle(
	name string,
	metrics *BaseMetrics,
	processFn func(*model.IncidentState) *model.IncidentState,
	state *model.IncidentState,
) *model.IncidentState {
	start := time.Now()
	log.Printf("[%s] Processing incident %s", name, state.IncidentID)

	state.CurrentAgent = name
	state.UpdatedAt = time.Now()

	defer func() {
		if r := recover(); r != nil {
			metrics.ErrorCount.Add(1)
			state.ErrorMessage = fmt.Sprintf("[%s] Panic: %v", name, r)
			log.Printf("[%s] Panic recovered: %v", name, r)
		}
	}()

	state = processFn(state)

	metrics.ProcessedCount.Add(1)
	elapsed := time.Since(start)
	log.Printf("[%s] Completed in %v", name, elapsed)

	return state
}
