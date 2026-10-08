-- Without the column a signal would become a strategy that buys at every
-- entry signal, so signals go before it.
DELETE FROM app.strategies WHERE signal IS NOT NULL;

ALTER TABLE app.strategies
    DROP CONSTRAINT IF EXISTS strategies_signal_without_trading,
    DROP COLUMN IF EXISTS signal;
