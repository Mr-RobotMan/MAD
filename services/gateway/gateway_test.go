package gateway

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type fakeUsers struct {
	known bool
	err   error
	gotID string
}

func (f *fakeUsers) Exists(_ context.Context, id string) (bool, error) {
	f.gotID = id
	return f.known, f.err
}

func makeToken(subject string) string {
	claims, err := json.Marshal(map[string]string{"sub": subject})
	if err != nil {
		panic(err)
	}
	return "header." + base64.RawURLEncoding.EncodeToString(claims) + ".signature"
}

func TestUserIDFromToken(t *testing.T) {
	id, err := userIDFromToken("Bearer " + makeToken("user-1"))
	if err != nil || id != "user-1" {
		t.Fatalf("id=%q err=%v", id, err)
	}
	for _, header := range []string{"", "Basic token", "Bearer invalid", "Bearer header.e30.signature"} {
		if _, err := userIDFromToken(header); err == nil {
			t.Errorf("userIDFromToken(%q) expected an error", header)
		}
	}
}

func TestDownstreamPath(t *testing.T) {
	cases := []struct {
		method, path, service, target string
		ok                            bool
	}{
		{http.MethodPost, "/api/v1/tasks", "tasks", "/tasks", true},
		{http.MethodGet, "/api/v1/tasks/123", "tasks", "/tasks/123", true},
		{http.MethodPatch, "/api/v1/tasks/123/progress", "tasks", "/tasks/123", true},
		{http.MethodPost, "/api/v1/tasks/assignments", "tasks", "/machines/assignments", true},
		{http.MethodPatch, "/api/v1/training/t1/complete", "training", "/api/v1/training/t1/complete", true},
		{http.MethodGet, "/api/v1/operators/op/training", "training", "/api/v1/operators/op/training", true},
		{http.MethodGet, "/api/v1/alerts", "notification", "/api/v1/alerts", true},
		{http.MethodGet, "/api/v1/escalations", "notification", "/api/v1/escalations", true},
		{http.MethodGet, "/api/v1/other", "", "", false},
	}
	for _, testCase := range cases {
		service, path, ok := downstreamPath(testCase.method, testCase.path)
		if ok != testCase.ok || service != testCase.service || path != testCase.target {
			t.Errorf("downstreamPath(%s, %s) = (%s, %s, %v)", testCase.method, testCase.path, service, path, ok)
		}
	}
}

func TestProxyForwardsRoutesAndQuery(t *testing.T) {
	var gotPath, gotMethod, gotQuery string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod, gotQuery = r.URL.Path, r.Method, r.URL.Query().Get("operatorId")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if _, err := w.Write([]byte(`{"success":true}`)); err != nil {
			t.Errorf("write backend response: %v", err)
		}
	}))
	defer backend.Close()
	backendURL, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatal(err)
	}
	app := New(&fakeUsers{}, backendURL, backendURL, backendURL, backend.Client())
	server := httptest.NewServer(app.Routes())
	defer func() {
		server.Close()
	}()
	response, err := http.Post(server.URL+"/api/v1/tasks/assignments?operatorId=op", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Errorf("close response body: %v", err)
		}
	}()
	if response.StatusCode != http.StatusCreated || gotPath != "/machines/assignments" || gotMethod != http.MethodPost || gotQuery != "op" {
		t.Fatalf("status=%d path=%s method=%s query=%s", response.StatusCode, gotPath, gotMethod, gotQuery)
	}
}

func TestProxyUnknownAndUnavailable(t *testing.T) {
	app := New(&fakeUsers{}, nil, nil, nil, nil)
	server := httptest.NewServer(app.Routes())
	defer server.Close()
	unknown, err := http.Get(server.URL + "/api/v1/unsupported")
	if err != nil {
		t.Fatal(err)
	}
	if err := unknown.Body.Close(); err != nil {
		t.Errorf("close unknown response: %v", err)
	}
	if unknown.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown status %d", unknown.StatusCode)
	}
	missing, err := http.Get(server.URL + "/api/v1/alerts")
	if err != nil {
		t.Fatal(err)
	}
	if err := missing.Body.Close(); err != nil {
		t.Errorf("close missing service response: %v", err)
	}
	if missing.StatusCode != http.StatusBadGateway {
		t.Fatalf("unconfigured service status %d", missing.StatusCode)
	}
}

func TestWebsocketAuthenticationAndEvents(t *testing.T) {
	users := &fakeUsers{known: true}
	app := New(users, nil, nil, nil, nil)
	server := httptest.NewServer(app.Routes())
	defer server.Close()
	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	requestHeader := http.Header{"Authorization": []string{"Bearer " + makeToken("operator-1")}}
	connection, response, err := websocket.DefaultDialer.Dial(url, requestHeader)
	if err != nil {
		t.Fatalf("websocket dial: %v (response=%v)", err, response)
	}
	defer func() {
		if err := connection.Close(); err != nil {
			t.Errorf("close websocket client: %v", err)
		}
	}()
	if users.gotID != "operator-1" {
		t.Fatalf("validated user %q", users.gotID)
	}
	if err := app.PublishTaskUpdate(map[string]string{"id": "task-1"}); err != nil {
		t.Fatal(err)
	}
	if err := app.PublishAnomaly(map[string]string{"type": "unsafe_speed"}); err != nil {
		t.Fatal(err)
	}
	for _, expectedType := range []string{"task_update", "anomaly"} {
		if err := connection.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		_, message, err := connection.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		var event ClientEvent
		if err := json.Unmarshal(message, &event); err != nil || event.Type != expectedType {
			t.Fatalf("event=%s err=%v; want type %q", message, err, expectedType)
		}
	}
}

func TestWebsocketRejectsInvalidAndUnknownUsers(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		header string
		users  *fakeUsers
		status int
	}{
		{name: "missing token", users: &fakeUsers{}, status: http.StatusUnauthorized},
		{name: "unknown user", header: "Bearer " + makeToken("absent"), users: &fakeUsers{}, status: http.StatusUnauthorized},
		{name: "database error", header: "Bearer " + makeToken("user"), users: &fakeUsers{err: errors.New("db down")}, status: http.StatusInternalServerError},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			app := New(testCase.users, nil, nil, nil, nil)
			request := httptest.NewRequest(http.MethodGet, "/ws", nil)
			request.Header.Set("Authorization", testCase.header)
			response := httptest.NewRecorder()
			app.Routes().ServeHTTP(response, request)
			if response.Code != testCase.status {
				t.Fatalf("status %d, want %d", response.Code, testCase.status)
			}
		})
	}
}

func TestHubDropsOldestWhenClientIsSlow(t *testing.T) {
	updates := newHub()
	client := updates.register()
	defer updates.unregister(client)
	for index := 0; index < 17; index++ {
		if err := updates.broadcast("anomaly", fmt.Sprintf("event-%d", index)); err != nil {
			t.Fatal(err)
		}
	}
	if len(client) != cap(client) {
		t.Fatalf("buffer size %d, capacity %d", len(client), cap(client))
	}
	var event ClientEvent
	if err := json.Unmarshal(<-client, &event); err != nil {
		t.Fatal(err)
	}
	if event.Payload != "event-1" {
		t.Fatalf("oldest retained payload = %v", event.Payload)
	}
}
