-- Whether charts of the interval draw the indicator; existing indicators keep
-- being drawn.
ALTER TABLE app.scanner_indicators ADD COLUMN show_in_chart BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE app.scanner_indicators ALTER COLUMN show_in_chart DROP DEFAULT;
