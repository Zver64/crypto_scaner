ALTER TABLE binance_spot.instruments DROP COLUMN deactivated_at;

ALTER TABLE binance_spot.candles
    ALTER COLUMN open TYPE NUMERIC,
    ALTER COLUMN high TYPE NUMERIC,
    ALTER COLUMN low TYPE NUMERIC,
    ALTER COLUMN close TYPE NUMERIC,
    ALTER COLUMN volume TYPE NUMERIC,
    ALTER COLUMN quote_asset_volume TYPE NUMERIC;

CREATE INDEX candles_instrument_interval_time_desc_idx
    ON binance_spot.candles (instrument_id, interval, open_time DESC);
