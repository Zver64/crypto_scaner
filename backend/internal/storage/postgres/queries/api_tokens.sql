-- name: CreateAPIToken :one
INSERT INTO app.api_tokens (user_id, name, token_hash, created_at)
VALUES (@user_id, @name, @token_hash, @created_at)
RETURNING id;

-- name: ListAPITokens :many
SELECT id, name, created_at, last_used_at
FROM app.api_tokens
WHERE user_id = $1
ORDER BY created_at DESC, id DESC;

-- name: DeleteAPIToken :execrows
DELETE FROM app.api_tokens
WHERE id = @id AND user_id = @user_id;

-- name: FindAPIToken :one
SELECT sqlc.embed(u), t.id, t.last_used_at
FROM app.api_tokens AS t
JOIN app.users AS u ON u.id = t.user_id
WHERE t.token_hash = $1;

-- name: TouchAPIToken :exec
-- The predicate keeps concurrent requests to one write per minimum interval
-- and never moves the last use back.
UPDATE app.api_tokens
SET last_used_at = @used_at
WHERE id = @id AND (last_used_at IS NULL OR last_used_at <= @touch_before);
