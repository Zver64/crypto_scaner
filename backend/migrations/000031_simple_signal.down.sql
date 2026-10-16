-- Simple signals have no other direction to fall back to, so they go.
DELETE FROM app.strategies WHERE direction = 'simple';

ALTER TABLE app.strategies
    DROP CONSTRAINT strategies_signal_evaluation,
    DROP CONSTRAINT strategies_trading_direction,
    DROP CONSTRAINT strategies_direction_check,
    ADD CONSTRAINT strategies_direction_check CHECK (direction IN ('long', 'short', 'sideways')),
    ADD CONSTRAINT strategies_sideways_signal CHECK (signal OR direction <> 'sideways'),
    ADD CONSTRAINT strategies_signal_evaluation CHECK (
        (signal AND target_ratio IS NOT NULL AND window_candles IS NOT NULL)
        OR (NOT signal AND target_ratio IS NULL AND window_candles IS NULL)
    );
