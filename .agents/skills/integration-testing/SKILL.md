---
name: integration-testing
description: Standard protocol for verifying microservice connectivity, database transactions, and NATS message flow.
---

# Integration & Verification Protocol

Before declaring any module or microservice complete:

1. **Compilation Check:**
   - Execute `go build ./...` for Go services.
   - Execute `python -m py_compile main.py` for Python services.

2. **Test Suite Execution:**
   - Run `go test -v -cover ./...` and ensure 0 failures.
   - Run `pytest -v` and ensure 0 failures.

3. **Container Build Check:**
   - Execute `docker build -t cat-<service-name>:test .` to ensure the multi-stage Docker build passes cleanly without missing binaries or dependencies.
