package notification

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"
)

var ErrUnauthorized = errors.New("administrator role required")

// Repository persists alerts, escalations, and role lookups.
type Repository interface {
	CreateAlert(context.Context, Anomaly) (Alert, error)
	CreateEscalation(context.Context, Alert) error
	ListAlerts(context.Context, string, string) ([]Alert, error)
	ListEscalations(context.Context) ([]Escalation, error)
	UserRole(context.Context, string) (string, error)
}

// Notifier abstracts operator alert delivery.
type Notifier interface {
	Notify(context.Context, Alert) error
}

// LogNotifier logs alerts until a real delivery provider is configured.
type LogNotifier struct{}

func (LogNotifier) Notify(_ context.Context, alert Alert) error {
	log.Printf("alert dispatched: operator=%s machine=%s type=%s severity=%s", alert.OperatorID, alert.MachineID, alert.Type, alert.Severity)
	return nil
}

// Service processes anomaly events and serves alert queries.
type Service struct {
	repository Repository
	notifier   Notifier
}

func NewService(repository Repository, notifier Notifier) *Service {
	return &Service{repository: repository, notifier: notifier}
}

// ProcessAnomaly stores and dispatches WARNING/CRITICAL events. Critical
// events are additionally recorded for administrator escalation review.
func (s *Service) ProcessAnomaly(ctx context.Context, anomaly Anomaly) error {
	if anomaly.Severity != "WARNING" && anomaly.Severity != "CRITICAL" {
		return nil
	}
	if anomaly.Timestamp.IsZero() {
		anomaly.Timestamp = time.Now().UTC()
	}
	alert, err := s.repository.CreateAlert(ctx, anomaly)
	if err != nil {
		return fmt.Errorf("store alert: %w", err)
	}
	if err := s.notifier.Notify(ctx, alert); err != nil {
		return fmt.Errorf("dispatch alert: %w", err)
	}
	if anomaly.Severity == "CRITICAL" {
		if err := s.repository.CreateEscalation(ctx, alert); err != nil {
			return fmt.Errorf("store escalation: %w", err)
		}
	}
	return nil
}

func (s *Service) Alerts(ctx context.Context, operatorID, severity string) ([]Alert, error) {
	return s.repository.ListAlerts(ctx, operatorID, severity)
}

func (s *Service) Escalations(ctx context.Context, userID string) ([]Escalation, error) {
	role, err := s.repository.UserRole(ctx, userID)
	if err != nil {
		return nil, err
	}
	if role != "ADMIN" {
		return nil, ErrUnauthorized
	}
	return s.repository.ListEscalations(ctx)
}
