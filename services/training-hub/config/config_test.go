package config

import "testing"

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("NATS_URL", "nats://localhost:4222")

	if _, err := Load(); err == nil {
		t.Fatalf("expected missing database url error")
	}
}

func TestLoadUsesEnvironment(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("NATS_URL", "nats://example")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Port != "9090" {
		t.Fatalf("expected port from env")
	}
	if cfg.DatabaseURL != "postgres://example" {
		t.Fatalf("expected database url from env")
	}
	if cfg.NATSURL != "nats://example" {
		t.Fatalf("expected nats url from env")
	}
}
