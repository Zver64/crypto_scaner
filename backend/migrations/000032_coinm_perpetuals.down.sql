DROP TABLE binance_spot.coinm_perpetuals;
DELETE FROM binance_spot.sync_state WHERE profile_key = 'binance:coinm_perpetual:USD:1d:UTC';
