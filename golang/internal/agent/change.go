package agent

import (
	"fmt"
	"log"
	"math"
	"time"

	"github.com/multi-agent-aiops/golang/internal/model"
)

/*
变更审批 Agent — Go 实现

面试要点（Go 特色）：
- 无类继承，通过接口多态实现策略模式
- 与 Java 比更简洁，与 Python 比类型更安全
*/

var operationRisk = map[string]float64{
	"restart_pod":     0.2,
	"scale_up":        0.15,
	"rate_limit":      0.1,
	"circuit_breaker": 0.1,
	"rollback":        0.6,
}

var serviceCriticality = map[string]float64{
	"payment-service":   1.0,
	"order-service":     0.9,
	"user-service":      0.8,
	"inventory-service": 0.7,
}

const autoApproveThreshold = 0.3

type ChangeAgent struct {
	metrics BaseMetrics
}

func NewChangeAgent() *ChangeAgent {
	return &ChangeAgent{}
}

func (a *ChangeAgent) Name() string { return "ChangeAgent" }

func (a *ChangeAgent) Handle(state *model.IncidentState) *model.IncidentState {
	return WrapHandle(a.Name(), &a.metrics, a.process, state)
}

func (a *ChangeAgent) Metrics() map[string]interface{} {
	return map[string]interface{}{
		"agent":           a.Name(),
		"processed_count": a.metrics.ProcessedCount.Load(),
		"error_count":     a.metrics.ErrorCount.Load(),
	}
}

func (a *ChangeAgent) process(state *model.IncidentState) *model.IncidentState {
	heal := state.HealAction
	if heal == nil {
		log.Println("[ChangeAgent] No heal action")
		return state
	}

	riskScore := computeRiskScore(heal.ActionType, heal.BlastRadius, heal.TargetService)

	var approvalStatus, approver, reason string

	switch {
	case heal.HealLevel == model.HealLevelL0Auto && riskScore <= autoApproveThreshold:
		approvalStatus = "approved"
		approver = "auto-system"
		reason = fmt.Sprintf("L0 auto-approved: risk=%.3f", riskScore)
	case heal.HealLevel == model.HealLevelL1Semi && riskScore <= 0.5:
		approvalStatus = "approved"
		approver = "oncall-engineer"
		reason = fmt.Sprintf("L1 approved: risk=%.3f", riskScore)
	case heal.HealLevel == model.HealLevelL2Manual:
		approvalStatus = "pending"
		approver = "tech-lead"
		reason = fmt.Sprintf("L2 requires TL approval: risk=%.3f", riskScore)
	default:
		approvalStatus = "pending"
		approver = "oncall-engineer"
		reason = fmt.Sprintf("Risk %.3f requires review", riskScore)
	}

	state.ChangeDecision = &model.ChangeDecision{
		HealEventID:    heal.RCAEventID,
		RiskScore:      riskScore,
		ApprovalStatus: approvalStatus,
		Approver:       approver,
		Reason:         reason,
	}

	if approvalStatus == "approved" {
		state.Status = "resolved"
	} else {
		state.Status = "pending_approval"
	}

	log.Printf("[ChangeAgent] %s: %s risk=%.3f approver=%s", approvalStatus, heal.ActionType, riskScore, approver)
	return state
}

func computeRiskScore(actionType string, blastRadius float64, service string) float64 {
	opRisk := 0.5
	if r, ok := operationRisk[actionType]; ok {
		opRisk = r
	}
	svcCrit := 0.5
	if c, ok := serviceCriticality[service]; ok {
		svcCrit = c
	}

	hour := time.Now().Hour()
	timeRisk := 0.1
	if hour < 8 || hour > 22 {
		timeRisk = 0.3
	}

	score := 0.30*blastRadius + 0.25*opRisk + 0.20*timeRisk + 0.15*0.1 + 0.10*svcCrit
	return math.Min(score, 1.0)
}
