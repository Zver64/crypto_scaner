# 09 — Run incremental synchronization on the daily boundary

**What to build:** Keep the stored market dataset current across restarts and UTC day boundaries without overlapping jobs or discarding previously valid data.

**Blocked by:** 08 — Configure local development

**Status:** resolved

- [x] Startup launches a catch-up synchronization asynchronously after HTTP becomes available.
- [x] An instrument with existing history requests and stores only missing closed candles.
- [x] The scheduler targets 00:00:30 UTC rather than an interval measured from process start.
- [x] Only one synchronization can run in the process at a time.
- [x] Binance calls use bounded concurrency and a shared rate limiter.
- [x] Only retryable transport, rate-limit, and server failures are retried with bounded backoff and cancellation.
- [x] New instruments are backfilled and removed or non-trading instruments become inactive while historical candles remain.
- [x] A partial failure marks the run failed but preserves all previously valid data and successful writes.
- [x] `GET /health/ready` returns HTTP 200 after at least one successful sync and remains ready when a later sync fails but usable data remains.
- [x] Graceful shutdown cancels scheduled and active Binance work.
- [x] Synchronization behaves identically when the application runs in Compose or on the host and does not depend on Docker-specific APIs.

## Answer

Added a process-local, non-overlapping synchronizer with configurable bounded
instrument concurrency. Existing instruments page forward from their latest
stored daily candle, while new instruments retain the latest-30 backfill
behavior; successful per-instrument writes survive partial failures.

Added a shared Binance rate limiter and bounded, cancellation-aware retries for
transport, HTTP 429, and server failures. The application now starts HTTP before
an asynchronous catch-up, schedules subsequent runs for 00:00:30 UTC, and
cancels and joins scheduler-owned work before shutting HTTP down.
