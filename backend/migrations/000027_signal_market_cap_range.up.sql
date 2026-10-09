-- A signal may announce only coins whose current market cap lies within its
-- range, like the buys of a trading strategy; it still has no exit, take
-- profit, or stop loss.
ALTER TABLE app.strategies
    DROP CONSTRAINT strategies_signal_without_trading,
    ADD CONSTRAINT strategies_signal_without_trading CHECK (
        NOT signal OR (exit_expression = '' AND take_profit_expression = '' AND stop_loss_expression = '')
    );
