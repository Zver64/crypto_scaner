-- The other instruments an expression reads through of. They must be in the
-- administrator's favorites, and the administrator cannot remove an active
-- one from them while a strategy reads it.
CREATE TABLE app.strategy_symbols (
    strategy_id   BIGINT NOT NULL REFERENCES app.strategies(id) ON DELETE CASCADE,
    instrument_id BIGINT NOT NULL REFERENCES binance_spot.instruments(id) ON DELETE CASCADE,
    PRIMARY KEY (strategy_id, instrument_id)
);

CREATE INDEX strategy_symbols_instrument_idx ON app.strategy_symbols (instrument_id);
