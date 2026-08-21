DELETE FROM binance_spot.candles WHERE interval = '1h';

ALTER TABLE binance_spot.candles
    DROP CONSTRAINT candles_supported_interval,
    ADD CONSTRAINT candles_supported_interval CHECK (interval = '1d');
