# Market data operations

## Candle policy

The backend stores closed Binance Spot/USDT candles for `1h`, `1d`, `1w`, and
`1M` intervals. Binance UTC boundaries are authoritative: weeks begin Monday at
00:00 UTC and months begin on the first day at 00:00 UTC. Forming candles are
not stored or returned by the chart API.

A newly discovered instrument receives one Binance page, up to 1,000 candles,
for each interval. This is initial coverage, not a retention cap. Later runs
retain all stored rows and request only intervals after the latest stored open
time. Catch-up is paginated when an outage spans more than one Binance page.
Internal gaps within the latest 1,000 stored rows are grouped into bounded ranges
and repaired without downloading the whole window.

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
