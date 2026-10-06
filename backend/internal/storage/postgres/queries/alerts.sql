-- name: ListPriceAlerts :many
SELECT sqlc.embed(a), i.symbol, u.telegram_id
FROM app.price_alerts a
JOIN app.users u ON u.id = a.user_id
JOIN binance_spot.instruments i ON i.id = a.instrument_id
WHERE a.user_id = $1 AND i.symbol = $2
ORDER BY a.target, a.id;

-- name: ListEnabledPriceAlerts :many
SELECT sqlc.embed(a), i.symbol, u.telegram_id
FROM app.price_alerts a
JOIN app.users u ON u.id = a.user_id
JOIN binance_spot.instruments i ON i.id = a.instrument_id AND i.is_active
ORDER BY i.symbol, a.target, a.id;

-- name: InsertPriceAlert :one
INSERT INTO app.price_alerts (user_id, instrument_id, target)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdatePriceAlert :one
UPDATE app.price_alerts
SET target = $3, version = version + 1, updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: LockPriceAlertSymbol :one
SELECT i.symbol
FROM app.price_alerts a
JOIN binance_spot.instruments i ON i.id = a.instrument_id
WHERE a.id = $1 AND a.user_id = $2
FOR UPDATE OF a;

-- name: DeletePriceAlert :execrows
DELETE FROM app.price_alerts WHERE id = $1 AND user_id = $2;

-- name: FirePriceAlert :one
DELETE FROM app.price_alerts a
USING app.users u
WHERE a.id = $1 AND a.version = $2 AND a.user_id = u.id
RETURNING a.id, a.user_id, a.instrument_id, a.target::text AS target;
