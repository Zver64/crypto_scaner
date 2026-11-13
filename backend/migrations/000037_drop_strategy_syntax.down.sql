-- Stored rules are in the syntax of calls, 2; new rows default to 1 as
-- they did before.
ALTER TABLE app.strategies ADD COLUMN syntax SMALLINT NOT NULL DEFAULT 2;
ALTER TABLE app.strategies ALTER COLUMN syntax SET DEFAULT 1;
