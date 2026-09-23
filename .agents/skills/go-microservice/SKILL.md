---
name: go-microservice
description: Standardized pattern for generating robust Go microservices with database access and unit tests.
---

# Go Microservice Construction Pattern

When building or modifying a Go microservice in this project:

1. **Folder Structure:**
   ```text
   services/<service-name>/
   ├── main.go
   ├── config/
   ├── handler/
   ├── repository/
   └── repository_test.go
   ```

2. **Handler Pattern:**
   - Always parse JSON into a strongly typed struct validated by `pkg/contracts`.
   - Respond using standard HTTP status codes (`200 OK`, `400 Bad Request`, `500 Internal Server Error`).

3. **Database Repository Pattern:**
   - Use interface abstractions for database operations to allow mock testing.

   ```go
   type TaskRepository interface {
       CreateTask(ctx context.Context, task *Task) error
       GetTaskByID(ctx context.Context, id string) (*Task, error)
   }
   ```

4. **Testing Protocol:**
   - Always generate `*_test.go` files alongside the implementation.
   - Use `httptest.NewServer` for testing endpoints without starting a real network listener.
