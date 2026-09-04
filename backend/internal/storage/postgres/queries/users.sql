-- name: FindEnabledUserByTelegramID :one
SELECT id, telegram_id, username, display_name, is_enabled
FROM app.users
WHERE telegram_id = $1 AND is_enabled = TRUE;

-- name: GrantUserAccess :one
INSERT INTO app.users (telegram_id, username, display_name, is_enabled)
VALUES ($1, $2, $3, TRUE)
ON CONFLICT (telegram_id) DO UPDATE
SET username = EXCLUDED.username,
    display_name = EXCLUDED.display_name,
    is_enabled = TRUE,
    updated_at = now()
RETURNING id, telegram_id, username, display_name, is_enabled;

-- name: ListNonAdministratorUsers :many
SELECT id, telegram_id, username, display_name, is_enabled
FROM app.users
WHERE is_enabled = TRUE AND telegram_id <> $1
ORDER BY telegram_id ASC
LIMIT $2 OFFSET $3;

-- name: DeleteUserByID :one
DELETE FROM app.users
WHERE id = $1 AND telegram_id = $2
RETURNING id;

-- name: BootstrapAdministrator :exec
INSERT INTO app.users (telegram_id, is_enabled)
VALUES ($1, TRUE)
ON CONFLICT (telegram_id) DO UPDATE
SET is_enabled = TRUE,
    updated_at = now();
