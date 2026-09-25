-- name: ListFavorites :many
SELECT i.id AS instrument_id, i.symbol, i.base_asset, i.quote_asset,
       i.is_active, f.created_at,
       count(a.id)::int AS alert_count
FROM app.favorites f
JOIN binance_spot.instruments i ON i.id = f.instrument_id
LEFT JOIN app.price_alerts a ON a.user_id = f.user_id AND a.instrument_id = f.instrument_id
WHERE f.user_id = $1
GROUP BY i.id, i.symbol, i.base_asset, i.quote_asset, i.exchange_status, i.is_active, f.created_at
ORDER BY f.created_at DESC, i.symbol;

-- name: ListFavoriteSymbols :many
SELECT i.symbol
FROM app.favorites f
JOIN binance_spot.instruments i ON i.id = f.instrument_id
WHERE f.user_id = $1 AND i.is_active
ORDER BY i.symbol;

-- name: GetFavorite :one
SELECT i.id AS instrument_id, i.symbol, i.base_asset, i.quote_asset,
       i.is_active, f.created_at,
       count(a.id)::int AS alert_count
FROM app.favorites f
JOIN binance_spot.instruments i ON i.id = f.instrument_id
LEFT JOIN app.price_alerts a ON a.user_id = f.user_id AND a.instrument_id = f.instrument_id
WHERE f.user_id = $1 AND i.symbol = $2
GROUP BY i.id, i.symbol, i.base_asset, i.quote_asset, i.exchange_status, i.is_active, f.created_at;

-- name: AddFavorite :exec
INSERT INTO app.favorites (user_id, instrument_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: DeleteFavorite :execrows
DELETE FROM app.favorites WHERE user_id = $1 AND instrument_id = $2;

-- name: CountFavoriteAlerts :one
SELECT count(*) FROM app.price_alerts WHERE user_id = $1 AND instrument_id = $2;

-- name: ListMonitoredSymbols :many
SELECT DISTINCT i.symbol
FROM app.favorites f
JOIN app.users u ON u.id = f.user_id AND u.is_enabled
JOIN binance_spot.instruments i ON i.id = f.instrument_id AND i.is_active
ORDER BY i.symbol;

-- name: ListMonitoredInstrumentIDs :many
SELECT DISTINCT i.id
FROM app.favorites f
JOIN app.users u ON u.id = f.user_id AND u.is_enabled
JOIN binance_spot.instruments i ON i.id = f.instrument_id AND i.is_active
ORDER BY i.id;
