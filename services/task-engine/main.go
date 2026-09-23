package main

import (
	"database/sql"
	"log"
	"net/http"

	_ "github.com/jackc/pgx/v5/stdlib"

	"mad/services/task-engine/config"
	"mad/services/task-engine/handler"
	"mad/services/task-engine/publisher"
	"mad/services/task-engine/repository"
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

	natsPublisher, err := publisher.NewNATSPublisher(cfg.NATSURL)
	if err != nil {
		log.Fatalf("connect nats: %v", err)
	}
	defer natsPublisher.Close()

	store := repository.NewPostgresStore(db)
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: handler.New(store, natsPublisher).Routes(),
	}

	log.Printf("task-engine listening on :%s", cfg.Port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve task-engine: %v", err)
	}
}
