-- The syntax the rules of a strategy are written in: 1 names indicators
-- after table column titles, such as h_rsi; 2 calls them, such as
-- rsi("1h"). The backend rewrites rules of syntax 1 once at startup.
ALTER TABLE app.strategies ADD COLUMN syntax SMALLINT NOT NULL DEFAULT 1;
ALTER TABLE app.strategies ALTER COLUMN syntax SET DEFAULT 2;
