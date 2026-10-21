-- The cascade from a deleted delisted instrument finds its strategy coins
-- without scanning the table.
CREATE INDEX strategy_coins_instrument_idx ON app.strategy_coins (instrument_id);
