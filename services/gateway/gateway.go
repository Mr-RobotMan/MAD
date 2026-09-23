package gateway

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type UserRepository interface {
	Exists(ctx context.Context, userID string) (bool, error)
}

type Envelope struct {
	Success bool `json:"success"`
	Data    any  `json:"data"`
	Error   any  `json:"error"`
}

type ClientEvent struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

type hub struct {
	mu      sync.RWMutex
	clients map[chan []byte]struct{}
}

func newHub() *hub { return &hub{clients: make(map[chan []byte]struct{})} }

func (h *hub) register() chan []byte {
	client := make(chan []byte, 16)
	h.mu.Lock()
	h.clients[client] = struct{}{}
	h.mu.Unlock()
	return client
}

func (h *hub) unregister(client chan []byte) {
	h.mu.Lock()
	delete(h.clients, client)
	close(client)
	h.mu.Unlock()
}

func (h *hub) broadcast(eventType string, payload any) error {
	message, err := json.Marshal(ClientEvent{Type: eventType, Payload: payload})
	if err != nil {
		return err
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		select {
		case client <- message:
		default:
			// Drop the oldest queued update so a slow client can catch up.
			select {
			case <-client:
			default:
			}
			select {
			case client <- message:
			default:
			}
		}
	}
	return nil
}

type Gateway struct {
	users       UserRepository
	hub         *hub
	proxyClient *http.Client
	services    map[string]*url.URL
	upgrader    websocket.Upgrader
}

func New(users UserRepository, taskEngine, trainingHub, notification *url.URL, proxyClient *http.Client) *Gateway {
	if proxyClient == nil {
		proxyClient = http.DefaultClient
	}
	return &Gateway{
		users: users, hub: newHub(), proxyClient: proxyClient,
		services: map[string]*url.URL{"tasks": taskEngine, "training": trainingHub, "notification": notification},
		upgrader: websocket.Upgrader{CheckOrigin: func(request *http.Request) bool {
			origin := request.Header.Get("Origin")
			if origin == "" {
				return true
			}
			parsed, err := url.Parse(origin)
			return err == nil && parsed.Host == request.Host
		}},
	}
}

func (g *Gateway) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", g.health)
	mux.HandleFunc("GET /ws", g.websocket)
	mux.HandleFunc("/api/v1/", g.proxy)
	return mux
}

func (g *Gateway) PublishTaskUpdate(payload any) error {
	return g.hub.broadcast("task_update", payload)
}
func (g *Gateway) PublishAnomaly(payload any) error { return g.hub.broadcast("anomaly", payload) }

func (g *Gateway) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, Envelope{Success: true, Data: map[string]string{"status": "ok"}})
}

func (g *Gateway) websocket(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromToken(r.Header.Get("Authorization"))
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, Envelope{Success: false, Error: "valid bearer token required"})
		return
	}
	ok, err := g.users.Exists(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Success: false, Error: "user lookup failed"})
		return
	}
	if !ok {
		writeJSON(w, http.StatusUnauthorized, Envelope{Success: false, Error: "unknown user"})
		return
	}
	connection, err := g.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer func() {
		if err := connection.Close(); err != nil {
			log.Printf("close websocket: %v", err)
		}
	}()
	client := g.hub.register()
	defer g.hub.unregister(client)
	writeDone := make(chan struct{})
	go func() {
		defer close(writeDone)
		for message := range client {
			if err := connection.WriteMessage(websocket.TextMessage, message); err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					log.Printf("write websocket event: %v", err)
				}
				return
			}
		}
	}()
	for {
		if _, _, err := connection.ReadMessage(); err != nil {
			break
		}
	}
	if err := connection.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), websocketControlDeadline()); err != nil {
		log.Printf("send websocket close: %v", err)
	}
	if err := connection.Close(); err != nil {
		log.Printf("close websocket after read: %v", err)
	}
	<-writeDone
}

func (g *Gateway) proxy(w http.ResponseWriter, r *http.Request) {
	serviceName, targetPath, ok := downstreamPath(r.Method, r.URL.Path)
	if !ok {
		writeJSON(w, http.StatusNotFound, Envelope{Success: false, Error: "no downstream route for path"})
		return
	}
	target := g.services[serviceName]
	if target == nil {
		writeJSON(w, http.StatusBadGateway, Envelope{Success: false, Error: "downstream service is not configured"})
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = roundTripper{client: g.proxyClient}
	proxy.Director = func(request *http.Request) {
		request.URL.Scheme = target.Scheme
		request.URL.Host = target.Host
		request.URL.Path = targetPath
		request.URL.RawPath = ""
		request.Host = target.Host
		request.RequestURI = ""
	}
	proxy.ErrorHandler = func(response http.ResponseWriter, _ *http.Request, err error) {
		log.Printf("gateway proxy to %s failed: %v", serviceName, err)
		writeJSON(response, http.StatusBadGateway, Envelope{Success: false, Error: "downstream service unavailable"})
	}
	proxy.ServeHTTP(w, r)
}

type roundTripper struct{ client *http.Client }

func (t roundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return t.client.Do(request)
}

func downstreamPath(method, requestPath string) (string, string, bool) {
	switch {
	case requestPath == "/api/v1/tasks" || strings.HasPrefix(requestPath, "/api/v1/tasks/"):
		suffix := strings.TrimPrefix(requestPath, "/api/v1/tasks")
		if suffix == "/assignments" && method == http.MethodPost {
			return "tasks", "/machines/assignments", true
		}
		if strings.HasSuffix(suffix, "/progress") && method == http.MethodPatch {
			suffix = strings.TrimSuffix(suffix, "/progress")
		}
		return "tasks", "/tasks" + suffix, true
	case strings.HasPrefix(requestPath, "/api/v1/training/") || strings.HasPrefix(requestPath, "/api/v1/operators/"):
		return "training", requestPath, true
	case requestPath == "/api/v1/alerts" || strings.HasPrefix(requestPath, "/api/v1/alerts/") ||
		requestPath == "/api/v1/escalations" || strings.HasPrefix(requestPath, "/api/v1/escalations/"):
		return "notification", requestPath, true
	default:
		return "", "", false
	}
}

func userIDFromToken(header string) (string, error) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", errors.New("missing bearer token")
	}
	tokenParts := strings.Split(parts[1], ".")
	if len(tokenParts) != 3 {
		return "", errors.New("malformed JWT")
	}
	// This scaffold extracts the JWT subject and verifies its user row. Token
	// signature verification is intentionally delegated to future gateway auth.
	claimsJSON, err := base64.RawURLEncoding.DecodeString(tokenParts[1])
	if err != nil {
		return "", errors.New("malformed JWT claims")
	}
	var claims struct {
		Subject string `json:"sub"`
	}
	if err := json.Unmarshal(claimsJSON, &claims); err != nil || claims.Subject == "" {
		return "", errors.New("JWT subject is required")
	}
	return claims.Subject, nil
}

func writeJSON(w http.ResponseWriter, status int, value Envelope) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write gateway response: %v", err)
	}
}

func websocketControlDeadline() time.Time { return time.Now().Add(time.Second) }
