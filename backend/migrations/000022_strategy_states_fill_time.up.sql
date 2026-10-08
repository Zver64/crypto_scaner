-- A trade's start is the open time of the candle its first buy fills at, so
-- it is known only once a buy has filled.
ALTER TABLE app.strategy_states
    DROP CONSTRAINT strategy_states_check1,
    ADD CONSTRAINT strategy_states_opened_when_filled CHECK ((opened_at IS NULL) = (filled = 0));
