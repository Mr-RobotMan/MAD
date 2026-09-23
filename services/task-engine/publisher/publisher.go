package publisher

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"

	"mad/pkg/contracts"
)

const ETAUpdatedSubject = "task.eta_updated"

type Publisher interface {
	PublishETAUpdated(ctx context.Context, task contracts.Task) error
	Close()
}

type NATSPublisher struct {
	conn *nats.Conn
}

func NewNATSPublisher(url string) (*NATSPublisher, error) {
	conn, err := nats.Connect(url)
	if err != nil {
		return nil, fmt.Errorf("connect to nats: %w", err)
	}

	return &NATSPublisher{conn: conn}, nil
}

func (p *NATSPublisher) PublishETAUpdated(ctx context.Context, task contracts.Task) error {
	payload, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("marshal eta update: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- p.conn.Publish(ETAUpdatedSubject, payload)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return fmt.Errorf("publish eta update: %w", err)
		}
		return nil
	}
}

func (p *NATSPublisher) Close() {
	if p.conn != nil {
		p.conn.Close()
	}
}
