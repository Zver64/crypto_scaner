// Package favorites contains per-user favorite instrument use cases.
package favorites

import (
	"context"
	"errors"
	"fmt"
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
	RemoveFavorite(ctx context.Context, userID int64, symbol string, confirm bool) (int, error)
}

type Service struct {
	store   Store
	changed func(ctx context.Context, userID int64)
}

// New creates the service; changed runs after every change of the favorites
// of the user userID.
func New(store Store, changed func(ctx context.Context, userID int64)) (*Service, error) {
	if store == nil || changed == nil {
		return nil, errors.New("favorites service dependencies are required")
	}
	return &Service{store: store, changed: changed}, nil
}
func (s *Service) List(ctx context.Context, userID int64) ([]Favorite, error) {
	items, err := s.store.ListFavorites(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list favorites of user %d: %w", userID, err)
	}
	return items, nil
}

func (s *Service) Add(ctx context.Context, userID int64, symbol string) (Favorite, error) {
	item, err := s.store.AddFavorite(ctx, userID, market.NormalizeSymbol(symbol))
	if err != nil {
		return Favorite{}, fmt.Errorf("add favorite %s of user %d: %w", symbol, userID, err)
	}
	s.changed(ctx, userID)
	return item, nil
}
func (s *Service) Remove(ctx context.Context, userID int64, symbol string, confirm bool) (int, error) {
	count, err := s.store.RemoveFavorite(ctx, userID, market.NormalizeSymbol(symbol), confirm)
	if err != nil {
		return count, fmt.Errorf("remove favorite %s of user %d: %w", symbol, userID, err)
	}
	s.changed(ctx, userID)
	return count, nil
}
