package postgres

import (
	"context"
	"errors"
	"fmt"

	"crypto-scanner/internal/alerts"
	"crypto-scanner/internal/market"
	generated "crypto-scanner/internal/storage/postgres/sqlc"

	"github.com/jackc/pgx/v5"
)

func (store *Store) ListAlerts(ctx context.Context, userID int64, symbol string) ([]alerts.Alert, error) {
	rows, err := store.queries.ListPriceAlerts(ctx, generated.ListPriceAlertsParams{UserID: userID, Symbol: symbol})
	if err != nil {
		return nil, err
	}
	items := make([]alerts.Alert, 0, len(rows))
	for _, r := range rows {
		target, normalizeErr := alerts.NormalizeTarget(r.Target)
		if normalizeErr != nil {
			return nil, fmt.Errorf("invalid persisted alert target: %w", normalizeErr)
		}
		items = append(items, alerts.Alert{ID: r.ID, UserID: r.UserID, InstrumentID: r.InstrumentID, Symbol: r.Symbol, Target: target, Version: r.Version, CreatedAt: r.CreatedAt.Time.UTC(), UpdatedAt: r.UpdatedAt.Time.UTC()})
	}
	return items, nil
}

func (store *Store) CreateAlert(ctx context.Context, userID int64, symbol, target string) (alerts.Alert, error) {
	tx, err := store.db.Begin(ctx)
	if err != nil {
		return alerts.Alert{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	telegramID, err := lockUser(ctx, tx, userID)
	if err != nil {
		return alerts.Alert{}, err
	}
	var instrumentID int64
	if err = tx.QueryRow(ctx, `SELECT id FROM binance_spot.instruments WHERE symbol=$1 AND is_active`, symbol).Scan(&instrumentID); errors.Is(err, pgx.ErrNoRows) {
		return alerts.Alert{}, market.ErrInstrumentNotFound
	} else if err != nil {
		return alerts.Alert{}, err
	}
	q := store.queries.WithTx(tx)
	if err = q.AddFavorite(ctx, generated.AddFavoriteParams{UserID: userID, InstrumentID: instrumentID}); err != nil {
		return alerts.Alert{}, err
	}
	count, err := q.CountFavoriteAlerts(ctx, generated.CountFavoriteAlertsParams{UserID: userID, InstrumentID: instrumentID})
	if err != nil {
		return alerts.Alert{}, err
	}
	if count >= alerts.MaxPerInstrument {
		return alerts.Alert{}, alerts.ErrLimit
	}
	r, err := q.InsertPriceAlert(ctx, generated.InsertPriceAlertParams{UserID: userID, InstrumentID: instrumentID, Target: target})
	if duplicateViolation(err) {
		return alerts.Alert{}, alerts.ErrDuplicate
	}
	if err != nil {
		return alerts.Alert{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return alerts.Alert{}, err
	}
	return alertFromInsert(r, symbol, target, telegramID), nil
}

func (store *Store) UpdateAlert(ctx context.Context, userID, id int64, target string) (alerts.Alert, error) {
	tx, err := store.db.Begin(ctx)
	if err != nil {
		return alerts.Alert{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	telegramID, err := lockUser(ctx, tx, userID)
	if err != nil {
		return alerts.Alert{}, err
	}
	var symbol string
	if err = tx.QueryRow(ctx, `SELECT i.symbol FROM app.price_alerts a JOIN binance_spot.instruments i ON i.id=a.instrument_id WHERE a.id=$1 AND a.user_id=$2 FOR UPDATE OF a`, id, userID).Scan(&symbol); errors.Is(err, pgx.ErrNoRows) {
		return alerts.Alert{}, alerts.ErrNotFound
	} else if err != nil {
		return alerts.Alert{}, err
	}
	r, err := store.queries.WithTx(tx).UpdatePriceAlert(ctx, generated.UpdatePriceAlertParams{ID: id, UserID: userID, Target: target})
	if duplicateViolation(err) {
		return alerts.Alert{}, alerts.ErrDuplicate
	}
	if err != nil {
		return alerts.Alert{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return alerts.Alert{}, err
	}
	return alerts.Alert{ID: r.ID, UserID: r.UserID, TelegramID: telegramID, InstrumentID: r.InstrumentID, Symbol: symbol, Target: target, Version: r.Version, CreatedAt: r.CreatedAt.Time.UTC(), UpdatedAt: r.UpdatedAt.Time.UTC()}, nil
}

func (store *Store) DeleteAlert(ctx context.Context, userID, id int64) error {
	tx, err := store.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = lockUser(ctx, tx, userID); err != nil {
		return err
	}
	rows, err := store.queries.WithTx(tx).DeletePriceAlert(ctx, generated.DeletePriceAlertParams{ID: id, UserID: userID})
	if err != nil {
		return err
	}
	if rows == 0 {
		return alerts.ErrNotFound
	}
	return tx.Commit(ctx)
}

func (store *Store) ListEnabledAlerts(ctx context.Context) ([]alerts.Alert, error) {
	rows, err := store.queries.ListEnabledPriceAlerts(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]alerts.Alert, 0, len(rows))
	for _, r := range rows {
		target, normalizeErr := alerts.NormalizeTarget(r.Target)
		if normalizeErr != nil {
			return nil, fmt.Errorf("invalid persisted alert target: %w", normalizeErr)
		}
		items = append(items, alerts.Alert{ID: r.ID, UserID: r.UserID, TelegramID: r.TelegramID, InstrumentID: r.InstrumentID, Symbol: r.Symbol, Target: target, Version: r.Version, CreatedAt: r.CreatedAt.Time.UTC(), UpdatedAt: r.UpdatedAt.Time.UTC()})
	}
	return items, nil
}

func (store *Store) FireAlert(ctx context.Context, id, version int64) (bool, error) {
	_, err := store.queries.FirePriceAlert(ctx, generated.FirePriceAlertParams{ID: id, Version: version})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (store *Store) ListMonitoredSymbols(ctx context.Context) ([]string, error) {
	return store.queries.ListMonitoredSymbols(ctx)
}

// ListMonitoredInstrumentIDs returns active instruments favorited by enabled users.
func (store *Store) ListMonitoredInstrumentIDs(ctx context.Context) ([]int64, error) {
	return store.queries.ListMonitoredInstrumentIDs(ctx)
}

func alertFromInsert(r generated.InsertPriceAlertRow, symbol, target string, telegramID int64) alerts.Alert {
	return alerts.Alert{ID: r.ID, UserID: r.UserID, TelegramID: telegramID, InstrumentID: r.InstrumentID, Symbol: symbol, Target: target, Version: r.Version, CreatedAt: r.CreatedAt.Time.UTC(), UpdatedAt: r.UpdatedAt.Time.UTC()}
}
