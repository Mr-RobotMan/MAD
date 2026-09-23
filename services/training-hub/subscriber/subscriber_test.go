package subscriber

import (
	"context"
	"encoding/json"
	"testing"

	"mad/pkg/contracts"
	"mad/services/training-hub/repository"
)

type fakeRepository struct {
	operatorID  string
	anomalyType string
	events      []repository.TrainingAssignedEvent
	err         error
}

func (r *fakeRepository) AssignModulesForAnomaly(ctx context.Context, operatorID string, anomalyType string) ([]repository.TrainingAssignedEvent, error) {
	if r.err != nil {
		return nil, r.err
	}
	r.operatorID = operatorID
	r.anomalyType = anomalyType
	return r.events, nil
}

func (r *fakeRepository) ListOperatorTraining(ctx context.Context, operatorID string) ([]repository.AssignedTraining, error) {
	return nil, nil
}

func (r *fakeRepository) CompleteTraining(ctx context.Context, id string, score float64) (*repository.AssignedTraining, error) {
	return nil, nil
}

type fakePublisher struct {
	published []repository.TrainingAssignedEvent
	err       error
}

func (p *fakePublisher) PublishTrainingAssigned(ctx context.Context, event repository.TrainingAssignedEvent) error {
	if p.err != nil {
		return p.err
	}
	p.published = append(p.published, event)
	return nil
}

func (p *fakePublisher) Close() {}

func TestHandleMessageAssignsAndPublishesTraining(t *testing.T) {
	repo := &fakeRepository{events: []repository.TrainingAssignedEvent{{
		OperatorID: "operator-1",
		ModuleID:   "module-1",
		Title:      "Seatbelt Refresher",
		TriggerTag: "seatbelt",
	}}}
	pub := &fakePublisher{}
	sub := New(nil, repo, pub)

	payload, err := json.Marshal(contracts.Anomaly{OperatorID: "operator-1", Type: "seatbelt"})
	if err != nil {
		t.Fatalf("marshal anomaly: %v", err)
	}

	if err := sub.HandleMessage(context.Background(), payload); err != nil {
		t.Fatalf("handle message: %v", err)
	}

	if repo.operatorID != "operator-1" || repo.anomalyType != "seatbelt" {
		t.Fatalf("expected anomaly to drive repository assignment")
	}
	if len(pub.published) != 1 {
		t.Fatalf("expected one training assigned event")
	}
}

func TestHandleMessageRejectsInvalidPayload(t *testing.T) {
	sub := New(nil, &fakeRepository{}, &fakePublisher{})
	if err := sub.HandleMessage(context.Background(), []byte(`{`)); err == nil {
		t.Fatalf("expected invalid payload error")
	}
}

func TestHandleMessageRequiresOperatorAndType(t *testing.T) {
	sub := New(nil, &fakeRepository{}, &fakePublisher{})
	payload, err := json.Marshal(contracts.Anomaly{OperatorID: "operator-1"})
	if err != nil {
		t.Fatalf("marshal anomaly: %v", err)
	}

	if err := sub.HandleMessage(context.Background(), payload); err == nil {
		t.Fatalf("expected missing type error")
	}
}
