-- The primary key (instrument_id, interval, open_time) already serves
-- newest-first scans; B-tree indexes are read in both directions.
DROP INDEX binance_spot.candles_instrument_interval_time_desc_idx;

-- Candle values pass through float64 in the application, so NUMERIC added
-- text conversions and wider rows without adding precision.
ALTER TABLE binance_spot.candles
    ALTER COLUMN open TYPE DOUBLE PRECISION,
    ALTER COLUMN high TYPE DOUBLE PRECISION,
    ALTER COLUMN low TYPE DOUBLE PRECISION,
    ALTER COLUMN close TYPE DOUBLE PRECISION,
    ALTER COLUMN volume TYPE DOUBLE PRECISION,
    ALTER COLUMN quote_asset_volume TYPE DOUBLE PRECISION;

-- When an instrument left trading. The retention pruner deletes instruments
-- that stayed inactive for a grace period and are nobody's favorite.
ALTER TABLE binance_spot.instruments ADD COLUMN deactivated_at TIMESTAMPTZ;
UPDATE binance_spot.instruments SET deactivated_at = now() WHERE NOT is_active;
ALTER TABLE binance_spot.instruments
    ADD CONSTRAINT instruments_deactivated_at_matches_activity
    CHECK (is_active = (deactivated_at IS NULL));
