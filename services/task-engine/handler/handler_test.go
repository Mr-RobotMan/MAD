package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"mad/pkg/contracts"
	"mad/services/task-engine/repository"
)

type fakeStore struct {
	task             contracts.Task
	createdTask      *contracts.Task
	progress         repository.TaskProgress
	assignedMachine  string
	assignedOperator string
	err              error
}

func (s *fakeStore) CreateTask(ctx context.Context, task *contracts.Task) error {
	if s.err != nil {
		return s.err
	}
	s.createdTask = task
	return nil
}

func (s *fakeStore) GetTaskByID(ctx context.Context, id string) (*contracts.Task, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &s.task, nil
}

func (s *fakeStore) UpdateTaskProgress(ctx context.Context, id string, progress repository.TaskProgress) (*contracts.Task, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.progress = progress
	s.task.ID = id
	s.task.Status = progress.Status
	s.task.TargetVolume = progress.TargetVolume
	s.task.EstimatedMinutes = progress.EstimatedMinutes
	return &s.task, nil
}

func (s *fakeStore) AssignOperator(ctx context.Context, machineID string, operatorID string) error {
	if s.err != nil {
		return s.err
	}
	s.assignedMachine = machineID
	s.assignedOperator = operatorID
	return nil
}

type fakePublisher struct {
	published *contracts.Task
	err       error
}

func (p *fakePublisher) PublishETAUpdated(ctx context.Context, task contracts.Task) error {
	if p.err != nil {
		return p.err
	}
	p.published = &task
	return nil
}

func (p *fakePublisher) Close() {}

func TestCreateTask(t *testing.T) {
	store := &fakeStore{}
	server := httptest.NewServer(New(store, &fakePublisher{}).Routes())
	defer server.Close()

	task := contracts.Task{
		ID:               "task-1",
		Category:         "hauling",
		MachineID:        "machine-1",
		OperatorID:       "operator-1",
		Status:           "ASSIGNED",
		TargetVolume:     120,
		EstimatedMinutes: 45,
	}
	body, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal task: %v", err)
	}

	resp, err := http.Post(server.URL+"/tasks", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post task: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}
	if store.createdTask == nil || store.createdTask.ID != task.ID {
		t.Fatalf("expected task to be stored")
	}
}

func TestCreateTaskRejectsMissingMachine(t *testing.T) {
	server := httptest.NewServer(New(&fakeStore{}, &fakePublisher{}).Routes())
	defer server.Close()

	body := []byte(`{"id":"task-1","category":"hauling","operatorId":"operator-1","status":"ASSIGNED"}`)
	resp, err := http.Post(server.URL+"/tasks", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post task: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestUpdateTaskProgressPublishesETA(t *testing.T) {
	store := &fakeStore{task: contracts.Task{Category: "grading", MachineID: "machine-1", OperatorID: "operator-1"}}
	pub := &fakePublisher{}
	server := httptest.NewServer(New(store, pub).Routes())
	defer server.Close()

	body := []byte(`{"status":"IN_PROGRESS","targetVolume":75.5,"estimatedMinutes":20}`)
	req, err := http.NewRequest(http.MethodPatch, server.URL+"/tasks/task-1", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create patch request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("patch task: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	if pub.published == nil || pub.published.ID != "task-1" {
		t.Fatalf("expected eta update to be published")
	}
	if store.progress.EstimatedMinutes != 20 {
		t.Fatalf("expected progress estimated minutes to be stored")
	}
}

func TestUpdateTaskProgressReportsPublishError(t *testing.T) {
	store := &fakeStore{task: contracts.Task{Category: "grading", MachineID: "machine-1", OperatorID: "operator-1"}}
	pub := &fakePublisher{err: errors.New("nats down")}
	server := httptest.NewServer(New(store, pub).Routes())
	defer server.Close()

	body := []byte(`{"status":"IN_PROGRESS","targetVolume":75.5,"estimatedMinutes":20}`)
	req, err := http.NewRequest(http.MethodPatch, server.URL+"/tasks/task-1", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create patch request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("patch task: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, resp.StatusCode)
	}
}

func TestAssignMachine(t *testing.T) {
	store := &fakeStore{}
	server := httptest.NewServer(New(store, &fakePublisher{}).Routes())
	defer server.Close()

	body := []byte(`{"machineId":"machine-1","operatorId":"operator-1"}`)
	resp, err := http.Post(server.URL+"/machines/assignments", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post assignment: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	if store.assignedMachine != "machine-1" || store.assignedOperator != "operator-1" {
		t.Fatalf("expected assignment to be stored")
	}
}
