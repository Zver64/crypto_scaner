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
		return nil, err
	}
	items := make([]scannerindicator.Indicator, 0, len(rows))
	for _, row := range rows {
		var parameters indicator.Parameters
		if err := json.Unmarshal(row.Parameters, &parameters); err != nil {
			return nil, fmt.Errorf("decode scanner indicator %d parameters: %w", row.ID, err)
		}
		items = append(items, scannerindicator.Indicator{
			ID:          row.ID,
			Interval:    market.CandleInterval(row.Interval),
			Selection:   indicator.Selection{Type: indicator.Type(row.IndicatorType), Parameters: parameters},
			ShowInTable: row.ShowInTable,
			ShowInChart: row.ShowInChart,
			Scale:       scannerindicator.Scale{Min: float8Pointer(row.ScaleMin), Max: float8Pointer(row.ScaleMax), Levels: row.ScaleLevels},
		})
	}
	return items, nil
}

func (store *Store) CreateScannerIndicators(ctx context.Context, items []scannerindicator.Indicator) ([]int64, error) {
	tx, err := store.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	queries := store.queries.WithTx(tx)
	ids := make([]int64, len(items))
	for i, item := range items {
		parameters, err := json.Marshal(item.Selection.Parameters)
		if err != nil {
			return nil, fmt.Errorf("encode scanner indicator parameters: %w", err)
		}
		ids[i], err = queries.InsertScannerIndicator(ctx, generated.InsertScannerIndicatorParams{
			Interval:      string(item.Interval),
			IndicatorType: string(item.Selection.Type),
			Parameters:    parameters,
			ShowInTable:   item.ShowInTable,
			ShowInChart:   item.ShowInChart,
			ScaleMin:      float8(item.Scale.Min),
			ScaleMax:      float8(item.Scale.Max),
			ScaleLevels:   append([]float64{}, item.Scale.Levels...),
		})
		if duplicateViolation(err) {
			return nil, scannerindicator.ErrConflict
		}
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return ids, nil
}

func (store *Store) UpdateScannerIndicator(ctx context.Context, item scannerindicator.Indicator) error {
	updated, err := store.queries.UpdateScannerIndicator(ctx, generated.UpdateScannerIndicatorParams{
		ID:          item.ID,
		ShowInTable: item.ShowInTable,
		ShowInChart: item.ShowInChart,
		ScaleMin:    float8(item.Scale.Min),
		ScaleMax:    float8(item.Scale.Max),
		ScaleLevels: append([]float64{}, item.Scale.Levels...),
	})
	if err != nil {
		return err
	}
	if updated == 0 {
		return scannerindicator.ErrNotFound
	}
	return nil
}

// ReorderScannerIndicators stores ids, which must name every indicator, as
// the display order.
func (store *Store) ReorderScannerIndicators(ctx context.Context, ids []int64) error {
	reordered, err := store.queries.ReorderScannerIndicators(ctx, ids)
	if err != nil {
		return err
	}
	if reordered != int64(len(ids)) {
		return scannerindicator.ErrNotFound
	}
	return nil
}

// DeleteUnusedScannerIndicators removes every indicator no strategy references
// and returns the removed ids.
func (store *Store) DeleteUnusedScannerIndicators(ctx context.Context) ([]int64, error) {
	deleted, err := store.queries.DeleteUnusedScannerIndicators(ctx)
	if foreignKeyViolation(err) {
		return nil, scannerindicator.ErrInUse
	}
	return deleted, err
}

func (store *Store) DeleteScannerIndicator(ctx context.Context, id int64) error {
	deleted, err := store.queries.DeleteScannerIndicator(ctx, id)
	if foreignKeyViolation(err) {
		return scannerindicator.ErrInUse
	}
	if err != nil {
		return err
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
