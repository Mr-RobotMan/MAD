# Agent Instructions

Before writing any code, read:
- architecture.md — system design, service boundaries, data models
- project-guidelines.md — build order, testing thresholds, deployment rules
- code-quality.md — language-specific style rules

Follow project-guidelines.md's "Strict Modular Sequential Build" rule:
build one microservice at a time, tests before moving on, no stubs left untested.

Skills available in .agents/skills/: go-microservice, fastapi-ml, integration-testing.
Invoke the matching skill whenever you touch a Go or Python service, or before
marking any module "done" (integration-testing).
