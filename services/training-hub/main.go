package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nats-io/nats.go"

	"mad/services/training-hub/config"
	"mad/services/training-hub/handler"
	"mad/services/training-hub/publisher"
	"mad/services/training-hub/repository"
	"mad/services/training-hub/subscriber"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("close database: %v", err)
		}
	}()

	natsConn, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		log.Fatalf("connect nats: %v", err)
	}
	defer natsConn.Close()

	repo := repository.NewPostgresRepository(db)
	eventPublisher := publisher.NewNATSPublisher(natsConn)
	defer eventPublisher.Close()

	anomalySubscriber := subscriber.New(natsConn, repo, eventPublisher)
	if _, err := anomalySubscriber.Start(context.Background()); err != nil {
		log.Fatalf("start anomaly subscriber: %v", err)
	}

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: handler.New(repo).Routes(),
	}

	log.Printf("training-hub listening on :%s", cfg.Port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve training-hub: %v", err)
	}
}
