-- name: CreateSession :exec
-- The deletion sees the snapshot before the insert, so it keeps the newest
-- keep_existing earlier sessions of the user next to the new one.
WITH inserted AS (
    INSERT INTO app.sessions (token_hash, user_id, created_at, idle_expires_at, absolute_expires_at)
    VALUES (@token_hash, @user_id, @created_at, @idle_expires_at, @absolute_expires_at)
)
DELETE FROM app.sessions AS s
WHERE s.user_id = @user_id
  AND s.token_hash IN (
      SELECT older.token_hash
      FROM app.sessions AS older
      WHERE older.user_id = @user_id
      ORDER BY older.created_at DESC
      OFFSET @keep_existing::int
  );

-- name: FindSession :one
SELECT sqlc.embed(u), s.idle_expires_at, s.absolute_expires_at
FROM app.sessions AS s
JOIN app.users AS u ON u.id = s.user_id
WHERE s.token_hash = $1;

-- name: ExtendSession :exec
-- The predicate keeps concurrent requests to one write per minimum extension
-- and never moves the expiry back.
UPDATE app.sessions
SET idle_expires_at = @idle_expires_at
WHERE token_hash = @token_hash AND idle_expires_at <= @extend_before;

-- name: DeleteSession :exec
DELETE FROM app.sessions
WHERE token_hash = $1;

-- name: DeleteExpiredSessions :execrows
-- The table checks idle_expires_at <= absolute_expires_at, so the idle expiry
-- alone finds every expired session.
DELETE FROM app.sessions
WHERE idle_expires_at <= $1;
