# Architecture Specification: CAT Smart Operator Assistant

## Overview
A microservices-based operator assistance platform built with Go, Python (FastAPI), React + Tailwind, PostgreSQL, and NATS JetStream, running fully containerized on Docker & Kubernetes (Kind/K3s).

## Technology Stack
- **API Gateway & Core Microservices:** Go 1.22+ (Chi or Fiber HTTP framework)
- **Analytics & Anomaly ML Service:** Python 3.11+ with FastAPI, Scikit-learn, Pandas
- **Frontend:** React (Vite) + Tailwind CSS + Lucide Icons + Recharts
- **Database:** PostgreSQL 16 (Relational state & transactional history)
- **Message Streaming / Pub-Sub:** NATS JetStream (or Redis Streams)
- **Synthetic Simulator:** Go/Python binaries running in Kubernetes Pods

## System Diagram & Service Communication

```text
[ Simulator Pods (K8s) ] --(HTTP/WebSocket)--> [ Ingestion Service (Go) ]
                                               │
                                               ▼
                                      NATS Stream: telemetry.raw
                                               │
                         ┌─────────────────────┴─────────────────────┐
                         ▼                                           ▼
          [ Analytics & Anomaly (Python) ]              [ Task & ETA Engine (Go) ]
                         │                                           │
               NATS: anomaly.detected                       NATS: task.eta_updated
                         │                                           │
             ┌───────────┴───────────┐                               │
             ▼                       ▼                               │
     [ Training Hub (Go) ]  [ Notification Service (Go) ]            │
             │                       │                               │
             └───────────────────────┴───────────────────────────────┤
                                                                     ▼
                                                        [ Dashboard API Gateway (Go) ]
                                                                     │
                                                              (WebSocket / Push)
                                                                     │
                                                                     ▼
                                                        [ React Frontend Dashboard ]
```

## Service Responsibilities

1. **`services/gateway` (Go):** Single entry point, authentication, WebSocket connection broker to React UI.
2. **`services/analytics-ml` (Python/FastAPI):** Isolation Forest / rule-based anomaly detection & XGBoost task ETA regression models.
3. **`services/task-engine` (Go):** Daily job schedules, machine assignments, progress tracking.
4. **`services/training-hub` (Go):** Assigns targeted micro-learning based on detected anomaly tags.
5. **`services/notification` (Go):** Dispatches cabin safety alerts to operators and escalates violations to admins.
6. **`services/simulator` (Go/Python):** Configurable worker-machine simulators emitting telemetry frames.

## Data Models & Storage Schema

- **`users`:** `id`, `name`, `role` (`ADMIN` | `OPERATOR`), `created_at`
- **`machines`:** `id`, `model`, `status`, `current_operator_id`
- **`tasks`:** `id`, `category`, `machine_id`, `operator_id`, `status`, `target_volume`, `estimated_minutes`
- **`anomalies`:** `id`, `operator_id`, `machine_id`, `type`, `severity`, `timestamp`
- **`training_modules`:** `id`, `title`, `duration_minutes`, `trigger_tag`
- **`operator_training`:** `id`, `operator_id`, `module_id`, `status`, `score`

## NATS JetStream Topics Schema

- **`telemetry.raw`:** Dynamic sensor metrics from active machinery (`seatbelt`, `speed`, `idle_seconds`, `fuel_rate`).
- **`anomaly.detected`:** Unsafe operating behavior flags triggered by rule/ML evaluation.
- **`task.eta_updated`:** Recalculated job completion predictions based on telemetry.
- **`training.assigned`:** Targeted micro-learning lessons dispatched to an operator profile.
