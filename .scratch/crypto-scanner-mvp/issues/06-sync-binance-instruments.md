# 06 — Synchronize the Binance Spot instrument catalog

**What to build:** Synchronize the eligible Binance Spot/USDT instrument catalog end to end, giving the service a durable and current set of symbols to process.

**Blocked by:** 04 — Add PostgreSQL stores and database readiness

**Status:** ready-for-agent

- [ ] The Binance adapter obtains Spot exchange information through the agreed official connector.
- [ ] Only USDT quote instruments are persisted in the Binance Spot schema.
- [ ] `TRADING` instruments become active and other known statuses become inactive.
- [ ] A complete discovery snapshot updates additions, removals, and status changes transactionally.
- [ ] A previously inactive instrument becomes active again when Binance returns it as trading.
- [ ] A failed or incomplete discovery never deactivates the previously valid catalog.
- [ ] Binance connector types and payloads remain inside the adapter.
- [ ] Synchronization state records the operation outcome without acting as a distributed lock.
