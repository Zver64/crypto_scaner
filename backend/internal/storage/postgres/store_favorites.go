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
	var row generated.GetFavoriteRow
	err := store.inTx(ctx, "favorite addition", func(q *generated.Queries) error {
		if _, err := lockUser(ctx, q, userID); err != nil {
			return err
		}
		instrumentID, err := activeInstrumentID(ctx, q, symbol)
		if err != nil {
			return err
		}
		if _, err = q.AddFavorite(ctx, generated.AddFavoriteParams{UserID: userID, InstrumentID: instrumentID}); err != nil {
			return fmt.Errorf("add favorite: %w", err)
		}
		if row, err = q.GetFavorite(ctx, generated.GetFavoriteParams{UserID: userID, InstrumentID: instrumentID}); err != nil {
			return fmt.Errorf("get added favorite: %w", err)
		}
		return nil
	})
	if err != nil {
		return favorites.Favorite{}, err
	}
	return favoriteFromRow(row.BinanceSpotInstrument, row.CreatedAt, row.AlertCount), nil
}

func favoriteFromRow(instrument generated.BinanceSpotInstrument, createdAt pgtype.Timestamptz, alertCount int32) favorites.Favorite {
	return favorites.Favorite{
		InstrumentID: instrument.ID, Symbol: instrument.Symbol, BaseAsset: instrument.BaseAsset, QuoteAsset: instrument.QuoteAsset,
		Active: instrument.IsActive, AlertCount: int(alertCount), CreatedAt: createdAt.Time.UTC(),
	}
}

func (store *Store) RemoveFavorite(ctx context.Context, userID int64, symbol string, confirm bool) (int, error) {
	var count int64
	err := store.inTx(ctx, "favorite removal", func(q *generated.Queries) error {
		if _, err := lockUser(ctx, q, userID); err != nil {
			return err
		}
		instrumentID, err := q.LockFavoriteInstrumentID(ctx, generated.LockFavoriteInstrumentIDParams{UserID: userID, Symbol: symbol})
		if errors.Is(err, pgx.ErrNoRows) {
			return favorites.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("lock favorite: %w", err)
		}
		if count, err = q.CountFavoriteAlerts(ctx, generated.CountFavoriteAlertsParams{UserID: userID, InstrumentID: instrumentID}); err != nil {
			return fmt.Errorf("count favorite price alerts: %w", err)
		}
		if count > 0 && !confirm {
			return favorites.ErrAlertsExist
		}
		rows, err := q.DeleteFavorite(ctx, generated.DeleteFavoriteParams{UserID: userID, InstrumentID: instrumentID})
		if err != nil {
			return fmt.Errorf("delete favorite: %w", err)
		}
		if rows == 0 {
			return favorites.ErrNotFound
		}
		return nil
	})
	switch {
	case errors.Is(err, favorites.ErrAlertsExist):
		return int(count), err
	case err != nil:
		return 0, err
	}
	return int(count), nil
}
