"""API tests for normal and boundary telemetry inputs."""

from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
from pathlib import Path

import httpx
import pytest
import pytest_asyncio
from fastapi import FastAPI
from sklearn.ensemble import IsolationForest

from main import app
from schemas.telemetry import TelemetryFrame


@asynccontextmanager
async def controlled_lifespan(application: FastAPI) -> AsyncIterator[None]:
    """Install a deterministic test model without needing a NATS server."""
    application.state.anomaly_model = IsolationForest(random_state=42).fit(
        [[1.0, 8.0, 90.0, 12.0], [1.0, 10.0, 120.0, 14.0], [1.0, 5.0, 60.0, 10.0]]
    )
    application.state.nats_client = None
    yield
    application.state.anomaly_model = None


@pytest_asyncio.fixture
async def client() -> AsyncIterator[httpx.AsyncClient]:
    """Create an asynchronous in-process API client."""
    app.router.lifespan_context = controlled_lifespan
    transport = httpx.ASGITransport(app=app)
    async with app.router.lifespan_context(app):
        async with httpx.AsyncClient(transport=transport, base_url="http://test") as test_client:
            yield test_client


@pytest.mark.asyncio
async def test_normal_frame_is_not_flagged(client: httpx.AsyncClient) -> None:
    """A normal operating frame returns the documented result fields."""
    response = await client.post(
        "/api/v1/anomaly/check",
        json={
            "seatbelt": True,
            "speed": 8.0,
            "idleSeconds": 90,
            "fuelRate": 12.0,
            "operatorId": "operator-1",
            "machineId": "machine-1",
        },
    )
    assert response.status_code == 200
    assert set(response.json()) == {"isAnomaly", "score", "type"}
    assert response.json()["isAnomaly"] is False
    assert response.json()["type"] is None


@pytest.mark.asyncio
async def test_out_of_bounds_frame_is_flagged(client: httpx.AsyncClient) -> None:
    """A negative speed and very long idle duration are always flagged."""
    response = await client.post(
        "/api/v1/anomaly/check",
        json={
            "seatbelt": True,
            "speed": -10.0,
            "idleSeconds": 90000,
            "fuelRate": 12.0,
            "operatorId": "operator-1",
            "machineId": "machine-1",
        },
    )
    assert response.status_code == 200
    assert response.json()["isAnomaly"] is True
    assert response.json()["score"] > 0.8
    assert response.json()["type"] == "unsafe_speed"


@pytest.mark.asyncio
async def test_health_uses_standard_envelope(client: httpx.AsyncClient) -> None:
    """The health route responds with the project JSON envelope."""
    response = await client.get("/api/v1/health")
    assert response.status_code == 200
    assert response.json() == {"success": True, "data": {"status": "healthy"}, "error": None}


def test_negative_idle_is_rejected() -> None:
    """Schema validation rejects negative idle durations."""
    with pytest.raises(ValueError):
        TelemetryFrame(
            seatbelt=True,
            speed=1,
            idleSeconds=-1,
            fuelRate=1,
            operatorId="operator-1",
            machineId="machine-1",
        )


def test_placeholder_training_writes_model(tmp_path: Path) -> None:
    """Missing model artifacts can be created from synthetic telemetry."""
    from main import train_placeholder_model

    model_path = tmp_path / "anomaly_model.pkl"
    model = train_placeholder_model(model_path)
    assert model_path.exists()
    assert model.predict([[1.0, 8.0, 90.0, 12.0]]).shape == (1,)
