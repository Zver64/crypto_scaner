-- name: ListStrategies :many
SELECT id, name, expression, message, enabled, baseline_pending, revision
FROM app.strategies
ORDER BY id;

-- name: InsertStrategy :one
INSERT INTO app.strategies (name, expression, message, enabled, baseline_pending)
VALUES (@name, @expression, @message, @enabled, @enabled)
RETURNING id;

-- name: UpdateStrategy :one
UPDATE app.strategies
SET name = @name, expression = @expression, message = @message, updated_at = now(),
    baseline_pending = baseline_pending OR @baseline::BOOLEAN,
    revision = revision + (expression IS DISTINCT FROM @expression)::INTEGER
WHERE id = @id
RETURNING revision;

-- name: DeleteStrategyIndicators :exec
DELETE FROM app.strategy_indicators WHERE strategy_id = $1;

-- name: InsertStrategyIndicators :exec
INSERT INTO app.strategy_indicators (strategy_id, indicator_id)
SELECT @strategy_id::BIGINT, indicator_id
FROM unnest(@indicator_ids::BIGINT[]) AS ids(indicator_id);

-- name: SetStrategyEnabled :one
UPDATE app.strategies
SET enabled = $2, baseline_pending = $2, revision = revision + 1, updated_at = now()
WHERE id = $1
RETURNING revision;

-- name: DeleteStrategy :execrows
DELETE FROM app.strategies WHERE id = $1;

-- name: ListStrategyMatches :many
SELECT strategy_id, instrument_id
FROM app.strategy_matches
ORDER BY strategy_id, instrument_id;

-- name: DeleteAllStrategyMatches :exec
DELETE FROM app.strategy_matches WHERE strategy_id = $1;

-- name: CompleteStrategyBaseline :execrows
-- Locks the strategy while its matches are replaced; nothing changes unless
-- the evaluated revision still awaits its baseline.
UPDATE app.strategies
SET baseline_pending = FALSE
WHERE id = @id AND enabled AND baseline_pending AND revision = @revision;

-- name: InsertStrategyMatches :exec
INSERT INTO app.strategy_matches (strategy_id, instrument_id)
SELECT @strategy_id::BIGINT, instrument_id
FROM unnest(@instrument_ids::BIGINT[]) AS ids(instrument_id)
ON CONFLICT DO NOTHING;

-- name: AddStrategyMatches :many
-- Adds matches only while the evaluated revision is current and announced,
-- and returns the instruments that were not matching yet. The row lock
-- orders it with a concurrent change.
INSERT INTO app.strategy_matches (strategy_id, instrument_id)
SELECT s.id, ids.instrument_id
FROM app.strategies s
CROSS JOIN unnest(@instrument_ids::BIGINT[]) AS ids(instrument_id)
WHERE s.id = @strategy_id AND s.enabled AND NOT s.baseline_pending AND s.revision = @revision
FOR SHARE OF s
ON CONFLICT DO NOTHING
RETURNING instrument_id;

-- name: DeleteStrategyMatches :exec
DELETE FROM app.strategy_matches
WHERE strategy_id = @strategy_id::BIGINT AND instrument_id = ANY(@instrument_ids::BIGINT[]);

-- name: ListStrategyInstruments :many
-- Strategies evaluate the active favorites of the administrator.
SELECT i.id, i.symbol
FROM app.favorites f
JOIN app.users u ON u.id = f.user_id AND u.telegram_id = @administrator_telegram_id::BIGINT
JOIN binance_spot.instruments i ON i.id = f.instrument_id AND i.is_active
ORDER BY i.id;

-- name: ListStrategyRecipients :many
SELECT telegram_id
FROM app.users
WHERE strategy_alerts OR telegram_id = @administrator_telegram_id::BIGINT
ORDER BY telegram_id;

-- name: LockStrategySymbols :many
-- Locks the instruments an expression reads through of against their
-- removal from the administrator's favorites, then lists them.
SELECT id, symbol
FROM binance_spot.instruments
WHERE symbol = ANY(@symbols::TEXT[])
ORDER BY id
FOR SHARE;

-- name: ListStrategyInstrumentsAmong :many
-- Runs after LockStrategySymbols, so it sees favorites removed meanwhile.
SELECT i.id
FROM app.favorites f
JOIN app.users u ON u.id = f.user_id AND u.telegram_id = @administrator_telegram_id::BIGINT
JOIN binance_spot.instruments i ON i.id = f.instrument_id AND i.is_active
WHERE i.id = ANY(@instrument_ids::BIGINT[]);

-- name: ListStrategySymbolIDs :many
SELECT instrument_id FROM app.strategy_symbols WHERE strategy_id = $1;

-- name: DeleteStrategySymbols :exec
DELETE FROM app.strategy_symbols WHERE strategy_id = $1;

-- name: InsertStrategySymbols :exec
INSERT INTO app.strategy_symbols (strategy_id, instrument_id)
SELECT @strategy_id::BIGINT, instrument_id
FROM unnest(@instrument_ids::BIGINT[]) AS ids(instrument_id);

-- name: LockInstrument :exec
-- Orders a favorite removal with strategy writes, which lock the instruments
-- they read FOR SHARE. NO KEY UPDATE conflicts with that but not with the
-- KEY SHARE locks of foreign key inserts, such as candles and favorites.
SELECT id FROM binance_spot.instruments WHERE id = $1 FOR NO KEY UPDATE;

-- name: ListStrategiesReading :many
-- The strategies that read the active instrument through of; a delisted one
-- leaves them unknown anyway. Runs after LockInstrument.
SELECT i.symbol, s.name
FROM app.strategy_symbols ss
JOIN app.strategies s ON s.id = ss.strategy_id
JOIN binance_spot.instruments i ON i.id = ss.instrument_id AND i.is_active
WHERE ss.instrument_id = $1
ORDER BY s.name;
