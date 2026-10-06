-- name: LockSnapshotInstruments :exec
-- Locks every row the snapshot may update in id order, the order of the
-- other instrument row locks (strategy symbols, delisted instrument
-- retention), so these transactions cannot deadlock. NO KEY UPDATE is the
-- lock the updates take themselves and does not block the KEY SHARE locks of
-- foreign key inserts such as candles and favorites.
SELECT id FROM binance_spot.instruments
WHERE is_active OR symbol = ANY(sqlc.arg(symbols)::text[])
ORDER BY id
FOR NO KEY UPDATE;

-- name: DeactivateInstrumentsExcept :exec
UPDATE binance_spot.instruments SET is_active = FALSE, deactivated_at = now()
WHERE is_active AND symbol <> ALL(sqlc.arg(symbols)::text[]);

-- name: UpsertInstruments :exec
-- Rows whose values are unchanged are not rewritten.
INSERT INTO binance_spot.instruments (
    symbol, base_asset, quote_asset, exchange_status, is_active, deactivated_at
)
SELECT snapshot.symbol, snapshot.base_asset, snapshot.quote_asset, snapshot.exchange_status, snapshot.is_active,
       CASE WHEN snapshot.is_active THEN NULL ELSE now() END
FROM (
    SELECT unnest(sqlc.arg(symbols)::text[]) AS symbol, unnest(sqlc.arg(base_assets)::text[]) AS base_asset,
           unnest(sqlc.arg(quote_assets)::text[]) AS quote_asset, unnest(sqlc.arg(statuses)::text[]) AS exchange_status,
           unnest(sqlc.arg(actives)::boolean[]) AS is_active
) AS snapshot
ON CONFLICT (symbol) DO UPDATE SET
    base_asset = EXCLUDED.base_asset,
    quote_asset = EXCLUDED.quote_asset,
    exchange_status = EXCLUDED.exchange_status,
    is_active = EXCLUDED.is_active,
    -- Keep the first deactivation time while an instrument stays inactive.
    deactivated_at = CASE
        WHEN EXCLUDED.is_active THEN NULL
        ELSE COALESCE(binance_spot.instruments.deactivated_at, now())
    END
WHERE (binance_spot.instruments.base_asset, binance_spot.instruments.quote_asset,
       binance_spot.instruments.exchange_status, binance_spot.instruments.is_active)
  IS DISTINCT FROM (EXCLUDED.base_asset, EXCLUDED.quote_asset, EXCLUDED.exchange_status, EXCLUDED.is_active);

-- name: GetActiveInstrumentBySymbol :one
SELECT sqlc.embed(instrument)
FROM binance_spot.instruments AS instrument
WHERE instrument.symbol = $1 AND instrument.is_active = TRUE;

-- name: GetActiveInstrumentIDBySymbol :one
SELECT id FROM binance_spot.instruments WHERE symbol = $1 AND is_active;

-- name: ListActiveInstruments :many
SELECT sqlc.embed(instrument)
FROM binance_spot.instruments AS instrument
WHERE instrument.is_active = TRUE;

-- name: SelectActiveInstruments :many
SELECT sqlc.embed(instrument),
       (market_cap.coin_id IS NOT NULL)::boolean AS market_cap_available,
       COALESCE(market_cap.market_cap_usd::text, ''::text)::text AS market_cap_usd
FROM binance_spot.instruments AS instrument
LEFT JOIN app.coingecko_asset_mappings AS mapping
  ON mapping.base_asset = instrument.base_asset
 AND mapping.status = 'resolved'
LEFT JOIN app.coingecko_market_caps AS market_cap
  ON market_cap.coin_id = mapping.coin_id
WHERE instrument.is_active = TRUE
  AND (sqlc.arg(symbol)::text = '' OR instrument.symbol = sqlc.arg(symbol)::text)
  AND (COALESCE(cardinality(sqlc.arg(symbols)::text[]), 0) = 0 OR instrument.symbol = ANY(sqlc.arg(symbols)::text[]))
  AND (NOT sqlc.arg(exclude_stablecoins)::boolean OR COALESCE(mapping.is_stablecoin, FALSE) = FALSE)
  AND (sqlc.narg(minimum_market_cap_usd)::numeric IS NULL OR market_cap.market_cap_usd >= sqlc.narg(minimum_market_cap_usd)::numeric)
ORDER BY
  CASE WHEN sqlc.arg(market_cap_sort)::text = 'asc' THEN market_cap.market_cap_usd END ASC NULLS LAST,
  CASE WHEN sqlc.arg(market_cap_sort)::text = 'desc' THEN market_cap.market_cap_usd END DESC NULLS LAST,
  instrument.symbol ASC
LIMIT NULLIF(sqlc.arg(result_limit)::int, 0);

-- name: DeleteDelistedInstrumentCandles :execrows
DELETE FROM binance_spot.candles
WHERE instrument_id = ANY(sqlc.arg(instrument_ids)::bigint[]);

-- name: DeleteDelistedInstruments :many
-- Their candles must already be deleted; history coverage cascades.
DELETE FROM binance_spot.instruments AS instrument
WHERE instrument.id = ANY(sqlc.arg(instrument_ids)::bigint[])
RETURNING instrument.symbol;

-- name: ListDelistedInstrumentIDs :many
-- Locks instruments inactive since before inactive_before that nobody has
-- favorited, so a concurrent snapshot cannot reactivate them mid-deletion.
-- Locks in id order, like the other instrument row locks.
SELECT instrument.id
FROM binance_spot.instruments AS instrument
WHERE NOT instrument.is_active
  AND instrument.deactivated_at < sqlc.arg(inactive_before)
  AND NOT EXISTS (SELECT 1 FROM app.favorites AS favorite WHERE favorite.instrument_id = instrument.id)
ORDER BY instrument.id
FOR UPDATE;
