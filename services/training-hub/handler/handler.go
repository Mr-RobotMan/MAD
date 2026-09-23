package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"mad/services/training-hub/repository"
)

type Handler struct {
	repo repository.Repository
}

type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Error   interface{} `json:"error"`
}

type CompleteTrainingRequest struct {
	Score float64 `json:"score"`
}

func New(repo repository.Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("GET /api/v1/operators/", h.listOperatorTraining)
	mux.HandleFunc("PATCH /api/v1/training/", h.completeTraining)
	return mux
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, Response{Success: true, Data: map[string]string{"status": "ok"}, Error: nil})
}

func (h *Handler) listOperatorTraining(w http.ResponseWriter, r *http.Request) {
	operatorID, ok := parseOperatorTrainingPath(r.URL.Path)
	if !ok {
		writeJSON(w, http.StatusNotFound, Response{Success: false, Data: nil, Error: "route not found"})
		return
	}

	ctx, cancel := requestContext(r)
	defer cancel()

	trainings, err := h.repo.ListOperatorTraining(ctx, operatorID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Response{Success: false, Data: nil, Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, Response{Success: true, Data: trainings, Error: nil})
}

func (h *Handler) completeTraining(w http.ResponseWriter, r *http.Request) {
	trainingID, ok := parseCompleteTrainingPath(r.URL.Path)
	if !ok {
		writeJSON(w, http.StatusNotFound, Response{Success: false, Data: nil, Error: "route not found"})
		return
	}

	var req CompleteTrainingRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, Response{Success: false, Data: nil, Error: err.Error()})
		return
	}

	ctx, cancel := requestContext(r)
	defer cancel()

	training, err := h.repo.CompleteTraining(ctx, trainingID, req.Score)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, Response{Success: false, Data: nil, Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, Response{Success: true, Data: training, Error: nil})
}

func parseOperatorTrainingPath(path string) (string, bool) {
	const prefix = "/api/v1/operators/"
	const suffix = "/training"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return "", false
	}

	operatorID := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	return operatorID, operatorID != ""
}

func parseCompleteTrainingPath(path string) (string, bool) {
	const prefix = "/api/v1/training/"
	const suffix = "/complete"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return "", false
	}

	trainingID := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	return trainingID, trainingID != ""
}

func decodeJSON(r *http.Request, destination interface{}) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(destination)
}

func requestContext(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), 5*time.Second)
}

func writeJSON(w http.ResponseWriter, status int, response Response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
