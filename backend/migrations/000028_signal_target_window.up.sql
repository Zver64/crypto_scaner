-- A signal's backtest judges each signal by a target and a stop over a
-- window of candles: target_ratio is the target distance in stops and
-- window_candles the window, in candles of the signal's interval. Both are
-- required for signals and absent for trading strategies.
ALTER TABLE app.strategies
    ADD COLUMN target_ratio SMALLINT CHECK (target_ratio IN (2, 3, 4, 5)),
    ADD COLUMN window_candles SMALLINT CHECK (window_candles IN (3, 6, 12, 24));

UPDATE app.strategies SET target_ratio = 2, window_candles = 6 WHERE signal;

ALTER TABLE app.strategies
    ADD CONSTRAINT strategies_signal_evaluation CHECK (
        (signal AND target_ratio IS NOT NULL AND window_candles IS NOT NULL)
        OR (NOT signal AND target_ratio IS NULL AND window_candles IS NULL)
    );
