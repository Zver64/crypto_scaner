-- name: FindEnabledUserByTelegramID :one
SELECT id, telegram_id, username, display_name, is_enabled
FROM app.users
WHERE telegram_id = $1 AND is_enabled = TRUE;
