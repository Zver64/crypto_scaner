-- Strategies get counter ids again, numbered in creation order.
ALTER TABLE app.strategies ADD COLUMN serial BIGINT;
UPDATE app.strategies s SET serial = n.serial
FROM (SELECT id, row_number() OVER (ORDER BY id) AS serial FROM app.strategies) n
WHERE n.id = s.id;

ALTER TABLE app.strategy_indicators ADD COLUMN strategy_serial BIGINT;
UPDATE app.strategy_indicators c SET strategy_serial = s.serial FROM app.strategies s WHERE s.id = c.strategy_id;
ALTER TABLE app.strategy_states ADD COLUMN strategy_serial BIGINT;
UPDATE app.strategy_states c SET strategy_serial = s.serial FROM app.strategies s WHERE s.id = c.strategy_id;
ALTER TABLE app.strategy_coins ADD COLUMN strategy_serial BIGINT;
UPDATE app.strategy_coins c SET strategy_serial = s.serial FROM app.strategies s WHERE s.id = c.strategy_id;

ALTER TABLE app.strategy_indicators DROP COLUMN strategy_id;
ALTER TABLE app.strategy_states DROP COLUMN strategy_id;
ALTER TABLE app.strategy_coins DROP COLUMN strategy_id;
ALTER TABLE app.strategies DROP COLUMN id;

ALTER TABLE app.strategies RENAME COLUMN serial TO id;
ALTER TABLE app.strategies
    ALTER COLUMN id SET NOT NULL,
    ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY,
    ADD PRIMARY KEY (id);
SELECT setval(pg_get_serial_sequence('app.strategies', 'id'), COALESCE(max(id), 0) + 1, false) FROM app.strategies;

ALTER TABLE app.strategy_indicators RENAME COLUMN strategy_serial TO strategy_id;
ALTER TABLE app.strategy_indicators
    ALTER COLUMN strategy_id SET NOT NULL,
    ADD FOREIGN KEY (strategy_id) REFERENCES app.strategies(id) ON DELETE CASCADE,
    ADD PRIMARY KEY (strategy_id, indicator_id);

ALTER TABLE app.strategy_states RENAME COLUMN strategy_serial TO strategy_id;
ALTER TABLE app.strategy_states
    ALTER COLUMN strategy_id SET NOT NULL,
    ADD FOREIGN KEY (strategy_id) REFERENCES app.strategies(id) ON DELETE CASCADE,
    ADD PRIMARY KEY (strategy_id, instrument_id);

ALTER TABLE app.strategy_coins RENAME COLUMN strategy_serial TO strategy_id;
ALTER TABLE app.strategy_coins
    ALTER COLUMN strategy_id SET NOT NULL,
    ADD FOREIGN KEY (strategy_id) REFERENCES app.strategies(id) ON DELETE CASCADE,
    ADD PRIMARY KEY (strategy_id, instrument_id);
