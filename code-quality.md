# Code Quality & Style Standard

## Go Standards

- Follow official Go style conventions (`gofmt`, `go vet`).
- Explicit error handling: Never ignore errors using `_` unless explicitly logged.
- Context propagation: Pass `ctx context.Context` as the first argument in all DB and HTTP calls.
- Standard JSON tags: Use camelCase JSON tags on Go structs (`json:"machineId"`).

## Python (FastAPI / ML) Standards

- Type hints strictly enforced on all function parameters and returns (`pydantic` schemas for API payloads).
- Load ML models in FastAPI `@asynccontextmanager` lifespan events (avoid reloading models per HTTP request).
- Catch specific exceptions (`ValueError`, `HTTPException`) instead of generic `except Exception:`.

## React & Frontend Standards

- Component boundaries: Keep views clean; extract reusable UI cards into `components/ui/`.
- Custom Hooks for State: Encapsulated WebSocket logic inside `useWebSocket.ts`.
- Tailwind CSS: No raw inline CSS styles; use semantic utility classes.

## API & Data Protocol

- All HTTP endpoints must return standard JSON response envelopes:

```json
{
  "success": true,
  "data": {},
  "error": null
}
```
