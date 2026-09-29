ALTER TABLE app.scanner_indicators ADD COLUMN position INTEGER;

UPDATE app.scanner_indicators AS indicators
SET position = ordered.position
FROM (
    SELECT id, (row_number() OVER (ORDER BY created_at, id) - 1)::INTEGER AS position
    FROM app.scanner_indicators
) AS ordered
WHERE indicators.id = ordered.id;

ALTER TABLE app.scanner_indicators ALTER COLUMN position SET NOT NULL;
