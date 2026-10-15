-- Strategies evaluate the administrator's favorites again; the coins chosen
-- for them are lost, and the coins they read through of are not recorded
-- until they are saved.
ALTER TABLE app.strategies
    DROP CONSTRAINT IF EXISTS strategies_market_cap_signal,
    DROP CONSTRAINT IF EXISTS strategies_all_coins_signal,
    DROP COLUMN IF EXISTS all_coins;

DROP TABLE IF EXISTS app.strategy_coins;

CREATE TABLE app.strategy_symbols (
    strategy_id   BIGINT NOT NULL REFERENCES app.strategies(id) ON DELETE CASCADE,
    instrument_id BIGINT NOT NULL REFERENCES binance_spot.instruments(id) ON DELETE CASCADE,
    PRIMARY KEY (strategy_id, instrument_id)
);

CREATE INDEX strategy_symbols_instrument_idx ON app.strategy_symbols (instrument_id);
