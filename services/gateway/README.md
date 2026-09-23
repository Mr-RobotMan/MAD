# Gateway

The gateway exposes `/ws`, `/health`, and the `/api/v1/*` downstream routes.
It consumes `task.eta_updated` and `anomaly.detected` from NATS, validates
payloads using `pkg/contracts`, and broadcasts task and anomaly events to
connected WebSocket clients.

REST requests route to task-engine (`/api/v1/tasks/*`), training-hub
(`/api/v1/training/*` and `/api/v1/operators/*`), or notification
(`/api/v1/alerts*` and `/api/v1/escalations*`). Task paths are translated to
the task-engine's existing `/tasks` and `/machines/assignments` routes.

## Configuration

Required: `DATABASE_URL` and `NATS_URL`. Optional: `PORT` (default `8080`),
`TASK_ENGINE_URL` (`http://task-engine:8081`), `TRAINING_HUB_URL`
(`http://training-hub:8082`), and `NOTIFICATION_URL`
(`http://notification:8083`).

For the current auth stub, send `Authorization: Bearer <JWT>` with a JWT-shaped
token whose `sub` claim is a user ID in PostgreSQL. The gateway checks that user
row. It does not validate token signatures; replace this stub with the gateway's
real authentication provider before relying on JWTs for identity.
