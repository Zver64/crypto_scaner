-- A signal may evaluate the administrator's favorites, whatever they are at
-- the time, besides the coins chosen for it or every active coin.
ALTER TABLE app.strategies
    ADD COLUMN coin_source TEXT NOT NULL DEFAULT 'selected'
        CONSTRAINT strategies_coin_source CHECK (coin_source IN ('selected', 'favorites', 'all'));

UPDATE app.strategies SET coin_source = 'all' WHERE all_coins;

ALTER TABLE app.strategies
    ALTER COLUMN coin_source DROP DEFAULT,
    DROP CONSTRAINT strategies_all_coins_signal,
    DROP COLUMN all_coins,
    ADD CONSTRAINT strategies_coin_source_signal CHECK (signal OR coin_source = 'selected');
