ALTER TABLE app.strategy_states
    DROP COLUMN IF EXISTS take_profit,
    DROP COLUMN IF EXISTS stop_loss;

ALTER TABLE app.strategies
    DROP COLUMN IF EXISTS take_profit_expression,
    DROP COLUMN IF EXISTS stop_loss_expression,
    ADD COLUMN accumulate BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN max_buys INTEGER NOT NULL DEFAULT 0,
    ADD CONSTRAINT strategies_max_buys_range CHECK (max_buys BETWEEN 0 AND 1000),
    ADD CONSTRAINT strategies_accumulate_needs_exit CHECK (NOT accumulate OR exit_expression <> '');

UPDATE app.strategies SET baseline_pending = TRUE, revision = revision + 1 WHERE enabled;
