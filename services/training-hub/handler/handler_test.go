package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mad/services/training-hub/repository"
)

type fakeRepository struct {
	trainings        []repository.AssignedTraining
	completed        *repository.AssignedTraining
	completeID       string
	completeScore    float64
	listOperatorID   string
	err              error
	assignedOperator string
	assignedType     string
}

func (r *fakeRepository) AssignModulesForAnomaly(ctx context.Context, operatorID string, anomalyType string) ([]repository.TrainingAssignedEvent, error) {
	if r.err != nil {
		return nil, r.err
	}
	r.assignedOperator = operatorID
	r.assignedType = anomalyType
	return []repository.TrainingAssignedEvent{{
		OperatorID: operatorID,
		ModuleID:   "module-1",
		Title:      "Seatbelt Refresher",
		TriggerTag: anomalyType,
	}}, nil
}

func (r *fakeRepository) ListOperatorTraining(ctx context.Context, operatorID string) ([]repository.AssignedTraining, error) {
	if r.err != nil {
		return nil, r.err
	}
	r.listOperatorID = operatorID
	return r.trainings, nil
}

func (r *fakeRepository) CompleteTraining(ctx context.Context, id string, score float64) (*repository.AssignedTraining, error) {
	if r.err != nil {
		return nil, r.err
	}
	r.completeID = id
	r.completeScore = score
	if r.completed != nil {
		return r.completed, nil
	}
	return &repository.AssignedTraining{ID: id, Status: "COMPLETED", Score: &score}, nil
}

func TestListOperatorTraining(t *testing.T) {
	score := 93.5
	repo := &fakeRepository{trainings: []repository.AssignedTraining{{
		ID:         "training-1",
		OperatorID: "operator-1",
		ModuleID:   "module-1",
		Title:      "Seatbelt Refresher",
		TriggerTag: "seatbelt",
		Status:     "COMPLETED",
		Score:      &score,
	}}}
	server := httptest.NewServer(New(repo).Routes())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/operators/operator-1/training")
	if err != nil {
		t.Fatalf("get operator training: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	if repo.listOperatorID != "operator-1" {
		t.Fatalf("expected operator id to be passed to repository")
	}
}

func TestListOperatorTrainingRejectsBadRoute(t *testing.T) {
	server := httptest.NewServer(New(&fakeRepository{}).Routes())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/operators/operator-1")
	if err != nil {
		t.Fatalf("get bad route: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestCompleteTraining(t *testing.T) {
	repo := &fakeRepository{}
	server := httptest.NewServer(New(repo).Routes())
	defer server.Close()

	body := []byte(`{"score":88.25}`)
	req, err := http.NewRequest(http.MethodPatch, server.URL+"/api/v1/training/training-1/complete", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create complete request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("patch complete training: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	if repo.completeID != "training-1" {
		t.Fatalf("expected training id to be passed to repository")
	}
	if repo.completeScore != 88.25 {
		t.Fatalf("expected score to be passed to repository")
	}
}

func TestCompleteTrainingRejectsUnknownJSON(t *testing.T) {
	server := httptest.NewServer(New(&fakeRepository{}).Routes())
	defer server.Close()

	body := []byte(`{"score":88.25,"extra":true}`)
	req, err := http.NewRequest(http.MethodPatch, server.URL+"/api/v1/training/training-1/complete", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create complete request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("patch complete training: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestCompleteTrainingNotFound(t *testing.T) {
	server := httptest.NewServer(New(&fakeRepository{err: sql.ErrNoRows}).Routes())
	defer server.Close()

	body := []byte(`{"score":70}`)
	req, err := http.NewRequest(http.MethodPatch, server.URL+"/api/v1/training/missing/complete", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create complete request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("patch complete training: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestCompleteTrainingRejectsMissingCompleteSuffix(t *testing.T) {
	server := httptest.NewServer(New(&fakeRepository{}).Routes())
	defer server.Close()

	body, err := json.Marshal(CompleteTrainingRequest{Score: 91})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req, err := http.NewRequest(http.MethodPatch, server.URL+"/api/v1/training/training-1", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create complete request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("patch complete training: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}
