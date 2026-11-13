package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/scannerindicator"
	generated "crypto-scanner/internal/storage/postgres/sqlc"

	"github.com/jackc/pgx/v5/pgtype"
)

func (store *Store) ListScannerIndicators(ctx context.Context) ([]scannerindicator.Indicator, error) {
	rows, err := store.queries.ListScannerIndicators(ctx)
	if err != nil {
		return nil, fmt.Errorf("list scanner indicators: %w", err)
	}
	items := make([]scannerindicator.Indicator, 0, len(rows))
	for _, row := range rows {
		var parameters indicator.Parameters
		if err := json.Unmarshal(row.Parameters, &parameters); err != nil {
			return nil, fmt.Errorf("decode scanner indicator %d parameters: %w", row.ID, err)
		}
		periods := make([]scannerindicator.Period, len(row.Intervals))
		for i, interval := range row.Intervals {
			periods[i] = scannerindicator.Period{Interval: market.CandleInterval(interval), ShowInTable: row.ShowInTable[i], ShowInChart: row.ShowInChart[i]}
		}
		items = append(items, scannerindicator.Indicator{
			ID:        row.ID,
			Selection: indicator.Selection{Type: indicator.Type(row.IndicatorType), Parameters: parameters},
			Periods:   periods,
			Scale:     scannerindicator.Scale{Min: float8Pointer(row.ScaleMin), Max: float8Pointer(row.ScaleMax), Levels: row.ScaleLevels},
		})
	}
	return items, nil
}

func (store *Store) CreateScannerIndicator(ctx context.Context, item scannerindicator.Indicator) (int64, error) {
	parameters, err := json.Marshal(item.Selection.Parameters)
	if err != nil {
		return 0, fmt.Errorf("encode scanner indicator parameters: %w", err)
	}
	tx, err := store.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin scanner indicator creation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	queries := store.queries.WithTx(tx)
	if err := queries.LockScannerIndicators(ctx); err != nil {
		return 0, fmt.Errorf("lock scanner indicators: %w", err)
	}
	id, err := queries.InsertScannerIndicator(ctx, generated.InsertScannerIndicatorParams{
		IndicatorType: string(item.Selection.Type),
		Parameters:    parameters,
		ScaleMin:      float8(item.Scale.Min),
		ScaleMax:      float8(item.Scale.Max),
		ScaleLevels:   append([]float64{}, item.Scale.Levels...),
	})
	if duplicateViolation(err) {
		return 0, scannerindicator.ErrConflict
	}
	if err != nil {
		return 0, fmt.Errorf("insert scanner indicator: %w", err)
	}
	if err := queries.InsertScannerIndicatorIntervals(ctx, scannerIndicatorIntervals(id, item.Periods)); err != nil {
		return 0, fmt.Errorf("insert scanner indicator periods: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit scanner indicator creation: %w", err)
	}
	return id, nil
}

// UpdateScannerIndicator replaces the periods and the scale of the indicator.
func (store *Store) UpdateScannerIndicator(ctx context.Context, item scannerindicator.Indicator) error {
	tx, err := store.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin scanner indicator update: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	queries := store.queries.WithTx(tx)
	updated, err := queries.UpdateScannerIndicator(ctx, generated.UpdateScannerIndicatorParams{
		ID:          item.ID,
		ScaleMin:    float8(item.Scale.Min),
		ScaleMax:    float8(item.Scale.Max),
		ScaleLevels: append([]float64{}, item.Scale.Levels...),
	})
	if err != nil {
		return fmt.Errorf("update scanner indicator: %w", err)
	}
	if updated == 0 {
		return scannerindicator.ErrNotFound
	}
	if err := queries.DeleteScannerIndicatorIntervals(ctx, item.ID); err != nil {
		return fmt.Errorf("delete scanner indicator periods: %w", err)
	}
	if err := queries.InsertScannerIndicatorIntervals(ctx, scannerIndicatorIntervals(item.ID, item.Periods)); err != nil {
		return fmt.Errorf("insert scanner indicator periods: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit scanner indicator update: %w", err)
	}
	return nil
}

func scannerIndicatorIntervals(id int64, periods []scannerindicator.Period) generated.InsertScannerIndicatorIntervalsParams {
	params := generated.InsertScannerIndicatorIntervalsParams{
		IndicatorID: id,
		Intervals:   make([]string, len(periods)),
		ShowInTable: make([]bool, len(periods)),
		ShowInChart: make([]bool, len(periods)),
	}
	for i, period := range periods {
		params.Intervals[i], params.ShowInTable[i], params.ShowInChart[i] = string(period.Interval), period.ShowInTable, period.ShowInChart
	}
	return params
}

// ReorderScannerIndicators stores ids, which must name every indicator, as
// the display order.
func (store *Store) ReorderScannerIndicators(ctx context.Context, ids []int64) error {
	reordered, err := store.queries.ReorderScannerIndicators(ctx, ids)
	if err != nil {
		return fmt.Errorf("reorder scanner indicators: %w", err)
	}
	if reordered != int64(len(ids)) {
		return scannerindicator.ErrNotFound
	}
	return nil
}

func (store *Store) DeleteScannerIndicator(ctx context.Context, id int64) error {
	deleted, err := store.queries.DeleteScannerIndicator(ctx, id)
	if err != nil {
		return fmt.Errorf("delete scanner indicator: %w", err)
	}
	if deleted == 0 {
		return scannerindicator.ErrNotFound
	}
	return nil
}

func float8(value *float64) pgtype.Float8 {
	if value == nil {
		return pgtype.Float8{}
	}
	return pgtype.Float8{Float64: *value, Valid: true}
}

func float8Pointer(value pgtype.Float8) *float64 {
	if !value.Valid {
		return nil
	}
	return &value.Float64
}
