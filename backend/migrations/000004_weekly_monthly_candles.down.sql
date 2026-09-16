DELETE FROM binance_spot.candles WHERE interval IN ('1w', '1M');
DELETE FROM binance_spot.sync_state
WHERE profile_key IN ('binance:spot:USDT:1w:UTC', 'binance:spot:USDT:1M:UTC');

ALTER TABLE binance_spot.candles
    DROP CONSTRAINT candles_supported_interval,
    ADD CONSTRAINT candles_supported_interval CHECK (interval IN ('1d', '1h'));
