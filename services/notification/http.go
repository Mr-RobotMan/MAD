package notification

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
)

type apiHandler struct{ service *Service }

func NewHTTPHandler(service *Service) http.Handler {
	h := &apiHandler{service: service}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/alerts", h.alerts)
	mux.HandleFunc("GET /api/v1/escalations", h.escalations)
	return mux
}

func (h *apiHandler) alerts(w http.ResponseWriter, r *http.Request) {
	alerts, err := h.service.Alerts(r.Context(), r.URL.Query().Get("operatorId"), r.URL.Query().Get("severity"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load alerts")
		return
	}
	writeJSON(w, http.StatusOK, Envelope{Success: true, Data: alerts, Error: nil})
}

func (h *apiHandler) escalations(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "X-User-ID is required")
		return
	}
	escalations, err := h.service.Escalations(r.Context(), userID)
	if errors.Is(err, ErrUnauthorized) {
		writeError(w, http.StatusForbidden, "administrator role required")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load escalations")
		return
	}
	writeJSON(w, http.StatusOK, Envelope{Success: true, Data: escalations, Error: nil})
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, Envelope{Success: false, Data: nil, Error: message})
}

func writeJSON(w http.ResponseWriter, status int, value Envelope) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode response: %v", err)
	}
}
