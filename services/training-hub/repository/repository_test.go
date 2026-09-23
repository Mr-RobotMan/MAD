package repository

import (
	"strings"
	"testing"
)

func TestNewPostgresRepository(t *testing.T) {
	repo := NewPostgresRepository(nil)
	if repo == nil {
		t.Fatalf("expected repository")
	}
}

func TestNewIDReturnsUUIDShape(t *testing.T) {
	id, err := NewID()
	if err != nil {
		t.Fatalf("new id: %v", err)
	}

	if len(id) != 36 {
		t.Fatalf("expected uuid length 36, got %d", len(id))
	}
	if strings.Count(id, "-") != 4 {
		t.Fatalf("expected uuid hyphens")
	}
	if id[14] != '4' {
		t.Fatalf("expected version 4 uuid, got %q", id[14])
	}
}

func TestAssignedTrainingCarriesWireFields(t *testing.T) {
	score := 81.5
	training := AssignedTraining{
		ID:              "training-1",
		OperatorID:      "operator-1",
		ModuleID:        "module-1",
		Title:           "Idle Time Basics",
		DurationMinutes: 12,
		TriggerTag:      "idle_seconds",
		Status:          "COMPLETED",
		Score:           &score,
	}

	if training.OperatorID != "operator-1" {
		t.Fatalf("expected operator id")
	}
	if training.Score == nil || *training.Score != 81.5 {
		t.Fatalf("expected score")
	}
}
