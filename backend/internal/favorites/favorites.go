// Package favorites contains per-user favorite instrument use cases.
package favorites

import (
	"context"
	"errors"
	"time"

	"crypto-scanner/internal/market"
)

var (
	ErrNotFound    = errors.New("favorite not found")
	ErrAlertsExist = errors.New("favorite has price alerts")
)

type Favorite struct {
	InstrumentID int64
	Symbol       string
	BaseAsset    string
	QuoteAsset   string
	Active       bool
	AlertCount   int
	CreatedAt    time.Time
}

type Store interface {
	ListFavorites(context.Context, int64) ([]Favorite, error)
	AddFavorite(context.Context, int64, string) (Favorite, error)
	// RemoveFavorite fails with strategy.InstrumentsInUseError when the user
	// is the administrator, administratorTelegramID, and strategies read the
	// coin.
	RemoveFavorite(ctx context.Context, userID, administratorTelegramID int64, symbol string, confirm bool) (int, error)
}

type Service struct {
	store Store
	// administratorID cannot remove favorites that strategies read.
	administratorID int64
	changed         func()
}

func New(store Store, administratorID int64, changed func()) (*Service, error) {
	if store == nil || changed == nil {
		return nil, errors.New("favorites service dependencies are required")
	}
	return &Service{store: store, administratorID: administratorID, changed: changed}, nil
}
func (s *Service) List(ctx context.Context, userID int64) ([]Favorite, error) {
	return s.store.ListFavorites(ctx, userID)
}

func (s *Service) Add(ctx context.Context, userID int64, symbol string) (Favorite, error) {
	item, err := s.store.AddFavorite(ctx, userID, market.NormalizeSymbol(symbol))
	if err != nil {
		return Favorite{}, err
	}
	s.changed()
	return item, nil
}
func (s *Service) Remove(ctx context.Context, userID int64, symbol string, confirm bool) (int, error) {
	count, err := s.store.RemoveFavorite(ctx, userID, s.administratorID, market.NormalizeSymbol(symbol), confirm)
	if err != nil {
		return count, err
	}
	s.changed()
	return count, nil
}
