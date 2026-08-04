-- name: FindEnabledUserByTelegramID :one
SELECT id, telegram_id, username, display_name, is_enabled
FROM app.users
WHERE telegram_id = $1 AND is_enabled = TRUE;

-- name: BootstrapAdministrator :exec
WITH reenabled AS (
    UPDATE app.users
    SET is_enabled = TRUE,
        updated_at = now()
    WHERE telegram_id = $1
      AND is_enabled = FALSE
    RETURNING telegram_id
)
INSERT INTO app.users (telegram_id, is_enabled)
SELECT $1, TRUE
WHERE NOT EXISTS (SELECT 1 FROM reenabled)
  AND NOT EXISTS (
      SELECT 1
      FROM app.users
      WHERE telegram_id = $1
  )
ON CONFLICT (telegram_id) DO UPDATE
SET is_enabled = TRUE,
    updated_at = now()
WHERE app.users.is_enabled = FALSE;
