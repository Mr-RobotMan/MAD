package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"mad/pkg/contracts"
	"mad/services/gateway"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	databaseURL := os.Getenv("DATABASE_URL")
	natsURL := os.Getenv("NATS_URL")
	if databaseURL == "" || natsURL == "" {
		return errors.New("DATABASE_URL and NATS_URL are required")
	}
	port := envOr("PORT", "8080")
	taskURL, err := serviceURL("TASK_ENGINE_URL", "http://task-engine:8081")
	if err != nil {
		return err
	}
	trainingURL, err := serviceURL("TRAINING_HUB_URL", "http://training-hub:8082")
	if err != nil {
		return err
	}
	notificationURL, err := serviceURL("NOTIFICATION_URL", "http://notification:8083")
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return err
	}
	natsConn, err := nats.Connect(natsURL)
	if err != nil {
		return err
	}
	defer natsConn.Close()
	app := gateway.New(gateway.NewPostgresUsers(pool), taskURL, trainingURL, notificationURL, http.DefaultClient)
	if _, err := natsConn.Subscribe("task.eta_updated", func(message *nats.Msg) {
		var task contracts.Task
		if err := json.Unmarshal(message.Data, &task); err != nil {
			log.Printf("decode task.eta_updated: %v", err)
			return
		}
		if err := app.PublishTaskUpdate(task); err != nil {
			log.Printf("publish websocket task event: %v", err)
		}
	}); err != nil {
		return err
	}
	if _, err := natsConn.Subscribe("anomaly.detected", func(message *nats.Msg) {
		var anomaly contracts.Anomaly
		if err := json.Unmarshal(message.Data, &anomaly); err != nil {
			log.Printf("decode anomaly.detected: %v", err)
			return
		}
		if err := app.PublishAnomaly(anomaly); err != nil {
			log.Printf("publish websocket anomaly event: %v", err)
		}
	}); err != nil {
		return err
	}
	if err := natsConn.Flush(); err != nil {
		return err
	}
	server := &http.Server{Addr: ":" + port, Handler: app.Routes(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("gateway HTTP server: %v", err)
			stop()
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func serviceURL(name, fallback string) (*url.URL, error) {
	value := envOr(name, fallback)
	target, err := url.Parse(value)
	if err != nil || target.Scheme == "" || target.Host == "" {
		return nil, fmt.Errorf("%s must be an absolute URL", name)
	}
	return target, nil
}
