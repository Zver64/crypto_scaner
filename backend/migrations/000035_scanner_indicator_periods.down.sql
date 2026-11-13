-- Every period becomes an indicator row of its own again: the first period,
-- in interval order, stays on the indicator's row and the others become
-- copies. Indicators without periods cannot be represented and are deleted.
-- The strategy indicator links are recreated empty.
ALTER TABLE app.scanner_indicators DROP CONSTRAINT scanner_indicators_unique_selection;

ALTER TABLE app.scanner_indicators
    ADD COLUMN interval TEXT,
    ADD COLUMN show_in_table BOOLEAN,
    ADD COLUMN show_in_chart BOOLEAN;

CREATE TEMPORARY TABLE scanner_indicator_periods AS
SELECT indicator_id, interval, show_in_table, show_in_chart,
    row_number() OVER (PARTITION BY indicator_id ORDER BY array_position(ARRAY['1h', '1d', '1w', '1M'], interval)) AS period_order
FROM app.scanner_indicator_intervals;

UPDATE app.scanner_indicators AS indicators
SET interval = periods.interval, show_in_table = periods.show_in_table, show_in_chart = periods.show_in_chart
FROM scanner_indicator_periods AS periods
WHERE periods.indicator_id = indicators.id AND periods.period_order = 1;

INSERT INTO app.scanner_indicators (interval, indicator_type, parameters, show_in_table, show_in_chart, scale_min, scale_max, scale_levels, created_at, position)
SELECT periods.interval, indicators.indicator_type, indicators.parameters, periods.show_in_table, periods.show_in_chart,
    indicators.scale_min, indicators.scale_max, indicators.scale_levels, indicators.created_at, indicators.position
FROM scanner_indicator_periods AS periods
JOIN app.scanner_indicators AS indicators ON indicators.id = periods.indicator_id
WHERE periods.period_order > 1;

DROP TABLE scanner_indicator_periods;
DROP TABLE app.scanner_indicator_intervals;

DELETE FROM app.scanner_indicators WHERE interval IS NULL;

UPDATE app.scanner_indicators AS indicators
SET position = ordered.position
FROM (
    SELECT id, (row_number() OVER (ORDER BY position, array_position(ARRAY['1h', '1d', '1w', '1M'], interval), id) - 1)::INTEGER AS position
    FROM app.scanner_indicators
) AS ordered
WHERE indicators.id = ordered.id;

ALTER TABLE app.scanner_indicators
    ALTER COLUMN interval SET NOT NULL,
    ALTER COLUMN show_in_table SET NOT NULL,
    ALTER COLUMN show_in_chart SET NOT NULL,
    ADD CONSTRAINT scanner_indicators_supported_interval CHECK (interval IN ('1h', '1d', '1w', '1M')),
    ADD CONSTRAINT scanner_indicators_unique_selection UNIQUE (interval, indicator_type, parameters);

CREATE TABLE app.strategy_indicators (
    strategy_id  UUID NOT NULL REFERENCES app.strategies(id) ON DELETE CASCADE,
    indicator_id BIGINT NOT NULL REFERENCES app.scanner_indicators(id) ON DELETE RESTRICT,
    PRIMARY KEY (strategy_id, indicator_id)
);

CREATE INDEX strategy_indicators_indicator_idx ON app.strategy_indicators (indicator_id);
