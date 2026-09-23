"""Telemetry anomaly inference API and NATS consumer."""

from __future__ import annotations

import asyncio
import os
from contextlib import asynccontextmanager
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

import joblib
import numpy as np
from fastapi import FastAPI, HTTPException, Request
from nats.aio.client import Client as NATS
from nats.errors import Error as NatsError
from sklearn.ensemble import IsolationForest

from schemas.telemetry import AnomalyEvent, AnomalyResult, HealthResponse, TelemetryFrame

MODEL_PATH = Path(__file__).parent / "models" / "anomaly_model.pkl"
def feature_vector(frame: TelemetryFrame) -> np.ndarray:
    """Convert validated telemetry to the feature order used during training."""
    return np.asarray(
        [[float(frame.seatbelt), frame.speed, float(frame.idleSeconds), frame.fuelRate]],
        dtype=float,
    )


def train_placeholder_model(path: Path) -> IsolationForest:
    """Train a deterministic placeholder using synthetic, normal operating data."""
    rng = np.random.default_rng(42)
    samples = np.column_stack(
        (
            rng.choice([0.0, 1.0], size=2000, p=[0.02, 0.98]),
            rng.normal(8.0, 2.5, size=2000).clip(0.0, 18.0),
            rng.gamma(2.0, 45.0, size=2000).clip(0.0, 600.0),
            rng.normal(12.0, 3.0, size=2000).clip(1.0, 25.0),
        )
    )
    model = IsolationForest(n_estimators=100, contamination=0.03, random_state=42)
    model.fit(samples)
    path.parent.mkdir(parents=True, exist_ok=True)
    joblib.dump(model, path)
    return model


def classify(frame: TelemetryFrame, model: IsolationForest) -> AnomalyResult:
    """Score telemetry and enforce clear safety bounds alongside model output."""
    try:
        vector = feature_vector(frame)
        decision = float(model.decision_function(vector)[0])
        predicted_anomaly = int(model.predict(vector)[0]) == -1
    except ValueError as exc:
        raise HTTPException(status_code=422, detail=str(exc)) from exc

    bound_type: str | None = None
    if frame.speed < 0:
        bound_type = "unsafe_speed"
    elif frame.speed > 40:
        bound_type = "unsafe_speed"
    elif frame.idleSeconds > 3600:
        bound_type = "excessive_idle"
    elif not frame.seatbelt:
        bound_type = "seatbelt_violation"
    elif frame.fuelRate > 60:
        bound_type = "abnormal_fuel_rate"

    is_anomaly = bound_type is not None or predicted_anomaly
    # IsolationForest's decision function is unbounded; this maps it to a
    # stable 0..1 confidence-like score while reserving 0.9 for hard bounds.
    model_score = float(np.clip(0.5 - decision, 0.0, 1.0))
    score = 0.9 if bound_type is not None else model_score
    anomaly_type = bound_type or ("statistical_outlier" if predicted_anomaly else None)
    return AnomalyResult(isAnomaly=is_anomaly, score=score, type=anomaly_type)


def severity_for(score: float) -> str:
    """Map an anomaly score to the contract severity."""
    if score > 0.8:
        return "CRITICAL"
    if score > 0.5:
        return "WARNING"
    return "INFO"


async def publish_anomaly(nats_client: NATS | None, frame: TelemetryFrame, result: AnomalyResult) -> None:
    """Publish anomaly events when a NATS connection is available."""
    if not result.isAnomaly or nats_client is None or not nats_client.is_connected:
        return
    if result.type is None:
        return
    event = AnomalyEvent(
        operatorId=frame.operatorId,
        machineId=frame.machineId,
        type=result.type,
        severity=severity_for(result.score),
        timestamp=datetime.now(timezone.utc),
    )
    await nats_client.publish("anomaly.detected", event.model_dump_json().encode())


async def consume_telemetry(message: Any, model: IsolationForest, nats_client: NATS) -> None:
    """Score each raw telemetry message and emit anomalies."""
    try:
        frame = TelemetryFrame.model_validate_json(message.data)
    except ValueError:
        await message.ack()
        return
    result = classify(frame, model)
    await publish_anomaly(nats_client, frame, result)
    await message.ack()


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Load the model and start the optional NATS subscription."""
    model = joblib.load(MODEL_PATH) if MODEL_PATH.exists() else train_placeholder_model(MODEL_PATH)
    app.state.anomaly_model = model
    client = NATS()
    app.state.nats_client = None
    try:
        await client.connect(servers=[os.getenv("NATS_URL", "nats://localhost:4222")], connect_timeout=2)
        await client.subscribe(
            "telemetry.raw",
            cb=lambda message: consume_telemetry(message, model, client),
        )
        app.state.nats_client = client
    except (NatsError, OSError, asyncio.TimeoutError):
        # HTTP inference remains available while NATS is temporarily offline.
        if client.is_connected:
            await client.close()
    try:
        yield
    finally:
        if client.is_connected:
            await client.drain()
        app.state.anomaly_model = None


app = FastAPI(lifespan=lifespan)


@app.post("/api/v1/anomaly/check", response_model=AnomalyResult)
async def check_anomaly(frame: TelemetryFrame, request: Request) -> AnomalyResult:
    """Run the loaded Isolation Forest model against one telemetry frame."""
    model = request.app.state.anomaly_model
    result = classify(frame, model)
    await publish_anomaly(request.app.state.nats_client, frame, result)
    return result


@app.get("/api/v1/health", response_model=HealthResponse)
async def health() -> HealthResponse:
    """Report service health using the standard project response envelope."""
    return HealthResponse(success=True, data={"status": "healthy"}, error=None)
