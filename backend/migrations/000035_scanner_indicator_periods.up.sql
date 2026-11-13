-- A configured indicator is a selection with its scale; its periods are
-- display settings only. Strategies no longer record the indicators they
-- read.
DROP TABLE app.strategy_indicators;

CREATE TABLE app.scanner_indicator_intervals (
    indicator_id  BIGINT NOT NULL REFERENCES app.scanner_indicators(id) ON DELETE CASCADE,
    interval      TEXT NOT NULL,
    show_in_table BOOLEAN NOT NULL,
    show_in_chart BOOLEAN NOT NULL,
    PRIMARY KEY (indicator_id, interval),
    CONSTRAINT scanner_indicator_intervals_supported_interval CHECK (interval IN ('1h', '1d', '1w', '1M'))
);

-- Rows of one selection merge into the one with the lowest position, which
-- keeps its scale; every row becomes a period of it with its flags.
INSERT INTO app.scanner_indicator_intervals (indicator_id, interval, show_in_table, show_in_chart)
SELECT first_value(id) OVER (PARTITION BY indicator_type, parameters ORDER BY position, id), interval, show_in_table, show_in_chart
FROM app.scanner_indicators;

DELETE FROM app.scanner_indicators AS merged
WHERE EXISTS (
    SELECT 1 FROM app.scanner_indicators AS kept
    WHERE kept.indicator_type = merged.indicator_type
        AND kept.parameters = merged.parameters
        AND (kept.position, kept.id) < (merged.position, merged.id)
);

ALTER TABLE app.scanner_indicators
    DROP CONSTRAINT scanner_indicators_unique_selection,
    DROP CONSTRAINT scanner_indicators_supported_interval,
    DROP COLUMN interval,
    DROP COLUMN show_in_table,
    DROP COLUMN show_in_chart;

ALTER TABLE app.scanner_indicators
    ADD CONSTRAINT scanner_indicators_unique_selection UNIQUE (indicator_type, parameters);

UPDATE app.scanner_indicators AS indicators
SET position = ordered.position
FROM (
    SELECT id, (row_number() OVER (ORDER BY position, id) - 1)::INTEGER AS position
    FROM app.scanner_indicators
) AS ordered
WHERE indicators.id = ordered.id;
