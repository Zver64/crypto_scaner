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

-- name: GetActiveInstrumentBySymbol :one
SELECT id, symbol, base_asset, quote_asset, exchange_status, is_active
FROM binance_spot.instruments
WHERE symbol = $1 AND is_active = TRUE;

-- name: ListActiveInstruments :many
SELECT id, symbol, base_asset, quote_asset, exchange_status, is_active
FROM binance_spot.instruments
WHERE is_active = TRUE;

-- name: SelectActiveInstruments :many
SELECT instrument.id, instrument.symbol, instrument.base_asset, instrument.quote_asset,
       instrument.exchange_status, instrument.is_active,
       market_cap.coin_id IS NOT NULL AS market_cap_available,
       COALESCE(market_cap.market_cap_usd::text, ''::text) AS market_cap_usd
FROM binance_spot.instruments AS instrument
LEFT JOIN app.asset_classifications AS classification
  ON classification.base_asset = instrument.base_asset
LEFT JOIN app.coingecko_asset_mappings AS mapping
  ON mapping.base_asset = instrument.base_asset
 AND mapping.status = 'resolved'
LEFT JOIN app.coingecko_market_caps AS market_cap
  ON market_cap.coin_id = mapping.coin_id
WHERE instrument.is_active = TRUE
  AND (sqlc.arg(symbol)::text = '' OR instrument.symbol = sqlc.arg(symbol)::text)
  AND (NOT sqlc.arg(exclude_stablecoins)::boolean OR COALESCE(classification.is_stablecoin, FALSE) = FALSE)
  AND (sqlc.narg(minimum_market_cap_usd)::numeric IS NULL OR market_cap.market_cap_usd >= sqlc.narg(minimum_market_cap_usd)::numeric)
ORDER BY
  CASE WHEN sqlc.arg(market_cap_sort)::text = 'asc' THEN market_cap.market_cap_usd END ASC NULLS LAST,
  CASE WHEN sqlc.arg(market_cap_sort)::text = 'desc' THEN market_cap.market_cap_usd END DESC NULLS LAST,
  CASE WHEN sqlc.arg(market_cap_sort)::text <> '' THEN instrument.symbol END ASC
LIMIT NULLIF(sqlc.arg(result_limit)::int, 0);
