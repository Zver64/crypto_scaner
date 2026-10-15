-- Signals of the favorites, which have no coins chosen, are disabled.
ALTER TABLE app.strategies
    ADD COLUMN all_coins BOOLEAN NOT NULL DEFAULT FALSE,
    ADD CONSTRAINT strategies_all_coins_signal CHECK (signal OR NOT all_coins);

UPDATE app.strategies SET all_coins = TRUE WHERE coin_source = 'all';
DELETE FROM app.strategy_states WHERE strategy_id IN (SELECT id FROM app.strategies WHERE coin_source = 'favorites');
UPDATE app.strategies SET enabled = FALSE, baseline_pending = FALSE, revision = revision + 1, updated_at = now()
WHERE coin_source = 'favorites' AND enabled;

ALTER TABLE app.strategies
    DROP CONSTRAINT strategies_coin_source_signal,
    DROP COLUMN coin_source;
