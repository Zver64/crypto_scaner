# 08 — Run incremental synchronization on the daily boundary

**What to build:** Keep the stored market dataset current across restarts and UTC day boundaries without overlapping jobs or discarding previously valid data.

**Blocked by:** 07 — Backfill closed daily candles

**Status:** ready-for-agent

- [ ] Startup launches a catch-up synchronization asynchronously after HTTP becomes available.
- [ ] An instrument with existing history requests and stores only missing closed candles.
- [ ] The scheduler targets 00:00:30 UTC rather than an interval measured from process start.
- [ ] Only one synchronization can run in the process at a time.
- [ ] Binance calls use bounded concurrency and a shared rate limiter.
- [ ] Only retryable transport, rate-limit, and server failures are retried with bounded backoff and cancellation.
- [ ] New instruments are backfilled and removed or non-trading instruments become inactive while historical candles remain.
- [ ] A partial failure marks the run failed but preserves all previously valid data and successful writes.
- [ ] `GET /health/ready` returns HTTP 200 after at least one successful sync and remains ready when a later sync fails but usable data remains.
- [ ] Graceful shutdown cancels scheduled and active Binance work.
