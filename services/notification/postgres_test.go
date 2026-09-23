package notification

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v4"
)

func TestPostgresRepositoryCreateAlert(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	timestamp := time.Now().UTC()
	mock.ExpectQuery("INSERT INTO alerts").WithArgs("op", "machine", "unsafe_speed", "WARNING", timestamp).
		WillReturnRows(pgxmock.NewRows([]string{"id", "operator_id", "machine_id", "type", "severity", "timestamp"}).
			AddRow("id", "op", "machine", "unsafe_speed", "WARNING", timestamp))
	repository := NewPostgresRepository(mock)
	alert, err := repository.CreateAlert(context.Background(), Anomaly{OperatorID: "op", MachineID: "machine", Type: "unsafe_speed", Severity: "WARNING", Timestamp: timestamp})
	if err != nil || alert.ID != "id" {
		t.Fatalf("alert=%+v err=%v", alert, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresRepositoryCreateFailures(t *testing.T) {
	t.Run("escalation insert success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()
		mock.ExpectExec("INSERT INTO escalations").WithArgs("alert").WillReturnResult(pgxmock.NewResult("INSERT", 1))
		if err := NewPostgresRepository(mock).CreateEscalation(context.Background(), Alert{ID: "alert"}); err != nil {
			t.Fatalf("CreateEscalation: %v", err)
		}
	})
	t.Run("alert insert", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()
		mock.ExpectQuery("INSERT INTO alerts").WillReturnError(errors.New("db error"))
		if _, err := NewPostgresRepository(mock).CreateAlert(context.Background(), Anomaly{}); err == nil {
			t.Fatal("expected insert error")
		}
	})
	t.Run("escalation insert", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()
		mock.ExpectExec("INSERT INTO escalations").WithArgs("alert").WillReturnError(errors.New("db error"))
		if err := NewPostgresRepository(mock).CreateEscalation(context.Background(), Alert{ID: "alert"}); err == nil {
			t.Fatal("expected insert error")
		}
	})
}

func TestPostgresRepositoryListEscalations(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	timestamp := time.Now().UTC()
	mock.ExpectQuery("SELECT e.id::text").WillReturnRows(pgxmock.NewRows([]string{"id", "alert_id", "operator", "machine", "type", "severity", "timestamp"}).
		AddRow("e1", "a1", "op", "machine", "unsafe_speed", "CRITICAL", timestamp))
	escalations, err := NewPostgresRepository(mock).ListEscalations(context.Background())
	if err != nil || len(escalations) != 1 || escalations[0].AlertID != "a1" {
		t.Fatalf("escalations=%+v err=%v", escalations, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresRepositoryUserRole(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	mock.ExpectQuery("SELECT role FROM users").WithArgs("admin").WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("ADMIN"))
	role, err := NewPostgresRepository(mock).UserRole(context.Background(), "admin")
	if err != nil || role != "ADMIN" {
		t.Fatalf("role=%q err=%v", role, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresRepositoryListAlerts(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	timestamp := time.Now().UTC()
	mock.ExpectQuery("SELECT id::text, operator_id::text, machine_id::text,").
		WithArgs("op", "WARNING").WillReturnRows(pgxmock.NewRows([]string{"id", "operator", "machine", "type", "severity", "timestamp"}).
		AddRow("id", "op", "machine", "unsafe_speed", "WARNING", timestamp))
	alerts, err := NewPostgresRepository(mock).ListAlerts(context.Background(), "op", "WARNING")
	if err != nil || len(alerts) != 1 || alerts[0].ID != "id" {
		t.Fatalf("alerts=%+v err=%v", alerts, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresRepositoryQueryErrors(t *testing.T) {
	t.Run("list alerts query", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()
		mock.ExpectQuery("SELECT id::text, operator_id::text, machine_id::text,").WillReturnError(errors.New("db error"))
		if _, err := NewPostgresRepository(mock).ListAlerts(context.Background(), "", ""); err == nil {
			t.Fatal("expected query error")
		}
	})
	t.Run("list escalations query", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()
		mock.ExpectQuery("SELECT e.id::text").WillReturnError(errors.New("db error"))
		if _, err := NewPostgresRepository(mock).ListEscalations(context.Background()); err == nil {
			t.Fatal("expected query error")
		}
	})
	t.Run("role lookup", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()
		mock.ExpectQuery("SELECT role FROM users").WithArgs("u").WillReturnError(errors.New("db error"))
		if _, err := NewPostgresRepository(mock).UserRole(context.Background(), "u"); err == nil {
			t.Fatal("expected lookup error")
		}
	})
}
