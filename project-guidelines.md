# Project Guidelines & Build Protocol

## Core Engineering Rules

1. **Strict Modular Sequential Build:**
   - Build ONE microservice at a time.
   - Write unit and integration tests BEFORE considering a module finished.
   - Do NOT jump ahead or leave stubbed implementations without tests.

2. **Database & Schema Rules:**
   - All PostgreSQL migrations must live in `migrations/` as numbered raw SQL files (`000001_init.up.sql`).
   - Database interaction in Go MUST use parameterized queries (`$1, $2`) to prevent SQL injection.

3. **Containerization & Deployment:**
   - Every service must have its own lightweight multi-stage Dockerfile (`Dockerfile.multistage`).
   - All services must bind to environment variables (`PORT`, `DATABASE_URL`, `NATS_URL`).
   - The synthetic simulator MUST be deployable as K8s Pods using manifests in `deploy/k8s/`.

4. **Testing Thresholds:**
   - **Go Services:** Unit test code coverage > 80% (`go test -v -cover ./...`).
   - **Python Service:** `pytest` suites covering all FastAPI routes and ML model edge cases.
   - **Frontend:** Component integration testing using Vitest / React Testing Library.

5. **Clean Code & Output:**
   - 0 compilation warnings in Go (`go vet`).
   - 0 linting errors in Python (`ruff` or `flake8`).
   - Clean, zero-console-error React render loops.
