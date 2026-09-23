package publisher

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"

	"mad/services/training-hub/repository"
)

const TrainingAssignedSubject = "training.assigned"

type Publisher interface {
	PublishTrainingAssigned(ctx context.Context, event repository.TrainingAssignedEvent) error
	Close()
}

type NATSPublisher struct {
	conn *nats.Conn
}

func NewNATSPublisher(conn *nats.Conn) *NATSPublisher {
	return &NATSPublisher{conn: conn}
}

func (p *NATSPublisher) PublishTrainingAssigned(ctx context.Context, event repository.TrainingAssignedEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal training assigned event: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- p.conn.Publish(TrainingAssignedSubject, payload)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return fmt.Errorf("publish training assigned event: %w", err)
		}
		return nil
	}
}

func (p *NATSPublisher) Close() {
	if p.conn != nil {
		p.conn.Close()
	}
}
