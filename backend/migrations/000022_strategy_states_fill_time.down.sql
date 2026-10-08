-- Trades whose first buy has not filled yet cannot satisfy the old rule.
DELETE FROM app.strategy_states WHERE buys > 0 AND filled = 0;

ALTER TABLE app.strategy_states
    DROP CONSTRAINT strategy_states_opened_when_filled,
    ADD CONSTRAINT strategy_states_check1 CHECK ((opened_at IS NULL) = (buys = 0));
