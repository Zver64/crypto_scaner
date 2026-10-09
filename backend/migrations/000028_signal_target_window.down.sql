-- Signals lose their target ratios and windows.
ALTER TABLE app.strategies
    DROP CONSTRAINT IF EXISTS strategies_signal_evaluation,
    DROP COLUMN IF EXISTS window_candles,
    DROP COLUMN IF EXISTS target_ratio;
