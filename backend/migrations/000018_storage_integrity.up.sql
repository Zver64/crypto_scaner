-- No query filters price alerts by instrument alone; the favorite foreign key
-- and the unique target constraint already index (user_id, instrument_id).
DROP INDEX app.price_alerts_instrument_idx;

-- Deleting an instrument cascades to its strategy matches.
CREATE INDEX strategy_matches_instrument_idx ON app.strategy_matches (instrument_id);

-- The constraints below are NOT VALID: they hold for every row written from
-- now on, while existing rows are not checked, so they cannot fail a deploy.

-- Exactly the resolved mappings name a CoinGecko coin.
ALTER TABLE app.coingecko_asset_mappings
    ADD CONSTRAINT coingecko_asset_mappings_coin_id_matches_status
    CHECK ((status = 'resolved') = (coin_id IS NOT NULL)) NOT VALID;

-- PostgreSQL sorts NaN above Infinity, so "< 'Infinity'" rejects both; the
-- positivity checks reject negative infinity.
ALTER TABLE binance_spot.candles
    ADD CONSTRAINT candles_low_positive CHECK (low > 0) NOT VALID,
    ADD CONSTRAINT candles_close_positive CHECK (close > 0) NOT VALID,
    ADD CONSTRAINT candles_values_finite CHECK (
        open < 'Infinity'::double precision
        AND high < 'Infinity'::double precision
        AND low < 'Infinity'::double precision
        AND close < 'Infinity'::double precision
        AND volume < 'Infinity'::double precision
        AND quote_asset_volume < 'Infinity'::double precision
    ) NOT VALID;
