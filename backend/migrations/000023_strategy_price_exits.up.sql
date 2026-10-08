-- A strategy may exit at prices fixed when its trade opens: take profit and
-- stop loss are CEL price expressions evaluated at the close of the signal
-- candle. A strategy with any exit holds one buy per trade; one without an
-- exit buys at every entry signal and never sells, so accumulate and max_buys
-- go.
ALTER TABLE app.strategies
    DROP CONSTRAINT strategies_accumulate_needs_exit,
    DROP CONSTRAINT strategies_max_buys_range,
    DROP COLUMN accumulate,
    DROP COLUMN max_buys,
    ADD COLUMN take_profit_expression TEXT NOT NULL DEFAULT '',
    ADD COLUMN stop_loss_expression TEXT NOT NULL DEFAULT '';

-- The take profit and stop loss prices of the open trade, 0 without a trade
-- or without that exit.
ALTER TABLE app.strategy_states
    ADD COLUMN take_profit DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (take_profit >= 0),
    ADD COLUMN stop_loss DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (stop_loss >= 0);

-- Open trades of several buys no longer exist, so enabled strategies start
-- afresh from their current entry values.
UPDATE app.strategies SET baseline_pending = TRUE, revision = revision + 1 WHERE enabled;
