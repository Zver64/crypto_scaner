DELETE FROM binance_spot.candles WHERE interval = '1h';
DELETE FROM binance_spot.sync_state WHERE profile_key = 'binance:spot:USDT:1h:UTC';

ALTER TABLE binance_spot.candles
    DROP CONSTRAINT candles_supported_interval,
    ADD CONSTRAINT candles_supported_interval CHECK (interval = '1d');
