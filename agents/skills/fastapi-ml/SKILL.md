---
name: fastapi-ml
description: Guidelines for building production-ready Python FastAPI microservices with scikit-learn models.
---

# FastAPI & ML Inference Service Pattern

When creating or modifying Python FastAPI services:

1. **Service Structure:**
   ```text
   services/analytics-ml/
   ├── main.py
   ├── models/
   │   ├── anomaly_model.pkl
   │   └── eta_model.pkl
   ├── schemas/
   │   └── telemetry.py
   ├── tests/
   │   └── test_api.py
   └── requirements.txt
   ```

2. **Lifespan Model Loading:**
   ```python
   from contextlib import asynccontextmanager
   from fastapi import FastAPI
   import joblib

   models = {}

   @asynccontextmanager
   async def lifespan(app: FastAPI):
       models["anomaly"] = joblib.load("models/anomaly_model.pkl")
       yield
       models.clear()

   app = FastAPI(lifespan=lifespan)
   ```

3. **Testing Protocol:**
   - Use `starlette.testclient.TestClient` or `httpx.AsyncClient` in `pytest`.
   - Test both normal feature vectors and out-of-bound boundary conditions.
