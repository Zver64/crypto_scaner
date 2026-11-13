-- The syntax the rules of a strategy are written in: 1 names indicators
-- after table column titles, such as h_rsi; 2 calls them, such as
-- rsi("1h"). The backend rewrites rules of syntax 1 once at startup. The
-- default stays 1, so a strategy the previous backend saves while the
-- migrations run is rewritten too; the current backend writes 2 itself.
ALTER TABLE app.strategies ADD COLUMN syntax SMALLINT NOT NULL DEFAULT 1;
