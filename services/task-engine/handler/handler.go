package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"mad/pkg/contracts"
	"mad/services/task-engine/publisher"
	"mad/services/task-engine/repository"
)

type Handler struct {
	store     repository.Store
	publisher publisher.Publisher
}

type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Error   interface{} `json:"error"`
}

type ProgressRequest struct {
	Status           string  `json:"status"`
	TargetVolume     float64 `json:"targetVolume"`
	EstimatedMinutes int     `json:"estimatedMinutes"`
}

type MachineAssignmentRequest struct {
	MachineID  string `json:"machineId"`
	OperatorID string `json:"operatorId"`
}

func New(store repository.Store, publisher publisher.Publisher) *Handler {
	return &Handler{store: store, publisher: publisher}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("POST /tasks", h.createTask)
	mux.HandleFunc("GET /tasks/", h.getTask)
	mux.HandleFunc("PATCH /tasks/", h.updateTaskProgress)
	mux.HandleFunc("POST /machines/assignments", h.assignMachine)
	return mux
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, Response{Success: true, Data: map[string]string{"status": "ok"}, Error: nil})
}

func (h *Handler) createTask(w http.ResponseWriter, r *http.Request) {
	var task contracts.Task
	if err := decodeJSON(r, &task); err != nil {
		writeJSON(w, http.StatusBadRequest, Response{Success: false, Data: nil, Error: err.Error()})
		return
	}

	if err := validateTask(task); err != nil {
		writeJSON(w, http.StatusBadRequest, Response{Success: false, Data: nil, Error: err.Error()})
		return
	}

	ctx, cancel := requestContext(r)
	defer cancel()

	if err := h.store.CreateTask(ctx, &task); err != nil {
		writeJSON(w, http.StatusInternalServerError, Response{Success: false, Data: nil, Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, Response{Success: true, Data: task, Error: nil})
}

func (h *Handler) getTask(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/tasks/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, Response{Success: false, Data: nil, Error: "task id is required"})
		return
	}

	ctx, cancel := requestContext(r)
	defer cancel()

	task, err := h.store.GetTaskByID(ctx, id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, Response{Success: false, Data: nil, Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, Response{Success: true, Data: task, Error: nil})
}

func (h *Handler) updateTaskProgress(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/tasks/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, Response{Success: false, Data: nil, Error: "task id is required"})
		return
	}

	var req ProgressRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, Response{Success: false, Data: nil, Error: err.Error()})
		return
	}
	if req.Status == "" {
		writeJSON(w, http.StatusBadRequest, Response{Success: false, Data: nil, Error: "status is required"})
		return
	}

	ctx, cancel := requestContext(r)
	defer cancel()

	task, err := h.store.UpdateTaskProgress(ctx, id, repository.TaskProgress{
		Status:           req.Status,
		TargetVolume:     req.TargetVolume,
		EstimatedMinutes: req.EstimatedMinutes,
	})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, Response{Success: false, Data: nil, Error: err.Error()})
		return
	}

	if err := h.publisher.PublishETAUpdated(ctx, *task); err != nil {
		writeJSON(w, http.StatusInternalServerError, Response{Success: false, Data: nil, Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, Response{Success: true, Data: task, Error: nil})
}

func (h *Handler) assignMachine(w http.ResponseWriter, r *http.Request) {
	var req MachineAssignmentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, Response{Success: false, Data: nil, Error: err.Error()})
		return
	}
	if req.MachineID == "" || req.OperatorID == "" {
		writeJSON(w, http.StatusBadRequest, Response{Success: false, Data: nil, Error: "machineId and operatorId are required"})
		return
	}

	ctx, cancel := requestContext(r)
	defer cancel()

	if err := h.store.AssignOperator(ctx, req.MachineID, req.OperatorID); err != nil {
		writeJSON(w, http.StatusInternalServerError, Response{Success: false, Data: nil, Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, Response{Success: true, Data: req, Error: nil})
}

func decodeJSON(r *http.Request, destination interface{}) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	return nil
}

func validateTask(task contracts.Task) error {
	if task.ID == "" {
		return errors.New("id is required")
	}
	if task.Category == "" {
		return errors.New("category is required")
	}
	if task.MachineID == "" {
		return errors.New("machineId is required")
	}
	if task.OperatorID == "" {
		return errors.New("operatorId is required")
	}
	if task.Status == "" {
		return errors.New("status is required")
	}
	return nil
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
