-- name: ListStrategies :many
SELECT id, name, signal, direction, expression, exit_expression, take_profit_expression, stop_loss_expression,
       min_market_cap_usd, max_market_cap_usd, target_ratio, window_candles, message, enabled, baseline_pending, revision
FROM app.strategies
ORDER BY id;

-- name: InsertStrategy :one
INSERT INTO app.strategies (name, signal, direction, expression, exit_expression, take_profit_expression, stop_loss_expression,
                            min_market_cap_usd, max_market_cap_usd, target_ratio, window_candles, message, enabled, baseline_pending)
VALUES (@name, @signal, @direction, @expression, @exit_expression, @take_profit_expression, @stop_loss_expression,
        sqlc.narg(min_market_cap_usd), sqlc.narg(max_market_cap_usd), sqlc.narg(target_ratio), sqlc.narg(window_candles),
        @message, @enabled, @enabled)
RETURNING id;

-- name: UpdateStrategy :one
-- A change of how the strategy trades starts a new revision; a market cap
-- range change only limits later buys and signals, and a target ratio or window
-- change only how backtests judge a signal. The kind and the direction never
-- change.
UPDATE app.strategies
SET name = @name, expression = @expression, exit_expression = @exit_expression,
    take_profit_expression = @take_profit_expression, stop_loss_expression = @stop_loss_expression,
    min_market_cap_usd = sqlc.narg(min_market_cap_usd), max_market_cap_usd = sqlc.narg(max_market_cap_usd),
    target_ratio = sqlc.narg(target_ratio), window_candles = sqlc.narg(window_candles),
    message = @message, updated_at = now(),
    baseline_pending = baseline_pending OR @baseline::BOOLEAN,
    revision = revision + (expression IS DISTINCT FROM @expression
        OR exit_expression IS DISTINCT FROM @exit_expression
        OR take_profit_expression IS DISTINCT FROM @take_profit_expression
        OR stop_loss_expression IS DISTINCT FROM @stop_loss_expression)::INTEGER
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

-- name: DisableStrategyAtRevision :one
UPDATE app.strategies
SET enabled = false, baseline_pending = false, revision = revision + 1, updated_at = now()
WHERE id = $1 AND revision = $2 AND enabled
RETURNING revision;

-- name: DeleteStrategy :execrows
DELETE FROM app.strategies WHERE id = $1;

-- name: ListStrategyStates :many
SELECT strategy_id, instrument_id, open_time, entry, buys, filled, quantity, opened_at, take_profit, stop_loss
FROM app.strategy_states
ORDER BY strategy_id, instrument_id;

-- name: DeleteAllStrategyStates :exec
DELETE FROM app.strategy_states WHERE strategy_id = $1;

-- name: CompleteStrategyBaseline :execrows
-- Locks the strategy while its states are replaced; nothing changes unless
-- the evaluated revision still awaits its baseline.
UPDATE app.strategies
SET baseline_pending = FALSE
WHERE id = @id AND enabled AND baseline_pending AND revision = @revision;

-- name: InsertStrategyStates :exec
-- Inserts the states of a baseline: no trades, only the entry values.
INSERT INTO app.strategy_states (strategy_id, instrument_id, open_time, entry)
SELECT @strategy_id::BIGINT, v.instrument_id, v.open_time, v.entry
FROM (
    SELECT unnest(@instrument_ids::BIGINT[]) AS instrument_id, unnest(@open_times::TIMESTAMPTZ[]) AS open_time,
           unnest(@entries::BOOLEAN[]) AS entry
) AS v;

-- name: SaveStrategyStates :many
-- Stores states of later candles only while the evaluated revision is
-- current and announced, and returns the instruments stored. The row lock
-- orders it with a concurrent change.
INSERT INTO app.strategy_states (strategy_id, instrument_id, open_time, entry, buys, filled, quantity, opened_at, take_profit, stop_loss)
SELECT s.id, v.instrument_id, v.open_time, v.entry, v.buys, v.filled, v.quantity, v.opened_at, v.take_profit, v.stop_loss
FROM app.strategies s
CROSS JOIN (
    SELECT unnest(@instrument_ids::BIGINT[]) AS instrument_id, unnest(@open_times::TIMESTAMPTZ[]) AS open_time,
           unnest(@entries::BOOLEAN[]) AS entry, unnest(@buys::INTEGER[]) AS buys,
           unnest(@filled::INTEGER[]) AS filled, unnest(@quantities::DOUBLE PRECISION[]) AS quantity,
           unnest(@opened_at::TIMESTAMPTZ[]) AS opened_at, unnest(@take_profits::DOUBLE PRECISION[]) AS take_profit,
           unnest(@stop_losses::DOUBLE PRECISION[]) AS stop_loss
) AS v
WHERE s.id = @strategy_id AND s.enabled AND NOT s.baseline_pending AND s.revision = @revision
FOR SHARE OF s
ON CONFLICT (strategy_id, instrument_id) DO UPDATE
SET open_time = EXCLUDED.open_time, entry = EXCLUDED.entry, buys = EXCLUDED.buys,
    filled = EXCLUDED.filled, quantity = EXCLUDED.quantity, opened_at = EXCLUDED.opened_at,
    take_profit = EXCLUDED.take_profit, stop_loss = EXCLUDED.stop_loss
WHERE app.strategy_states.open_time < EXCLUDED.open_time
RETURNING instrument_id;

-- name: DeleteStrategyStates :exec
DELETE FROM app.strategy_states
WHERE strategy_id = @strategy_id::BIGINT AND instrument_id = ANY(@instrument_ids::BIGINT[]);

-- name: ListStrategyInstruments :many
-- Strategies evaluate the active favorites of the administrator, with their
-- market caps when known; CoinGecko reports 0 for coins without supply data.
SELECT i.id, i.symbol, COALESCE(market_cap.market_cap_usd > 0, FALSE)::BOOLEAN AS market_cap_known,
       COALESCE(market_cap.market_cap_usd, 0)::DOUBLE PRECISION AS market_cap_usd
FROM app.favorites f
JOIN app.users u ON u.id = f.user_id AND u.telegram_id = @administrator_telegram_id::BIGINT
JOIN binance_spot.instruments i ON i.id = f.instrument_id AND i.is_active
LEFT JOIN app.coingecko_asset_mappings mapping ON mapping.base_asset = i.base_asset AND mapping.status = 'resolved'
LEFT JOIN app.coingecko_market_caps market_cap ON market_cap.coin_id = mapping.coin_id
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
