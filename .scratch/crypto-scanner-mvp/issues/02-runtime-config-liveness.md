# 02 — Add configuration, logging, and process lifecycle

**What to build:** Run the project as a well-behaved HTTP process that validates its environment, emits safe structured logs, exposes liveness, and shuts down gracefully.

**Blocked by:** 01 — Initialize the Go project

**Status:** resolved

- [x] Required settings are validated before the server listens, with a precise startup error for each invalid value.
- [x] Optional settings use the defaults defined by the specification.
- [x] Application logs are structured JSON and obey the configured log level.
- [x] `GET /health/live` returns HTTP 200 while the process is running.
- [x] Every HTTP response includes a request ID and request logs include that ID.
- [x] `SIGINT` and `SIGTERM` stop new work and shut down within the configured deadline.
- [x] Secrets, authorization headers, raw Telegram init data, and database credentials never appear in logs.

## Answer

Implemented the environment configuration SSOT and validation, safe level-aware
JSON logging, liveness and request-correlation middleware, context-driven HTTP
graceful shutdown, and signal-aware command composition. Added behavioral tests
at the configuration and real HTTP process seams, aligned `.env.example` and the
README, and verified with `go test ./...`, `go vet ./...`, `make check`, plus a
direct liveness request and SIGINT shutdown of the built binary.
