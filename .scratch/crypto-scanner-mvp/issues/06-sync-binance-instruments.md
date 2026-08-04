# 06 — Synchronize the Binance Spot instrument catalog

**What to build:** Synchronize the eligible Binance Spot/USDT instrument catalog end to end, giving the service a durable and current set of symbols to process.

**Blocked by:** 04 — Add PostgreSQL stores and database readiness

**Status:** resolved

- [x] The Binance adapter obtains Spot exchange information through the agreed official connector.
- [x] Only USDT quote instruments are persisted in the Binance Spot schema.
- [x] `TRADING` instruments become active and other known statuses become inactive.
- [x] A complete discovery snapshot updates additions, removals, and status changes transactionally.
- [x] A previously inactive instrument becomes active again when Binance returns it as trading.
- [x] A failed or incomplete discovery never deactivates the previously valid catalog.
- [x] Binance connector types and payloads remain inside the adapter.
- [x] Synchronization state records the operation outcome without acting as a distributed lock.

## Answer

Implemented the Binance Spot instrument adapter through the pinned official Go
connector and a market synchronizer for the code-owned Binance Spot/USDT MVP
profile. Discovery returns domain instruments only, rejects incomplete snapshots,
and maps only Spot/USDT symbols with `TRADING` as active. All other documented
Binance Spot statuses are inactive; an unknown status rejects the snapshot. The
synchronizer records
running, succeeded, and failed outcomes while preserving the last successful
progress and never treats an existing running state as a lock. Existing
PostgreSQL snapshot transactions cover rollback, removal, status changes, and
stable-identity reactivation.

Runtime composition, asynchronous startup catch-up, and scheduled invocation
remain deferred to issue 08 so instrument discovery does not delay HTTP
availability.

Validated with focused and full Go tests, the race detector, `go vet`, `go
build`, `go mod tidy`, and `git diff --check`. The disposable PostgreSQL test
remains opt-in and was skipped because its guarded environment variables were
not supplied.
