-- name: FindUserByTelegramID :one
SELECT id, telegram_id, username, display_name
FROM app.users
WHERE telegram_id = $1;

-- name: GrantUserAccess :one
INSERT INTO app.users (telegram_id, username, display_name)
VALUES ($1, $2, $3)
ON CONFLICT (telegram_id) DO NOTHING
RETURNING id, telegram_id, username, display_name;

-- name: ListUsers :many
SELECT id, telegram_id, username, display_name, strategy_alerts
FROM app.users
ORDER BY telegram_id ASC;

-- name: SetUserStrategyAlerts :execrows
UPDATE app.users
SET strategy_alerts = $2, updated_at = now()
WHERE telegram_id = $1;

-- name: DeleteUserByTelegramID :one
DELETE FROM app.users
WHERE telegram_id = $1
RETURNING id;

-- name: BootstrapAdministrator :exec
INSERT INTO app.users (telegram_id)
VALUES ($1)
ON CONFLICT (telegram_id) DO NOTHING;

-- name: LockUser :one
-- Serializes a user's favorite and price alert writes.
SELECT telegram_id FROM app.users WHERE id = $1 FOR UPDATE;
