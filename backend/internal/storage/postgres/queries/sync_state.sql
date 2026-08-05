-- name: GetSyncState :one
SELECT profile_key, last_started_at, last_succeeded_at, last_closed_open_time,
       status, error_message
FROM binance_spot.sync_state
WHERE profile_key = $1;

-- name: SaveSyncState :exec
INSERT INTO binance_spot.sync_state (
    profile_key, last_started_at, last_succeeded_at, last_closed_open_time,
    status, error_message
) VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (profile_key) DO UPDATE SET
    last_started_at = EXCLUDED.last_started_at,
    last_succeeded_at = COALESCE(EXCLUDED.last_succeeded_at, sync_state.last_succeeded_at),
    last_closed_open_time = COALESCE(EXCLUDED.last_closed_open_time, sync_state.last_closed_open_time),
    status = EXCLUDED.status,
    error_message = EXCLUDED.error_message;

-- name: SuccessfulMarketSyncExists :one
SELECT EXISTS (
    SELECT 1 FROM binance_spot.sync_state WHERE last_succeeded_at IS NOT NULL
);
