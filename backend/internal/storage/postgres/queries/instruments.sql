-- name: DeactivateAllInstruments :exec
UPDATE binance_spot.instruments SET is_active = FALSE;

-- name: UpsertInstrument :one
INSERT INTO binance_spot.instruments (
    symbol, base_asset, quote_asset, exchange_status, is_active
) VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (symbol) DO UPDATE SET
    base_asset = EXCLUDED.base_asset,
    quote_asset = EXCLUDED.quote_asset,
    exchange_status = EXCLUDED.exchange_status,
    is_active = EXCLUDED.is_active
RETURNING id, symbol, base_asset, quote_asset, exchange_status, is_active;

-- name: ListActiveInstruments :many
SELECT id, symbol, base_asset, quote_asset, exchange_status, is_active
FROM binance_spot.instruments
WHERE is_active = TRUE;

-- name: ListActiveInstrumentsLimited :many
SELECT id, symbol, base_asset, quote_asset, exchange_status, is_active
FROM binance_spot.instruments
WHERE is_active = TRUE
LIMIT NULLIF(sqlc.arg(result_limit)::int, 0);

-- name: ListActiveInstrumentsSortedByMarketCap :many
SELECT instrument.id, instrument.symbol, instrument.base_asset, instrument.quote_asset,
       instrument.exchange_status, instrument.is_active
FROM binance_spot.instruments AS instrument
JOIN app.coingecko_asset_mappings AS mapping
  ON mapping.base_asset = instrument.base_asset
 AND mapping.status = 'resolved'
JOIN app.coingecko_market_caps AS market_cap
  ON market_cap.coin_id = mapping.coin_id
WHERE instrument.is_active = TRUE
ORDER BY
  CASE WHEN sqlc.arg(sort_direction)::text = 'asc' THEN market_cap.market_cap_usd END ASC,
  CASE WHEN sqlc.arg(sort_direction)::text = 'desc' THEN market_cap.market_cap_usd END DESC,
  instrument.symbol ASC
LIMIT NULLIF(sqlc.arg(result_limit)::int, 0);
