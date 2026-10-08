-- Without the direction a short strategy would trade long, so short
-- strategies go before it.
DELETE FROM app.strategies WHERE NOT signal AND direction = 'short';

ALTER TABLE app.strategies
    DROP CONSTRAINT IF EXISTS strategies_short_levels,
    DROP CONSTRAINT IF EXISTS strategies_sideways_signal,
    DROP CONSTRAINT IF EXISTS strategies_signal_without_trading,
    ALTER COLUMN signal DROP NOT NULL,
    ALTER COLUMN signal DROP DEFAULT,
    ALTER COLUMN signal TYPE TEXT USING CASE WHEN signal THEN direction END;

ALTER TABLE app.strategies
    DROP COLUMN direction,
    ADD CONSTRAINT strategies_signal_check CHECK (signal IN ('long', 'short', 'sideways')),
    ADD CONSTRAINT strategies_signal_without_trading CHECK (
        signal IS NULL OR (exit_expression = '' AND take_profit_expression = '' AND stop_loss_expression = ''
            AND min_market_cap_usd IS NULL AND max_market_cap_usd IS NULL)
    );
