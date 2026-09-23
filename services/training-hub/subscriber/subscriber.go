package subscriber

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"

	"mad/pkg/contracts"
	"mad/services/training-hub/publisher"
	"mad/services/training-hub/repository"
)

const AnomalyDetectedSubject = "anomaly.detected"

type Subscriber struct {
	conn      *nats.Conn
	repo      repository.Repository
	publisher publisher.Publisher
}

func New(conn *nats.Conn, repo repository.Repository, publisher publisher.Publisher) *Subscriber {
	return &Subscriber{conn: conn, repo: repo, publisher: publisher}
}

func (s *Subscriber) Start(ctx context.Context) (*nats.Subscription, error) {
	sub, err := s.conn.Subscribe(AnomalyDetectedSubject, func(msg *nats.Msg) {
		if err := s.HandleMessage(ctx, msg.Data); err != nil {
			log.Printf("handle anomaly detected message: %v", err)
		}
	})
	if err != nil {
		return nil, fmt.Errorf("subscribe to anomaly detected: %w", err)
	}

	return sub, nil
}

func (s *Subscriber) HandleMessage(ctx context.Context, payload []byte) error {
	var anomaly contracts.Anomaly
	if err := json.Unmarshal(payload, &anomaly); err != nil {
		return fmt.Errorf("decode anomaly detected payload: %w", err)
	}
	if anomaly.OperatorID == "" || anomaly.Type == "" {
		return fmt.Errorf("operatorId and type are required")
	}

	messageCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	events, err := s.repo.AssignModulesForAnomaly(messageCtx, anomaly.OperatorID, anomaly.Type)
	if err != nil {
		return err
	}

	for _, event := range events {
		if err := s.publisher.PublishTrainingAssigned(messageCtx, event); err != nil {
			return err
		}
	}

	return nil
}
