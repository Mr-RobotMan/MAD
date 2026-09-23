package repository

import "testing"

func TestNewPostgresStore(t *testing.T) {
	store := NewPostgresStore(nil)
	if store == nil {
		t.Fatalf("expected store")
	}
}

func TestTaskProgressCarriesProgressFields(t *testing.T) {
	progress := TaskProgress{
		Status:           "IN_PROGRESS",
		TargetVolume:     88.5,
		EstimatedMinutes: 14,
	}

	if progress.Status != "IN_PROGRESS" {
		t.Fatalf("expected status to round trip")
	}
	if progress.TargetVolume != 88.5 {
		t.Fatalf("expected target volume to round trip")
	}
	if progress.EstimatedMinutes != 14 {
		t.Fatalf("expected estimated minutes to round trip")
	}
}
