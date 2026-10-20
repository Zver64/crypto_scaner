-- name: ListStrategies :many
-- coins lists the coins chosen, delisted ones included, in byte order, as the
-- strategy package sorts them.
SELECT s.id, s.name, s.signal, s.direction, s.expression, s.exit_expression, s.take_profit_expression, s.stop_loss_expression,
       ARRAY(
           SELECT i.symbol
           FROM app.strategy_coins sc
           JOIN binance_spot.instruments i ON i.id = sc.instrument_id
           WHERE sc.strategy_id = s.id
           ORDER BY i.symbol COLLATE "C"
       )::TEXT[] AS coins,
       s.coin_source, s.min_market_cap_usd, s.max_market_cap_usd, s.target_ratio, s.window_candles, s.message, s.enabled, s.baseline_pending, s.revision
FROM app.strategies s
ORDER BY s.id;

-- name: InsertStrategy :one
INSERT INTO app.strategies (name, signal, direction, expression, exit_expression, take_profit_expression, stop_loss_expression,
                            coin_source, min_market_cap_usd, max_market_cap_usd, target_ratio, window_candles, message, enabled, baseline_pending)
VALUES (@name, @signal, @direction, @expression, @exit_expression, @take_profit_expression, @stop_loss_expression,
        @coin_source, sqlc.narg(min_market_cap_usd), sqlc.narg(max_market_cap_usd), sqlc.narg(target_ratio), sqlc.narg(window_candles),
        @message, @enabled, @enabled)
RETURNING id;

-- name: UpdateStrategy :one
-- A change of how the strategy trades or of the coins it evaluates starts a
-- new revision, @retrade; a market cap range change only limits later signals,
-- and a target ratio or window change only how backtests judge a signal. The
-- kind and the direction never change.
UPDATE app.strategies
SET name = @name, expression = @expression, exit_expression = @exit_expression,
    take_profit_expression = @take_profit_expression, stop_loss_expression = @stop_loss_expression,
    coin_source = @coin_source, min_market_cap_usd = sqlc.narg(min_market_cap_usd), max_market_cap_usd = sqlc.narg(max_market_cap_usd),
    target_ratio = sqlc.narg(target_ratio), window_candles = sqlc.narg(window_candles),
    message = @message, updated_at = now(),
    baseline_pending = baseline_pending OR @baseline::BOOLEAN,
    revision = revision + CASE WHEN @retrade::BOOLEAN THEN 1 ELSE 0 END
WHERE id = @id
RETURNING revision;

-- name: DeleteStrategyIndicators :exec
DELETE FROM app.strategy_indicators WHERE strategy_id = $1;

-- name: InsertStrategyIndicators :exec
INSERT INTO app.strategy_indicators (strategy_id, indicator_id)
SELECT @strategy_id::UUID, indicator_id
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
SELECT @strategy_id::UUID, v.instrument_id, v.open_time, v.entry
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
WHERE strategy_id = @strategy_id::UUID AND instrument_id = ANY(@instrument_ids::BIGINT[]);

-- name: ListStrategyInstruments :many
-- Strategies evaluate active coins, with their market caps when known;
-- CoinGecko reports 0 for coins without supply data.
SELECT i.id, i.symbol, COALESCE(market_cap.market_cap_usd > 0, FALSE)::BOOLEAN AS market_cap_known,
       COALESCE(market_cap.market_cap_usd, 0)::DOUBLE PRECISION AS market_cap_usd
FROM binance_spot.instruments i
LEFT JOIN app.coingecko_asset_mappings mapping ON mapping.base_asset = i.base_asset AND mapping.status = 'resolved'
LEFT JOIN app.coingecko_market_caps market_cap ON market_cap.coin_id = mapping.coin_id
WHERE i.is_active
ORDER BY i.id;

-- name: ListStrategyRecipients :many
SELECT telegram_id
FROM app.users
WHERE strategy_alerts OR telegram_id = @administrator_telegram_id::BIGINT
ORDER BY telegram_id;

-- name: DeleteStrategyCoins :exec
DELETE FROM app.strategy_coins WHERE strategy_id = $1;

-- name: InsertStrategyCoins :exec
INSERT INTO app.strategy_coins (strategy_id, instrument_id)
SELECT @strategy_id::UUID, i.id
FROM binance_spot.instruments i
WHERE i.symbol = ANY(@symbols::TEXT[]);

-- name: ListStrategyFavorites :many
-- The user's favorites, active or not, of which signals of the favorites
-- evaluate the active ones; in byte order, as the strategy package sorts
-- symbols.
SELECT i.symbol
FROM app.favorites f
JOIN binance_spot.instruments i ON i.id = f.instrument_id
WHERE f.user_id = @user_id
ORDER BY i.symbol COLLATE "C";
