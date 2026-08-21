ALTER TABLE binance_spot.candles
    DROP CONSTRAINT candles_supported_interval,
    ADD CONSTRAINT candles_supported_interval CHECK (interval IN ('1d', '1h'));
