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
SELECT id, telegram_id, username, display_name
FROM app.users
ORDER BY telegram_id ASC;

-- name: DeleteUserByTelegramID :one
DELETE FROM app.users
WHERE telegram_id = $1
RETURNING id;

-- name: BootstrapAdministrator :exec
INSERT INTO app.users (telegram_id)
VALUES ($1)
ON CONFLICT (telegram_id) DO NOTHING;
