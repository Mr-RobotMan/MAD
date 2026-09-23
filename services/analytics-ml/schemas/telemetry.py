"""Validated telemetry and anomaly response schemas."""

from datetime import datetime

from pydantic import BaseModel, Field


class TelemetryFrame(BaseModel):
    seatbelt: bool
    speed: float
    idleSeconds: int = Field(ge=0)
    fuelRate: float = Field(ge=0)
    operatorId: str = Field(min_length=1)
    machineId: str = Field(min_length=1)


class AnomalyResult(BaseModel):
    isAnomaly: bool
    score: float
    type: str | None


class AnomalyEvent(BaseModel):
    operatorId: str
    machineId: str
    type: str
    severity: str
    timestamp: datetime


class HealthResponse(BaseModel):
    success: bool
    data: dict[str, str]
    error: str | None
