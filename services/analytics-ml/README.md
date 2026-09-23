# Analytics & Anomaly Service

FastAPI service for Isolation Forest telemetry scoring. It exposes
`POST /api/v1/anomaly/check` and `GET /api/v1/health`, and subscribes to
`telemetry.raw` on startup when NATS is available. Detected anomalies are
published to `anomaly.detected` with operator, machine, type, severity, and UTC
timestamp fields.

## Placeholder model

When `models/anomaly_model.pkl` is absent, startup trains and saves an
Isolation Forest using synthetic telemetry shaped as `(seatbelt, speed,
idleSeconds, fuelRate)`. This is only a development placeholder, not a model
trained on operational data. Replace it with an Isolation Forest trained and
validated on representative, labeled machinery telemetry before production
deployment. Keep the feature order and preprocessing compatible with
`main.py`.

Set `NATS_URL` to the NATS server URL. If NATS is unavailable at startup, HTTP
inference remains available and the service skips event publishing.
