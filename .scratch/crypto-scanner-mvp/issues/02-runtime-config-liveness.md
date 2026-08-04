# 02 — Add configuration, logging, and process lifecycle

**What to build:** Run the project as a well-behaved HTTP process that validates its environment, emits safe structured logs, exposes liveness, and shuts down gracefully.

**Blocked by:** 01 — Initialize the Go project

**Status:** ready-for-agent

- [ ] Required settings are validated before the server listens, with a precise startup error for each invalid value.
- [ ] Optional settings use the defaults defined by the specification.
- [ ] Application logs are structured JSON and obey the configured log level.
- [ ] `GET /health/live` returns HTTP 200 while the process is running.
- [ ] Every HTTP response includes a request ID and request logs include that ID.
- [ ] `SIGINT` and `SIGTERM` stop new work and shut down within the configured deadline.
- [ ] Secrets, authorization headers, raw Telegram init data, and database credentials never appear in logs.
