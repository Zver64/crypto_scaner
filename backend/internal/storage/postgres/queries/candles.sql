-- name: UpsertCandle :batchone
INSERT INTO binance_spot.candles (
    instrument_id, interval, open_time, close_time, open, high, low, close,
    volume, quote_asset_volume, trade_count
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (instrument_id, interval, open_time) DO UPDATE SET
    close_time = EXCLUDED.close_time,
    open = EXCLUDED.open,
    high = EXCLUDED.high,
    low = EXCLUDED.low,
    close = EXCLUDED.close,
    volume = EXCLUDED.volume,
    quote_asset_volume = EXCLUDED.quote_asset_volume,
    trade_count = EXCLUDED.trade_count
WHERE (binance_spot.candles.close_time, binance_spot.candles.open, binance_spot.candles.high,
       binance_spot.candles.low, binance_spot.candles.close, binance_spot.candles.volume,
       binance_spot.candles.quote_asset_volume, binance_spot.candles.trade_count)
  IS DISTINCT FROM (EXCLUDED.close_time, EXCLUDED.open, EXCLUDED.high, EXCLUDED.low,
                    EXCLUDED.close, EXCLUDED.volume, EXCLUDED.quote_asset_volume, EXCLUDED.trade_count)
RETURNING open_time;

-- name: ListLatestCandles :many
-- Returns up to row_limit latest candles per instrument, grouped by
-- instrument. Rows are not sorted globally; callers order each instrument's
-- candles, which avoids sorting every returned row.
SELECT candle.instrument_id, candle.interval, candle.open_time, candle.close_time,
       candle.open, candle.high, candle.low, candle.close,
       candle.volume, candle.quote_asset_volume, candle.trade_count
FROM unnest(sqlc.arg(instrument_ids)::bigint[]) AS selected(instrument_id)
CROSS JOIN LATERAL (
    SELECT instrument_id, interval, open_time, close_time, open, high, low, close,
           volume, quote_asset_volume, trade_count
    FROM binance_spot.candles
    WHERE instrument_id = selected.instrument_id
      AND interval = sqlc.arg(interval)
    ORDER BY open_time DESC
    LIMIT sqlc.arg(row_limit)
) AS candle;

-- name: SummarizeCandleHistory :many
-- Walks the latest row_limit candles per instrument once, on the primary key
-- index. Every returned row carries the count and time bounds of those
-- candles; the oldest one has no gap, and every other returned row is a
-- candle that follows a gap of more than one interval step (calendar-aware in
-- UTC), with the retry time of a gap verified empty. Instruments without
-- candles are omitted.
SELECT selected.instrument_id::bigint AS instrument_id,
       history.candle_count::int AS candle_count,
       history.oldest_open_time::timestamptz AS oldest_open_time,
       history.latest_open_time::timestamptz AS latest_open_time,
       history.gap_from::timestamptz AS gap_from,
       history.open_time::timestamptz AS gap_to,
       empty_gap.retry_after::timestamptz AS gap_retry_after
FROM unnest(sqlc.arg(instrument_ids)::bigint[]) AS selected(instrument_id)
CROSS JOIN LATERAL (
    SELECT bounded.open_time, bounded.previous_open_time, bounded.candle_count,
           bounded.oldest_open_time, bounded.latest_open_time,
           ((bounded.previous_open_time AT TIME ZONE 'UTC') + sqlc.arg(step)::interval) AT TIME ZONE 'UTC' AS gap_from
    FROM (
        SELECT recent.open_time,
               lag(recent.open_time) OVER (ORDER BY recent.open_time) AS previous_open_time,
               count(*) OVER () AS candle_count,
               min(recent.open_time) OVER () AS oldest_open_time,
               max(recent.open_time) OVER () AS latest_open_time
        FROM (
            SELECT candle.open_time
            FROM binance_spot.candles AS candle
            WHERE candle.instrument_id = selected.instrument_id
              AND candle.interval = sqlc.arg(interval)
            ORDER BY candle.open_time DESC
            LIMIT sqlc.arg(row_limit)
        ) AS recent
    ) AS bounded
) AS history
LEFT JOIN binance_spot.empty_candle_gaps AS empty_gap
    ON empty_gap.instrument_id = selected.instrument_id
   AND empty_gap.interval = sqlc.arg(interval)
   AND empty_gap.gap_from = history.gap_from
   AND empty_gap.gap_to = history.open_time
WHERE history.previous_open_time IS NULL
   OR history.open_time > history.gap_from
ORDER BY 1, history.open_time;

-- name: SaveEmptyCandleGap :exec
INSERT INTO binance_spot.empty_candle_gaps (instrument_id, interval, gap_from, gap_to, retry_after)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (instrument_id, interval, gap_from) DO UPDATE SET
    gap_to = EXCLUDED.gap_to,
    retry_after = EXCLUDED.retry_after;

-- name: ListCandlePage :many
SELECT instrument_id, interval, open_time, close_time, open, high, low, close,
       volume, quote_asset_volume, trade_count
FROM binance_spot.candles
WHERE instrument_id = sqlc.arg(instrument_id)
  AND interval = sqlc.arg(interval)
  AND close_time < now()
  AND (sqlc.narg(before_time)::timestamptz IS NULL OR open_time < sqlc.narg(before_time))
ORDER BY open_time DESC
LIMIT sqlc.arg(row_limit);

-- name: PruneCandles :execrows
-- Keeps the newest keep_count candles of the interval per instrument.
DELETE FROM binance_spot.candles AS candle
USING (
    SELECT instrument.id AS instrument_id, cutoff.open_time
    FROM binance_spot.instruments AS instrument
    CROSS JOIN LATERAL (
        SELECT recent.open_time
        FROM binance_spot.candles AS recent
        WHERE recent.instrument_id = instrument.id AND recent.interval = sqlc.arg(interval)
        ORDER BY recent.open_time DESC
        OFFSET sqlc.arg(keep_count)::int
        LIMIT 1
    ) AS cutoff
) AS bound
WHERE candle.instrument_id = bound.instrument_id
  AND candle.interval = sqlc.arg(interval)
  AND candle.open_time <= bound.open_time;

-- name: ListCandleHistoryCoverage :many
SELECT instrument_id, interval, verified_oldest_open_time, target_depth,
       policy_version, retry_after
FROM binance_spot.candle_history_coverage
WHERE instrument_id = ANY(sqlc.arg(instrument_ids)::bigint[])
  AND interval = sqlc.arg(interval);

-- name: SaveCandleHistoryCoverage :exec
INSERT INTO binance_spot.candle_history_coverage (
    instrument_id, interval, verified_oldest_open_time, target_depth,
    policy_version, retry_after
) VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (instrument_id, interval) DO UPDATE SET
    verified_oldest_open_time = EXCLUDED.verified_oldest_open_time,
    target_depth = EXCLUDED.target_depth,
    policy_version = EXCLUDED.policy_version,
    retry_after = EXCLUDED.retry_after;

-- name: ListHourlyPrices :many
SELECT instrument_id, open_time, close
FROM binance_spot.candles
WHERE instrument_id = ANY(sqlc.arg(instrument_ids)::bigint[])
  AND interval = '1h'
  AND open_time >= sqlc.arg(from_time)
  AND open_time <= sqlc.arg(to_time)
  AND close_time < sqlc.arg(to_time)::timestamptz + INTERVAL '1 hour'
ORDER BY instrument_id, open_time;
