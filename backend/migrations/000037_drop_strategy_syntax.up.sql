-- Every stored rule is written in indicator calls, so the syntax version
-- of the one-time rewrite is no longer needed.
ALTER TABLE app.strategies DROP COLUMN syntax;
