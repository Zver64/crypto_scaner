-- name: ListFavorites :many
SELECT sqlc.embed(i), f.created_at, count(a.id)::int AS alert_count
FROM app.favorites f
JOIN binance_spot.instruments i ON i.id = f.instrument_id
LEFT JOIN app.price_alerts a ON a.user_id = f.user_id AND a.instrument_id = f.instrument_id
WHERE f.user_id = $1
GROUP BY i.id, f.created_at
ORDER BY f.created_at DESC, i.symbol;

-- name: GetFavorite :one
SELECT sqlc.embed(i), f.created_at, count(a.id)::int AS alert_count
FROM app.favorites f
JOIN binance_spot.instruments i ON i.id = f.instrument_id
LEFT JOIN app.price_alerts a ON a.user_id = f.user_id AND a.instrument_id = f.instrument_id
WHERE f.user_id = $1 AND f.instrument_id = $2
GROUP BY i.id, f.created_at;

-- name: AddFavorite :execrows
-- Affects no row when the favorite already exists.
INSERT INTO app.favorites (user_id, instrument_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: LockFavoriteInstrumentID :one
SELECT i.id
FROM app.favorites f
JOIN binance_spot.instruments i ON i.id = f.instrument_id
WHERE f.user_id = $1 AND i.symbol = $2
FOR UPDATE OF f;

-- name: DeleteFavorite :execrows
DELETE FROM app.favorites WHERE user_id = $1 AND instrument_id = $2;

-- name: CountFavoriteAlerts :one
SELECT count(*) FROM app.price_alerts WHERE user_id = $1 AND instrument_id = $2;

-- name: ListMonitoredSymbols :many
SELECT DISTINCT i.symbol
FROM app.favorites f
JOIN app.users u ON u.id = f.user_id
JOIN binance_spot.instruments i ON i.id = f.instrument_id AND i.is_active
ORDER BY i.symbol;
