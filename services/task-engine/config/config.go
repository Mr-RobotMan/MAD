package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port        string
	DatabaseURL string
	NATSURL     string
}

func Load() (Config, error) {
	cfg := Config{
		Port:        getEnv("PORT", "8081"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		NATSURL:     os.Getenv("NATS_URL"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.NATSURL == "" {
		return Config{}, fmt.Errorf("NATS_URL is required")
	}

	return cfg, nil
}

func getEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
