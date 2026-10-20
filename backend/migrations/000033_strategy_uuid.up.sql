-- Strategies and signals are identified by UUIDv7 instead of a counter, so
-- their ids tell nothing about how many exist. Existing ids are generated at
-- their creation time, keeping the creation order.
ALTER TABLE app.strategies ADD COLUMN uuid UUID;
UPDATE app.strategies SET uuid = uuidv7(created_at - clock_timestamp());

ALTER TABLE app.strategy_indicators ADD COLUMN strategy_uuid UUID;
UPDATE app.strategy_indicators c SET strategy_uuid = s.uuid FROM app.strategies s WHERE s.id = c.strategy_id;
ALTER TABLE app.strategy_states ADD COLUMN strategy_uuid UUID;
UPDATE app.strategy_states c SET strategy_uuid = s.uuid FROM app.strategies s WHERE s.id = c.strategy_id;
ALTER TABLE app.strategy_coins ADD COLUMN strategy_uuid UUID;
UPDATE app.strategy_coins c SET strategy_uuid = s.uuid FROM app.strategies s WHERE s.id = c.strategy_id;

-- Dropping the old columns drops their primary and foreign keys.
ALTER TABLE app.strategy_indicators DROP COLUMN strategy_id;
ALTER TABLE app.strategy_states DROP COLUMN strategy_id;
ALTER TABLE app.strategy_coins DROP COLUMN strategy_id;
ALTER TABLE app.strategies DROP COLUMN id;

ALTER TABLE app.strategies RENAME COLUMN uuid TO id;
ALTER TABLE app.strategies
    ALTER COLUMN id SET NOT NULL,
    ALTER COLUMN id SET DEFAULT uuidv7(),
    ADD PRIMARY KEY (id);

ALTER TABLE app.strategy_indicators RENAME COLUMN strategy_uuid TO strategy_id;
ALTER TABLE app.strategy_indicators
    ALTER COLUMN strategy_id SET NOT NULL,
    ADD FOREIGN KEY (strategy_id) REFERENCES app.strategies(id) ON DELETE CASCADE,
    ADD PRIMARY KEY (strategy_id, indicator_id);

ALTER TABLE app.strategy_states RENAME COLUMN strategy_uuid TO strategy_id;
ALTER TABLE app.strategy_states
    ALTER COLUMN strategy_id SET NOT NULL,
    ADD FOREIGN KEY (strategy_id) REFERENCES app.strategies(id) ON DELETE CASCADE,
    ADD PRIMARY KEY (strategy_id, instrument_id);

ALTER TABLE app.strategy_coins RENAME COLUMN strategy_uuid TO strategy_id;
ALTER TABLE app.strategy_coins
    ALTER COLUMN strategy_id SET NOT NULL,
    ADD FOREIGN KEY (strategy_id) REFERENCES app.strategies(id) ON DELETE CASCADE,
    ADD PRIMARY KEY (strategy_id, instrument_id);
