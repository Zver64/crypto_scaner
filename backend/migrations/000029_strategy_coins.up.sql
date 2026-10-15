-- Strategies and signals evaluate the coins chosen for them, among all active
-- coins, instead of the administrator's favorites; a signal may evaluate every
-- active coin instead. Only signals keep a market cap range. Coins read
-- through of no longer need to be favorites, so they are no longer recorded.
-- Existing strategies have no coins chosen, so they start disabled.
DROP TABLE app.strategy_symbols;

CREATE TABLE app.strategy_coins (
    strategy_id   BIGINT NOT NULL REFERENCES app.strategies(id) ON DELETE CASCADE,
    instrument_id BIGINT NOT NULL REFERENCES binance_spot.instruments(id) ON DELETE CASCADE,
    PRIMARY KEY (strategy_id, instrument_id)
);

UPDATE app.strategies SET min_market_cap_usd = NULL, max_market_cap_usd = NULL WHERE NOT signal;
DELETE FROM app.strategy_states;
UPDATE app.strategies SET enabled = FALSE, baseline_pending = FALSE, revision = revision + 1, updated_at = now() WHERE enabled;

ALTER TABLE app.strategies
    ADD COLUMN all_coins BOOLEAN NOT NULL DEFAULT FALSE,
    ADD CONSTRAINT strategies_all_coins_signal CHECK (signal OR NOT all_coins),
    ADD CONSTRAINT strategies_market_cap_signal CHECK (signal OR (min_market_cap_usd IS NULL AND max_market_cap_usd IS NULL));
