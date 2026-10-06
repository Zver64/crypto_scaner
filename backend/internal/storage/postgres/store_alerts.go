package postgres

import (
	"context"
	"errors"
	"fmt"

	"crypto-scanner/internal/alerts"
	generated "crypto-scanner/internal/storage/postgres/sqlc"

	"github.com/jackc/pgx/v5"
)

func (store *Store) ListAlerts(ctx context.Context, userID int64, symbol string) ([]alerts.Alert, error) {
	rows, err := store.queries.ListPriceAlerts(ctx, generated.ListPriceAlertsParams{UserID: userID, Symbol: symbol})
	if err != nil {
		return nil, fmt.Errorf("list price alerts: %w", err)
	}
	items := make([]alerts.Alert, 0, len(rows))
	for _, row := range rows {
		item, err := alertFromRow(row.AppPriceAlert, row.Symbol, row.TelegramID)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// CreateAlert adds the instrument to the user's favorites when it is not one
// yet and reports whether it did.
func (store *Store) CreateAlert(ctx context.Context, userID int64, symbol, target string) (alerts.Alert, bool, error) {
	tx, err := store.db.Begin(ctx)
	if err != nil {
		return alerts.Alert{}, false, fmt.Errorf("begin price alert creation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	q := store.queries.WithTx(tx)
	telegramID, err := lockUser(ctx, q, userID)
	if err != nil {
		return alerts.Alert{}, false, err
	}
	instrumentID, err := activeInstrumentID(ctx, q, symbol)
	if err != nil {
		return alerts.Alert{}, false, err
	}
	added, err := q.AddFavorite(ctx, generated.AddFavoriteParams{UserID: userID, InstrumentID: instrumentID})
	if err != nil {
		return alerts.Alert{}, false, fmt.Errorf("add favorite for price alert: %w", err)
	}
	count, err := q.CountFavoriteAlerts(ctx, generated.CountFavoriteAlertsParams{UserID: userID, InstrumentID: instrumentID})
	if err != nil {
		return alerts.Alert{}, false, fmt.Errorf("count price alerts: %w", err)
	}
	if count >= alerts.MaxPerInstrument {
		return alerts.Alert{}, false, alerts.ErrLimit
	}
	row, err := q.InsertPriceAlert(ctx, generated.InsertPriceAlertParams{UserID: userID, InstrumentID: instrumentID, Target: target})
	if duplicateViolation(err) {
		return alerts.Alert{}, false, alerts.ErrDuplicate
	}
	if err != nil {
		return alerts.Alert{}, false, fmt.Errorf("insert price alert: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return alerts.Alert{}, false, fmt.Errorf("commit price alert creation: %w", err)
	}
	item, err := alertFromRow(row, symbol, telegramID)
	return item, added > 0, err
}

func (store *Store) UpdateAlert(ctx context.Context, userID, id int64, target string) (alerts.Alert, error) {
	tx, err := store.db.Begin(ctx)
	if err != nil {
		return alerts.Alert{}, fmt.Errorf("begin price alert update: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	q := store.queries.WithTx(tx)
	telegramID, err := lockUser(ctx, q, userID)
	if err != nil {
		return alerts.Alert{}, err
	}
	symbol, err := q.LockPriceAlertSymbol(ctx, generated.LockPriceAlertSymbolParams{ID: id, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return alerts.Alert{}, alerts.ErrNotFound
	}
	if err != nil {
		return alerts.Alert{}, fmt.Errorf("lock price alert: %w", err)
	}
	row, err := q.UpdatePriceAlert(ctx, generated.UpdatePriceAlertParams{ID: id, UserID: userID, Target: target})
	if duplicateViolation(err) {
		return alerts.Alert{}, alerts.ErrDuplicate
	}
	if err != nil {
		return alerts.Alert{}, fmt.Errorf("update price alert: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return alerts.Alert{}, fmt.Errorf("commit price alert update: %w", err)
	}
	return alertFromRow(row, symbol, telegramID)
}

func (store *Store) DeleteAlert(ctx context.Context, userID, id int64) error {
	tx, err := store.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin price alert deletion: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	q := store.queries.WithTx(tx)
	if _, err = lockUser(ctx, q, userID); err != nil {
		return err
	}
	rows, err := q.DeletePriceAlert(ctx, generated.DeletePriceAlertParams{ID: id, UserID: userID})
	if err != nil {
		return fmt.Errorf("delete price alert: %w", err)
	}
	if rows == 0 {
		return alerts.ErrNotFound
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit price alert deletion: %w", err)
	}
	return nil
}

func (store *Store) ListEnabledAlerts(ctx context.Context) ([]alerts.Alert, error) {
	rows, err := store.queries.ListEnabledPriceAlerts(ctx)
	if err != nil {
		return nil, fmt.Errorf("list enabled price alerts: %w", err)
	}
	items := make([]alerts.Alert, 0, len(rows))
	for _, row := range rows {
		item, err := alertFromRow(row.AppPriceAlert, row.Symbol, row.TelegramID)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (store *Store) FireAlert(ctx context.Context, id, version int64) (bool, error) {
	_, err := store.queries.FirePriceAlert(ctx, generated.FirePriceAlertParams{ID: id, Version: version})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("fire price alert: %w", err)
	}
	return true, nil
}

func (store *Store) ListMonitoredSymbols(ctx context.Context) ([]string, error) {
	symbols, err := store.queries.ListMonitoredSymbols(ctx)
	if err != nil {
		return nil, fmt.Errorf("list monitored symbols: %w", err)
	}
	return symbols, nil
}

// alertFromRow normalizes the persisted NUMERIC target, which carries the
// column's trailing zeros.
func alertFromRow(row generated.AppPriceAlert, symbol string, telegramID int64) (alerts.Alert, error) {
	target, err := alerts.NormalizeTarget(row.Target)
	if err != nil {
		return alerts.Alert{}, fmt.Errorf("invalid persisted alert target: %w", err)
	}
	return alerts.Alert{
		ID: row.ID, UserID: row.UserID, TelegramID: telegramID, InstrumentID: row.InstrumentID, Symbol: symbol, Target: target,
		Version: row.Version, CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
	}, nil
}
