package notification

import "time"

// Anomaly is the event published by the analytics service.
type Anomaly struct {
	OperatorID string    `json:"operatorId"`
	MachineID  string    `json:"machineId"`
	Type       string    `json:"type"`
	Severity   string    `json:"severity"`
	Timestamp  time.Time `json:"timestamp"`
}

// Alert is a persisted notification dispatched to an operator.
type Alert struct {
	ID         string    `json:"id"`
	OperatorID string    `json:"operatorId"`
	MachineID  string    `json:"machineId"`
	Type       string    `json:"type"`
	Severity   string    `json:"severity"`
	Timestamp  time.Time `json:"timestamp"`
}

// Escalation records a critical alert for administrator review.
type Escalation struct {
	ID         string    `json:"id"`
	AlertID    string    `json:"alertId"`
	OperatorID string    `json:"operatorId"`
	MachineID  string    `json:"machineId"`
	Type       string    `json:"type"`
	Severity   string    `json:"severity"`
	Timestamp  time.Time `json:"timestamp"`
}

// Envelope is the standard API response wrapper.
type Envelope struct {
	Success bool `json:"success"`
	Data    any  `json:"data"`
	Error   any  `json:"error"`
}
