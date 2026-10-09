-- Signals lose their market cap ranges.
UPDATE app.strategies SET min_market_cap_usd = NULL, max_market_cap_usd = NULL WHERE signal;

ALTER TABLE app.strategies
    DROP CONSTRAINT strategies_signal_without_trading,
    ADD CONSTRAINT strategies_signal_without_trading CHECK (
        NOT signal OR (exit_expression = '' AND take_profit_expression = '' AND stop_loss_expression = ''
            AND min_market_cap_usd IS NULL AND max_market_cap_usd IS NULL)
    );
