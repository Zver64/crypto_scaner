-- A signal may be simple: it expects no particular move, so backtests only
-- show where it fired and judge nothing, and it has no target ratio or
-- window. Only a signal may be simple.
ALTER TABLE app.strategies
    DROP CONSTRAINT strategies_direction_check,
    DROP CONSTRAINT strategies_sideways_signal,
    DROP CONSTRAINT strategies_signal_evaluation,
    ADD CONSTRAINT strategies_direction_check CHECK (direction IN ('simple', 'long', 'short', 'sideways')),
    ADD CONSTRAINT strategies_trading_direction CHECK (signal OR direction IN ('long', 'short')),
    ADD CONSTRAINT strategies_signal_evaluation CHECK (
        (signal AND direction <> 'simple' AND target_ratio IS NOT NULL AND window_candles IS NOT NULL)
        OR ((NOT signal OR direction = 'simple') AND target_ratio IS NULL AND window_candles IS NULL)
    );
