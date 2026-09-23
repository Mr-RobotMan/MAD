package notification

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type memoryRepository struct {
	alerts      []Alert
	escalations []Escalation
	role        string
	createError error
	listError   error
	roleError   error
}

func (r *memoryRepository) CreateAlert(_ context.Context, anomaly Anomaly) (Alert, error) {
	if r.createError != nil {
		return Alert{}, r.createError
	}
	alert := Alert{ID: "alert-1", OperatorID: anomaly.OperatorID, MachineID: anomaly.MachineID,
		Type: anomaly.Type, Severity: anomaly.Severity, Timestamp: anomaly.Timestamp}
	r.alerts = append(r.alerts, alert)
	return alert, nil
}

func (r *memoryRepository) CreateEscalation(_ context.Context, alert Alert) error {
	r.escalations = append(r.escalations, Escalation{ID: "escalation-1", AlertID: alert.ID,
		OperatorID: alert.OperatorID, MachineID: alert.MachineID, Type: alert.Type,
		Severity: alert.Severity, Timestamp: alert.Timestamp})
	return nil
}

func (r *memoryRepository) ListAlerts(_ context.Context, operatorID, severity string) ([]Alert, error) {
	if r.listError != nil {
		return nil, r.listError
	}
	filtered := make([]Alert, 0)
	for _, alert := range r.alerts {
		if (operatorID == "" || alert.OperatorID == operatorID) && (severity == "" || alert.Severity == severity) {
			filtered = append(filtered, alert)
		}
	}
	return filtered, nil
}

func (r *memoryRepository) ListEscalations(context.Context) ([]Escalation, error) {
	return r.escalations, nil
}

func (r *memoryRepository) UserRole(context.Context, string) (string, error) {
	return r.role, r.roleError
}

type recordingNotifier struct{ alerts []Alert }

func (n *recordingNotifier) Notify(_ context.Context, alert Alert) error {
	n.alerts = append(n.alerts, alert)
	return nil
}

type failingNotifier struct{}

func (failingNotifier) Notify(context.Context, Alert) error {
	return errors.New("provider unavailable")
}

func TestProcessAnomalyDispatchesAndEscalates(t *testing.T) {
	repository := &memoryRepository{}
	notifier := &recordingNotifier{}
	service := NewService(repository, notifier)
	for _, severity := range []string{"INFO", "WARNING", "CRITICAL"} {
		err := service.ProcessAnomaly(context.Background(), Anomaly{OperatorID: "op-1", MachineID: "m-1", Type: "unsafe_speed", Severity: severity})
		if err != nil {
			t.Fatalf("ProcessAnomaly(%s): %v", severity, err)
		}
	}
	if len(repository.alerts) != 2 || len(notifier.alerts) != 2 || len(repository.escalations) != 1 {
		t.Fatalf("stored=%d notified=%d escalated=%d", len(repository.alerts), len(notifier.alerts), len(repository.escalations))
	}
	if repository.alerts[0].Timestamp.IsZero() {
		t.Fatal("missing default event timestamp")
	}
}

func TestProcessAnomalyReturnsRepositoryError(t *testing.T) {
	want := errors.New("storage unavailable")
	service := NewService(&memoryRepository{createError: want}, &recordingNotifier{})
	if err := service.ProcessAnomaly(context.Background(), Anomaly{Severity: "WARNING"}); !errors.Is(err, want) {
		t.Fatalf("got %v, want wrapped %v", err, want)
	}
}

func TestProcessAnomalyReturnsNotifierError(t *testing.T) {
	service := NewService(&memoryRepository{}, failingNotifier{})
	if err := service.ProcessAnomaly(context.Background(), Anomaly{Severity: "WARNING"}); err == nil {
		t.Fatal("expected notifier error")
	}
}

func TestEscalationsRequireAdmin(t *testing.T) {
	repository := &memoryRepository{role: "OPERATOR"}
	service := NewService(repository, &recordingNotifier{})
	if _, err := service.Escalations(context.Background(), "user-1"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("got %v, want ErrUnauthorized", err)
	}
	repository.role = "ADMIN"
	escalations, err := service.Escalations(context.Background(), "admin-1")
	if err != nil || len(escalations) != 0 {
		t.Fatalf("escalations=%v err=%v", escalations, err)
	}
}

func TestEscalationsPropagateRoleLookupError(t *testing.T) {
	want := errors.New("role lookup failed")
	service := NewService(&memoryRepository{roleError: want}, &recordingNotifier{})
	if _, err := service.Escalations(context.Background(), "user-1"); !errors.Is(err, want) {
		t.Fatalf("got %v, want wrapped %v", err, want)
	}
}

func TestLogNotifier(t *testing.T) {
	if err := (LogNotifier{}).Notify(context.Background(), Alert{OperatorID: "op", Severity: "WARNING"}); err != nil {
		t.Fatalf("Notify returned error: %v", err)
	}
}

func TestHTTPRoutesAndFilters(t *testing.T) {
	repository := &memoryRepository{role: "ADMIN", alerts: []Alert{
		{ID: "a1", OperatorID: "op-1", Severity: "WARNING", Timestamp: time.Now()},
		{ID: "a2", OperatorID: "op-2", Severity: "CRITICAL", Timestamp: time.Now()},
	}}
	service := NewService(repository, &recordingNotifier{})
	handler := NewHTTPHandler(service)
	server := httptest.NewServer(handler)
	defer server.Close()

	response, err := http.Get(server.URL + "/api/v1/alerts?operatorId=op-1&severity=WARNING")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("alerts status %d", response.StatusCode)
	}

	request, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/escalations", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-User-ID", "admin-1")
	response2, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response2.Body.Close()
	if response2.StatusCode != http.StatusOK {
		t.Fatalf("admin escalations status %d", response2.StatusCode)
	}
}

func TestEscalationsHTTPAuthorization(t *testing.T) {
	service := NewService(&memoryRepository{role: "OPERATOR"}, &recordingNotifier{})
	handler := NewHTTPHandler(service)
	for _, testCase := range []struct {
		name   string
		userID string
		want   int
	}{{name: "missing identity", want: http.StatusUnauthorized}, {name: "operator", userID: "operator", want: http.StatusForbidden}} {
		t.Run(testCase.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/v1/escalations", nil)
			if testCase.userID != "" {
				request.Header.Set("X-User-ID", testCase.userID)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != testCase.want {
				t.Fatalf("status %d, want %d", response.Code, testCase.want)
			}
		})
	}
}

func TestHTTPRepositoryErrors(t *testing.T) {
	service := NewService(&memoryRepository{listError: errors.New("query failed"), roleError: errors.New("role failed")}, &recordingNotifier{})
	handler := NewHTTPHandler(service)
	for _, path := range []string{"/api/v1/alerts", "/api/v1/escalations"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("X-User-ID", "admin")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusInternalServerError {
			t.Fatalf("%s status %d, want 500", path, response.Code)
		}
	}
}

func TestWriteJSONLogsEncodingError(t *testing.T) {
	response := httptest.NewRecorder()
	writeJSON(response, http.StatusOK, Envelope{Success: true, Data: make(chan int)})
	if response.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", response.Code)
	}
}
