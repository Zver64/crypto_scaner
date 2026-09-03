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
