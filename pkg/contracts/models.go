package contracts

import "time"

type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

type Machine struct {
	ID                string `json:"id"`
	Model             string `json:"model"`
	Status            string `json:"status"`
	CurrentOperatorID string `json:"currentOperatorId"`
}

type Task struct {
	ID               string  `json:"id"`
	Category         string  `json:"category"`
	MachineID        string  `json:"machineId"`
	OperatorID       string  `json:"operatorId"`
	Status           string  `json:"status"`
	TargetVolume     float64 `json:"targetVolume"`
	EstimatedMinutes int     `json:"estimatedMinutes"`
}

type Anomaly struct {
	ID         string    `json:"id"`
	OperatorID string    `json:"operatorId"`
	MachineID  string    `json:"machineId"`
	Type       string    `json:"type"`
	Severity   string    `json:"severity"`
	Timestamp  time.Time `json:"timestamp"`
}

type TrainingModule struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	DurationMinutes int    `json:"durationMinutes"`
	TriggerTag      string `json:"triggerTag"`
}

type OperatorTraining struct {
	ID         string  `json:"id"`
	OperatorID string  `json:"operatorId"`
	ModuleID   string  `json:"moduleId"`
	Status     string  `json:"status"`
	Score      float64 `json:"score"`
}
