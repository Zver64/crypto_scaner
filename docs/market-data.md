# Market data operations

## Candle policy

The backend stores closed Binance Spot/USDT candles for `1h`, `1d`, `1w`, and
`1M` intervals. Binance UTC boundaries are authoritative: weeks begin Monday at
00:00 UTC and months begin on the first day at 00:00 UTC. Forming candles are
not stored or returned by the chart API.

Synchronization keeps the latest 2,000 closed candles (`market.SyncDepth`) of
every instrument and interval complete. A newly discovered instrument receives
the newest Binance page, up to 1,000 candles, and older pages follow as
history repair until it has 2,000 candles or Binance has no older ones, which
is recorded as coverage and retried after a week. Later runs request only
intervals after the latest stored open time. Catch-up is paginated when an
outage spans more than one Binance page. Internal gaps within the latest 2,000
stored rows are grouped into bounded ranges and repaired without downloading
the whole window.

Stored candles accumulate beyond that depth up to 20,000 per instrument and
interval (`market.RetentionDepth`); the weekly retention pruner deletes older
ones. Because synchronization covers every active instrument, every hourly
history grows by about 8,760 rows a year until it reaches 20,000 (about 2.3
years), so hourly candle storage grows up to tenfold over the former 2,000-row
cap; daily, weekly, and monthly histories grow by 365, 52, and 12 rows a year.

Backtests read only stored candles, up to 20,000, and never call Binance. Only
the administrator loads deeper history, from the Commands section of the Mini
App: `POST /api/v1/admin/candle-history-loads` starts a background job that
extends the named coins and intervals backwards to the requested depth (2,001
to 20,000 candles) or to Binance's oldest candle, at most one request per
second, and `GET` on the same path shows its progress. One job runs at a time;
it is kept in memory only, so a restart stops it and forgets it. Already
committed pages remain stored if a later page fails; `history_changed` reports
these changes even when the job fails, and retrying resumes from stored
history. A coin and interval without stored candles is skipped as `not_ready`
until its initial synchronization stores them, and the job goes on with the
other pairs. The server
refuses to start a load for a request authenticated with an API token
(`session_required`), and the `scanner` CLI has no such command, so agents
cannot trigger Binance loads. Synchronization never inspects rows older than
its latest 2,000, so deeper rows are neither refetched nor repaired.

The authenticated candle endpoint uses keyset pagination:

```text
GET /api/v1/instruments/{symbol}/candles?interval=1h&limit=200
GET /api/v1/instruments/{symbol}/candles?interval=1h&limit=200&before=<RFC3339-open-time>
```

`before` is exclusive. Rows are returned chronologically; `has_more` and
`next_before` describe the next older page.

## Rollout

1. Apply migration `000004_weekly_monthly_candles` before deploying the new
   backend. It expands the existing candle interval constraint without changing
   or deleting `1h`/`1d` rows.
2. Deploy the backend. Startup catch-up initializes all four synchronization
   profiles. Existing hourly and daily data remains usable while weekly and
   monthly one-page bootstraps run.
3. Wait until readiness reports `market_sync: ok`; readiness requires a
   successful run for all four profiles.
4. Deploy/expose the frontend interval selector and lazy-loading chart after the
   four-profile backend is ready.

Watch structured `market synchronization completed` logs by `profile`. Important
fields are `lag_intervals`, `exchange_requests`, `candle_rows_requested`,
`candle_rows_written`, `gap_ranges_repaired`, `retry_count`, and `duration`.
A nonzero lag after a successful scheduled boundary or repeated gap repairs
should be investigated.

## Rollback

Do not deploy an unchanged pre-migration backend against a version-4 database:
startup verifies the exact embedded schema version, so that binary will refuse
to start. Before rollout, build and test a rollback artifact from the previous
application behavior with migration 4 still embedded as its latest migration.
That compatibility artifact may ignore `1w` and `1M` rows while continuing to
accept the version-4 schema and preserving all history.

The data-preserving rollback is therefore to deploy that prepared compatibility
artifact and leave migration 4 applied. Only migrate down when weekly/monthly
data loss is explicitly accepted; the down migration deletes those candle rows
and synchronization states before restoring the old constraint. Test either
rollback path against a disposable version-4 database before production use.
