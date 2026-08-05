# Crypto Scanner MVP — Implementation Map

The implementation is split into ordered, locally tracked tickets. A ticket is
available when every entry in its **Blocked by** column is resolved.

| Issue | Status | Blocked by |
|---|---|---|
| [01 — Initialize the Go project](issues/01-initialize-go-project.md) | resolved | — |
| [02 — Runtime configuration and liveness](issues/02-runtime-config-liveness.md) | resolved | 01 (resolved) |
| [03 — PostgreSQL migrations](issues/03-postgres-migrations.md) | resolved | 02 (resolved) |
| [04 — PostgreSQL stores and readiness](issues/04-postgres-stores-readiness.md) | resolved | 03 (resolved) |
| [05 — Bootstrap administrator](issues/05-bootstrap-admin.md) | resolved | 04 (resolved) |
| [06 — Synchronize Binance instruments](issues/06-sync-binance-instruments.md) | resolved | 04 (resolved) |
| [07 — Backfill closed candles](issues/07-backfill-closed-candles.md) | resolved | 06 (resolved) |
| [08 — Local development](issues/08-local-development.md) | ready-for-agent | 07 (resolved) |
| [09 — Incremental synchronization](issues/09-incremental-sync-scheduler.md) | ready-for-agent | 08 |
| [10 — Percentile analyzer](issues/10-percentile-analyzer.md) | ready-for-agent | 01 (resolved) |
| [11 — Telegram Mini App authentication](issues/11-telegram-mini-app-auth.md) | ready-for-agent | 05 (resolved) |
| [12 — Analysis HTTP API](issues/12-analysis-http-api.md) | ready-for-agent | 09, 10, 11 |
| [13 — Telegram webhook launch](issues/13-telegram-webhook-launch.md) | ready-for-agent | 08 |

Issues 08, 10, and 11 are currently unblocked.
