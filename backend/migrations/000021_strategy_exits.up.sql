-- A strategy may sell: its exit expression ends a trade, and accumulate lets
-- entry signals add buys to an open trade. Without an exit every entry signal
-- buys and nothing is sold; max_buys caps the buys of a trade, 0 meaning no
-- cap.
ALTER TABLE app.strategies
    ADD COLUMN exit_expression TEXT NOT NULL DEFAULT '',
    ADD COLUMN accumulate BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN max_buys INTEGER NOT NULL DEFAULT 0,
    ADD CONSTRAINT strategies_max_buys_range CHECK (max_buys BETWEEN 0 AND 1000),
    ADD CONSTRAINT strategies_accumulate_needs_exit CHECK (NOT accumulate OR exit_expression <> '');

DROP TABLE app.strategy_matches;

-- The trading state of a strategy on an instrument after the last candle it
-- processed, so a restart never repeats or loses a signal.
CREATE TABLE app.strategy_states (
    strategy_id   BIGINT NOT NULL REFERENCES app.strategies(id) ON DELETE CASCADE,
    instrument_id BIGINT NOT NULL REFERENCES binance_spot.instruments(id) ON DELETE CASCADE,
    -- Open time of the last processed candle of the strategy's interval.
    open_time     TIMESTAMPTZ NOT NULL,
    -- Whether the entry expression was true there, so a signal is its turn
    -- from false to true.
    entry         BOOLEAN NOT NULL,
    -- Buys of the open trade, 0 without one; filled of them have their
    -- price, one quote unit each buying quantity coins in total.
    buys          INTEGER NOT NULL DEFAULT 0 CHECK (buys >= 0),
    filled        INTEGER NOT NULL DEFAULT 0 CHECK (filled BETWEEN 0 AND buys),
    quantity      DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (quantity >= 0),
    -- Open time of the candle the first buy fills at; NULL without a trade.
    opened_at     TIMESTAMPTZ CHECK ((opened_at IS NULL) = (buys = 0)),
    PRIMARY KEY (strategy_id, instrument_id)
);

CREATE INDEX strategy_states_instrument_idx ON app.strategy_states (instrument_id);

-- Enabled strategies start afresh from their current entry values instead of
-- buying every coin whose entry is true now.
UPDATE app.strategies SET baseline_pending = TRUE, revision = revision + 1 WHERE enabled;
