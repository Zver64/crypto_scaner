-- name: UpsertCandle :exec
INSERT INTO binance_spot.candles (
    instrument_id, interval, open_time, close_time, open, high, low, close,
    volume, quote_asset_volume, trade_count
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (instrument_id, interval, open_time) DO UPDATE SET
    close_time = EXCLUDED.close_time,
    open = EXCLUDED.open,
    high = EXCLUDED.high,
    low = EXCLUDED.low,
    close = EXCLUDED.close,
    volume = EXCLUDED.volume,
    quote_asset_volume = EXCLUDED.quote_asset_volume,
    trade_count = EXCLUDED.trade_count;

-- name: ListLatestCandles :many
SELECT instrument_id, interval, open_time, close_time, open, high, low, close,
       volume, quote_asset_volume, trade_count
FROM binance_spot.candles
WHERE instrument_id = $1
ORDER BY open_time DESC
LIMIT $2;
