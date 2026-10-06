ALTER TABLE binance_spot.candles
    DROP CONSTRAINT candles_values_finite,
    DROP CONSTRAINT candles_close_positive,
    DROP CONSTRAINT candles_low_positive;

ALTER TABLE app.coingecko_asset_mappings
    DROP CONSTRAINT coingecko_asset_mappings_coin_id_matches_status;

DROP INDEX app.strategy_matches_instrument_idx;

CREATE INDEX price_alerts_instrument_idx ON app.price_alerts (instrument_id);
