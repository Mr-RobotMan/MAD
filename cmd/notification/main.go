package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"mad/services/notification"
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
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
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

	service := notification.NewService(notification.NewPostgresRepository(pool), notification.LogNotifier{})
	_, err = natsConn.Subscribe("anomaly.detected", func(message *nats.Msg) {
		var anomaly notification.Anomaly
		if decodeErr := json.Unmarshal(message.Data, &anomaly); decodeErr != nil {
			log.Printf("decode anomaly event: %v", decodeErr)
			return
		}
		if processErr := service.ProcessAnomaly(ctx, anomaly); processErr != nil {
			log.Printf("process anomaly event: %v", processErr)
		}
	})
	if err != nil {
		return err
	}
	if err := natsConn.Flush(); err != nil {
		return err
	}

	server := &http.Server{Addr: ":" + port, Handler: notification.NewHTTPHandler(service), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if serveErr := server.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			log.Printf("HTTP server: %v", serveErr)
			stop()
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}
