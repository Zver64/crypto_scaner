-- Sessions issued in exchange for verified Telegram init data. Only the SHA-256
-- of the bearer token is stored; deleting a user revokes their sessions.
CREATE TABLE app.sessions (
    token_hash          BYTEA PRIMARY KEY CHECK (octet_length(token_hash) = 32),
    user_id             BIGINT NOT NULL REFERENCES app.users (id) ON DELETE CASCADE,
    created_at          TIMESTAMPTZ NOT NULL,
    idle_expires_at     TIMESTAMPTZ NOT NULL,
    absolute_expires_at TIMESTAMPTZ NOT NULL,
    CHECK (idle_expires_at <= absolute_expires_at)
);

CREATE INDEX sessions_user_id_created_at_idx ON app.sessions (user_id, created_at DESC);
CREATE INDEX sessions_idle_expires_at_idx ON app.sessions (idle_expires_at);
