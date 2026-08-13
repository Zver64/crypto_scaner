\set ON_ERROR_STOP on

SELECT :'admin_telegram_id'::bigint AS admin_telegram_id \gset
SELECT :admin_telegram_id > 0 AS valid_admin_telegram_id \gset
\if :valid_admin_telegram_id
WITH reenabled AS (
    UPDATE app.users
    SET is_enabled = TRUE,
        updated_at = now()
    WHERE telegram_id = :admin_telegram_id
      AND is_enabled = FALSE
    RETURNING telegram_id
)
INSERT INTO app.users (telegram_id, is_enabled)
SELECT :admin_telegram_id, TRUE
WHERE NOT EXISTS (SELECT 1 FROM reenabled)
  AND NOT EXISTS (
      SELECT 1
      FROM app.users
      WHERE telegram_id = :admin_telegram_id
  )
ON CONFLICT (telegram_id) DO UPDATE
SET is_enabled = TRUE,
    updated_at = now()
WHERE app.users.is_enabled = FALSE;
\else
\echo 'ADMIN_TELEGRAM_ID must be a positive base-10 integer'
SELECT 1 / 0;
\endif
