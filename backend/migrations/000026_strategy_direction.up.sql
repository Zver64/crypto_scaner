-- Every strategy has a direction: a trading strategy trades long or short,
-- a signal expects a move up, down, or sideways. signal now only tells a
-- signal, which trades nothing, from a trading strategy. A short strategy
-- always has a take profit and a stop loss.
ALTER TABLE app.strategies
    ADD COLUMN direction TEXT NOT NULL DEFAULT 'long' CHECK (direction IN ('long', 'short', 'sideways'));

UPDATE app.strategies SET direction = signal WHERE signal IS NOT NULL;

ALTER TABLE app.strategies
    DROP CONSTRAINT strategies_signal_without_trading,
    DROP CONSTRAINT strategies_signal_check,
    ALTER COLUMN signal TYPE BOOLEAN USING signal IS NOT NULL,
    ALTER COLUMN signal SET DEFAULT FALSE,
    ALTER COLUMN signal SET NOT NULL,
    ALTER COLUMN direction DROP DEFAULT,
    ADD CONSTRAINT strategies_signal_without_trading CHECK (
        NOT signal OR (exit_expression = '' AND take_profit_expression = '' AND stop_loss_expression = ''
            AND min_market_cap_usd IS NULL AND max_market_cap_usd IS NULL)
    ),
    ADD CONSTRAINT strategies_sideways_signal CHECK (signal OR direction <> 'sideways'),
    ADD CONSTRAINT strategies_short_levels CHECK (
        signal OR direction <> 'short' OR (take_profit_expression <> '' AND stop_loss_expression <> '')
    );
