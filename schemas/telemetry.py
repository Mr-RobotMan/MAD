from datetime import datetime
from uuid import UUID

from pydantic import BaseModel


class Task(BaseModel):
    id: UUID
    category: str
    machineId: UUID
    operatorId: UUID
    status: str
    targetVolume: float
    estimatedMinutes: int


class Anomaly(BaseModel):
    id: UUID
    operatorId: UUID
    machineId: UUID
    type: str
    severity: str
    timestamp: datetime
