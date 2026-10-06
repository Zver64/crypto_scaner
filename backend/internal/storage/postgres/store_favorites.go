package postgres

import (
	"context"
	"errors"
	"fmt"

	"crypto-scanner/internal/favorites"
	generated "crypto-scanner/internal/storage/postgres/sqlc"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ListFavorites returns saved instruments even after delisting.
func (store *Store) ListFavorites(ctx context.Context, userID int64) ([]favorites.Favorite, error) {
	rows, err := store.queries.ListFavorites(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list favorites: %w", err)
	}
	items := make([]favorites.Favorite, 0, len(rows))
	for _, row := range rows {
		items = append(items, favoriteFromRow(row.BinanceSpotInstrument, row.CreatedAt, row.AlertCount))
	}
	return items, nil
}

func (store *Store) AddFavorite(ctx context.Context, userID int64, symbol string) (favorites.Favorite, error) {
	tx, err := store.db.Begin(ctx)
	if err != nil {
		return favorites.Favorite{}, fmt.Errorf("begin favorite addition: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	q := store.queries.WithTx(tx)
	if _, err = lockUser(ctx, q, userID); err != nil {
		return favorites.Favorite{}, err
	}
	instrumentID, err := activeInstrumentID(ctx, q, symbol)
	if err != nil {
		return favorites.Favorite{}, err
	}
	if _, err = q.AddFavorite(ctx, generated.AddFavoriteParams{UserID: userID, InstrumentID: instrumentID}); err != nil {
		return favorites.Favorite{}, fmt.Errorf("add favorite: %w", err)
	}
	row, err := q.GetFavorite(ctx, generated.GetFavoriteParams{UserID: userID, InstrumentID: instrumentID})
	if err != nil {
		return favorites.Favorite{}, fmt.Errorf("get added favorite: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return favorites.Favorite{}, fmt.Errorf("commit favorite addition: %w", err)
	}
	return favoriteFromRow(row.BinanceSpotInstrument, row.CreatedAt, row.AlertCount), nil
}

func favoriteFromRow(instrument generated.BinanceSpotInstrument, createdAt pgtype.Timestamptz, alertCount int32) favorites.Favorite {
	return favorites.Favorite{
		InstrumentID: instrument.ID, Symbol: instrument.Symbol, BaseAsset: instrument.BaseAsset, QuoteAsset: instrument.QuoteAsset,
		Active: instrument.IsActive, AlertCount: int(alertCount), CreatedAt: createdAt.Time.UTC(),
	}
}

// RemoveFavorite fails with strategy.InstrumentsInUseError when the
// administrator removes a coin that strategies read through of.
func (store *Store) RemoveFavorite(ctx context.Context, userID, administratorTelegramID int64, symbol string, confirm bool) (int, error) {
	tx, err := store.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin favorite removal: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	q := store.queries.WithTx(tx)
	telegramID, err := lockUser(ctx, q, userID)
	if err != nil {
		return 0, err
	}
	instrumentID, err := q.LockFavoriteInstrumentID(ctx, generated.LockFavoriteInstrumentIDParams{UserID: userID, Symbol: symbol})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, favorites.ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("lock favorite: %w", err)
	}
	if telegramID == administratorTelegramID {
		if err := strategiesReading(ctx, q, instrumentID); err != nil {
			return 0, err
		}
	}
	count, err := q.CountFavoriteAlerts(ctx, generated.CountFavoriteAlertsParams{UserID: userID, InstrumentID: instrumentID})
	if err != nil {
		return 0, fmt.Errorf("count favorite price alerts: %w", err)
	}
	if count > 0 && !confirm {
		return int(count), favorites.ErrAlertsExist
	}
	rows, err := q.DeleteFavorite(ctx, generated.DeleteFavoriteParams{UserID: userID, InstrumentID: instrumentID})
	if err != nil {
		return 0, fmt.Errorf("delete favorite: %w", err)
	}
	if rows == 0 {
		return 0, favorites.ErrNotFound
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit favorite removal: %w", err)
	}
	return int(count), nil
}
