-- A strategy may be a signal instead of a trading strategy: its entry
-- expression announces the price move it expects, long, short, or sideways,
-- and buys nothing. A signal has no exit, take profit, stop loss, or market
-- cap range. NULL keeps the strategy trading.
ALTER TABLE app.strategies
    ADD COLUMN signal TEXT CHECK (signal IN ('long', 'short', 'sideways')),
    ADD CONSTRAINT strategies_signal_without_trading CHECK (
        signal IS NULL OR (exit_expression = '' AND take_profit_expression = '' AND stop_loss_expression = ''
            AND min_market_cap_usd IS NULL AND max_market_cap_usd IS NULL)
    );
