-- Application API tokens for clients other than the Mini App, such as the
-- scanner CLI. Only the SHA-256 of the bearer token is stored; deleting a
-- token revokes it, and deleting a user revokes their tokens.
CREATE TABLE app.api_tokens (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES app.users (id) ON DELETE CASCADE,
    name         TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 64),
    token_hash   BYTEA NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    created_at   TIMESTAMPTZ NOT NULL,
    last_used_at TIMESTAMPTZ
);

CREATE INDEX api_tokens_user_id_idx ON app.api_tokens (user_id);
