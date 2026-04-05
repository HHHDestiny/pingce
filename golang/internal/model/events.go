package model

import (
	"time"

	"github.com/google/uuid"
)

type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityInfo     Severity = "info"
)

type HealLevel string

const (
	HealLevelL0Auto   HealLevel = "L0"
	HealLevelL1Semi   HealLevel = "L1"
	HealLevelL2Manual HealLevel = "L2"
)

// AlertEvent 监控告警事件
type AlertEvent struct {
	EventID       string            `json:"event_id"`
	Timestamp     time.Time         `json:"timestamp"`
	AlertName     string            `json:"alert_name"`
	Severity      Severity          `json:"severity"`
	Source        string            `json:"source"`
	TargetService string            `json:"target_service"`
	MetricName    string            `json:"metric_name"`
	MetricValue   float64           `json:"metric_value"`
	Threshold     float64           `json:"threshold"`
	Description   string            `json:"description"`
	Labels        map[string]string `json:"labels"`
}

// RCAResult 根因分析结果
type RCAResult struct {
	AlertEventID     string                   `json:"alert_event_id"`
	RootCause        string                   `json:"root_cause"`
	Confidence       float64                  `json:"confidence"`
	AffectedServices []string                 `json:"affected_services"`
	Evidence         []map[string]interface{} `json:"evidence"`
	SuggestedActions []string                 `json:"suggested_actions"`
}

// HealAction 自愈操作
type HealAction struct {
	RCAEventID      string                 `json:"rca_event_id"`
	HealLevel       HealLevel              `json:"heal_level"`
	ActionType      string                 `json:"action_type"`
	ActionParams    map[string]interface{} `json:"action_params"`
	TargetService   string                 `json:"target_service"`
	EstimatedImpact string                 `json:"estimated_impact"`
	BlastRadius     float64                `json:"blast_radius"`
	DryRunResult    string                 `json:"dry_run_result"`
	ExecutionResult string                 `json:"execution_result"`
}

// ChangeDecision 变更审批决策
type ChangeDecision struct {
	HealEventID    string  `json:"heal_event_id"`
	RiskScore      float64 `json:"risk_score"`
	ApprovalStatus string  `json:"approval_status"`
	Approver       string  `json:"approver"`
	Reason         string  `json:"reason"`
}

// IncidentState 故障事件全局状态
type IncidentState struct {
	IncidentID     string                 `json:"incident_id"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
	Status         string                 `json:"status"`
	AlertEvent     *AlertEvent            `json:"alert_event,omitempty"`
	RCAResult      *RCAResult             `json:"rca_result,omitempty"`
	HealAction     *HealAction            `json:"heal_action,omitempty"`
	ChangeDecision *ChangeDecision        `json:"change_decision,omitempty"`
	CurrentAgent   string                 `json:"current_agent"`
	RetryCount     int                    `json:"retry_count"`
	ErrorMessage   string                 `json:"error_message,omitempty"`
	Metadata       map[string]interface{} `json:"metadata"`
}

func NewIncidentState() *IncidentState {
	return &IncidentState{
		IncidentID: uuid.New().String(),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		Status:     "open",
		Metadata:   make(map[string]interface{}),
	}
}
