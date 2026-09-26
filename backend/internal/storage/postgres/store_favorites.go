package postgres

import (
	"context"
	"errors"
	"fmt"

	"crypto-scanner/internal/favorites"
	"crypto-scanner/internal/market"
	generated "crypto-scanner/internal/storage/postgres/sqlc"

	"github.com/jackc/pgx/v5"
)

// ListFavorites returns saved instruments even after delisting.
func (store *Store) ListFavorites(ctx context.Context, userID int64) ([]favorites.Favorite, error) {
	rows, err := store.queries.ListFavorites(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list favorites: %w", err)
	}
	items := make([]favorites.Favorite, 0, len(rows))
	for _, r := range rows {
		items = append(items, favorites.Favorite{InstrumentID: r.InstrumentID, Symbol: r.Symbol, BaseAsset: r.BaseAsset, QuoteAsset: r.QuoteAsset, Active: r.IsActive, AlertCount: int(r.AlertCount), CreatedAt: r.CreatedAt.Time.UTC()})
	}
	return items, nil
}

func (store *Store) ListFavoriteSymbols(ctx context.Context, userID int64) ([]string, error) {
	symbols, err := store.queries.ListFavoriteSymbols(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list favorite symbols: %w", err)
	}
	return symbols, nil
}

func (store *Store) AddFavorite(ctx context.Context, userID int64, symbol string) (favorites.Favorite, error) {
	tx, err := store.db.Begin(ctx)
	if err != nil {
		return favorites.Favorite{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var locked int64
	if err = tx.QueryRow(ctx, `SELECT id FROM app.users WHERE id=$1 AND is_enabled FOR UPDATE`, userID).Scan(&locked); err != nil {
		return favorites.Favorite{}, err
	}
	var instrumentID int64
	if err = tx.QueryRow(ctx, `SELECT id FROM binance_spot.instruments WHERE symbol=$1 AND is_active`, symbol).Scan(&instrumentID); errors.Is(err, pgx.ErrNoRows) {
		return favorites.Favorite{}, market.ErrInstrumentNotFound
	} else if err != nil {
		return favorites.Favorite{}, err
	}
	q := store.queries.WithTx(tx)
	if err = q.AddFavorite(ctx, generated.AddFavoriteParams{UserID: userID, InstrumentID: instrumentID}); err != nil {
		return favorites.Favorite{}, err
	}
	row, err := q.GetFavorite(ctx, generated.GetFavoriteParams{UserID: userID, Symbol: symbol})
	if err != nil {
		return favorites.Favorite{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return favorites.Favorite{}, err
	}
	return favoriteFromRow(row), nil
}

func favoriteFromRow(r generated.GetFavoriteRow) favorites.Favorite {
	return favorites.Favorite{InstrumentID: r.InstrumentID, Symbol: r.Symbol, BaseAsset: r.BaseAsset, QuoteAsset: r.QuoteAsset, Active: r.IsActive, AlertCount: int(r.AlertCount), CreatedAt: r.CreatedAt.Time.UTC()}
}

func (store *Store) RemoveFavorite(ctx context.Context, userID int64, symbol string, confirm bool) (int, error) {
	tx, err := store.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = lockUser(ctx, tx, userID); err != nil {
		return 0, err
	}
	var instrumentID int64
	if err = tx.QueryRow(ctx, `SELECT i.id FROM app.favorites f JOIN binance_spot.instruments i ON i.id=f.instrument_id WHERE f.user_id=$1 AND i.symbol=$2 FOR UPDATE OF f`, userID, symbol).Scan(&instrumentID); errors.Is(err, pgx.ErrNoRows) {
		return 0, favorites.ErrNotFound
	} else if err != nil {
		return 0, err
	}
	q := store.queries.WithTx(tx)
	count, err := q.CountFavoriteAlerts(ctx, generated.CountFavoriteAlertsParams{UserID: userID, InstrumentID: instrumentID})
	if err != nil {
		return 0, err
	}
	if count > 0 && !confirm {
		return int(count), favorites.ErrAlertsExist
	}
	rows, err := q.DeleteFavorite(ctx, generated.DeleteFavoriteParams{UserID: userID, InstrumentID: instrumentID})
	if err != nil {
		return 0, err
	}
	if rows == 0 {
		return 0, favorites.ErrNotFound
	}
	return int(count), tx.Commit(ctx)
}
