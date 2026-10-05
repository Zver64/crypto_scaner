-- name: ListScannerIndicators :many
SELECT id, interval, indicator_type, parameters, show_in_table, show_in_chart, scale_min, scale_max, scale_levels
FROM app.scanner_indicators
ORDER BY position, id;

-- name: InsertScannerIndicator :one
INSERT INTO app.scanner_indicators (interval, indicator_type, parameters, show_in_table, show_in_chart, scale_min, scale_max, scale_levels, position)
SELECT $1, $2, $3, $4, $5, $6, $7, $8, COALESCE(MAX(position) + 1, 0)::INTEGER
FROM app.scanner_indicators
RETURNING id;

-- name: UpdateScannerIndicator :execrows
UPDATE app.scanner_indicators
SET show_in_table = $2, show_in_chart = $3, scale_min = $4, scale_max = $5, scale_levels = $6
WHERE id = $1;

-- name: DeleteScannerIndicator :execrows
DELETE FROM app.scanner_indicators WHERE id = $1;

-- name: ReorderScannerIndicators :execrows
UPDATE app.scanner_indicators AS indicators
SET position = (ordered.position - 1)::INTEGER
FROM unnest(@ids::BIGINT[]) WITH ORDINALITY AS ordered(id, position)
WHERE indicators.id = ordered.id;

-- name: DeleteUnusedScannerIndicators :many
DELETE FROM app.scanner_indicators AS indicators
WHERE NOT EXISTS (
        SELECT 1 FROM app.strategy_indicators AS used
        WHERE used.indicator_id = indicators.id
    )
RETURNING indicators.id;
