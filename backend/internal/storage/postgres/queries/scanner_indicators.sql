-- name: ListScannerIndicators :many
-- Periods come as parallel arrays, empty for an indicator without periods.
SELECT indicators.id, indicators.indicator_type, indicators.parameters, indicators.scale_min, indicators.scale_max, indicators.scale_levels,
    COALESCE(array_agg(periods.interval ORDER BY periods.interval) FILTER (WHERE periods.indicator_id IS NOT NULL), '{}')::TEXT[] AS intervals,
    COALESCE(array_agg(periods.show_in_table ORDER BY periods.interval) FILTER (WHERE periods.indicator_id IS NOT NULL), '{}')::BOOLEAN[] AS show_in_table,
    COALESCE(array_agg(periods.show_in_chart ORDER BY periods.interval) FILTER (WHERE periods.indicator_id IS NOT NULL), '{}')::BOOLEAN[] AS show_in_chart
FROM app.scanner_indicators AS indicators
LEFT JOIN app.scanner_indicator_intervals AS periods ON periods.indicator_id = indicators.id
GROUP BY indicators.id
ORDER BY indicators.position, indicators.id;

-- name: LockScannerIndicators :exec
-- Serializes appends, which number positions after the current maximum, with
-- each other and with other writes; reads stay allowed.
LOCK TABLE app.scanner_indicators IN SHARE ROW EXCLUSIVE MODE;

-- name: InsertScannerIndicator :one
-- Runs after LockScannerIndicators.
INSERT INTO app.scanner_indicators (indicator_type, parameters, scale_min, scale_max, scale_levels, position)
SELECT $1, $2, $3, $4, $5, COALESCE(MAX(position) + 1, 0)::INTEGER
FROM app.scanner_indicators
RETURNING id;

-- name: UpdateScannerIndicator :execrows
UPDATE app.scanner_indicators
SET scale_min = $2, scale_max = $3, scale_levels = $4
WHERE id = $1;

-- name: DeleteScannerIndicatorIntervals :exec
DELETE FROM app.scanner_indicator_intervals WHERE indicator_id = $1;

-- name: InsertScannerIndicatorIntervals :exec
INSERT INTO app.scanner_indicator_intervals (indicator_id, interval, show_in_table, show_in_chart)
SELECT @indicator_id::BIGINT, periods.interval, periods.show_in_table, periods.show_in_chart
FROM (
    SELECT unnest(@intervals::TEXT[]) AS interval, unnest(@show_in_table::BOOLEAN[]) AS show_in_table,
           unnest(@show_in_chart::BOOLEAN[]) AS show_in_chart
) AS periods;

-- name: DeleteScannerIndicator :execrows
DELETE FROM app.scanner_indicators WHERE id = $1;

-- name: ReorderScannerIndicators :execrows
UPDATE app.scanner_indicators AS indicators
SET position = (ordered.position - 1)::INTEGER
FROM unnest(@ids::BIGINT[]) WITH ORDINALITY AS ordered(id, position)
WHERE indicators.id = ordered.id;
