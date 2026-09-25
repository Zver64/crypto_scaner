// Package favorites contains per-user favorite instrument use cases.
package favorites

import (
	"context"
	"errors"
	"fmt"
	"time"

	"crypto-scanner/internal/analysis"
	"crypto-scanner/internal/closedindicator"
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
	// ClosedIndicators is filled regardless of analysis criteria outcomes.
	ClosedIndicators []closedindicator.Value
}

type Store interface {
	ListFavorites(context.Context, int64) ([]Favorite, error)
	ListFavoriteSymbols(context.Context, int64) ([]string, error)
	AddFavorite(context.Context, int64, string) (Favorite, error)
	RemoveFavorite(context.Context, int64, string, bool) (int, error)
}

type Analyzer interface {
	SearchSymbols(context.Context, analysis.SearchRequest, []string) (analysis.SearchResult, error)
}

type Service struct {
	store    Store
	changed  func()
	analyzer Analyzer
	closed   analysis.ClosedIndicators
}

func New(store Store, changed func(), analyzer Analyzer, closed analysis.ClosedIndicators) *Service {
	return &Service{store: store, changed: changed, analyzer: analyzer, closed: closed}
}
func (s *Service) List(ctx context.Context, userID int64) ([]Favorite, error) {
	items, err := s.store.ListFavorites(ctx, userID)
	if err != nil || s.closed == nil || len(items) == 0 {
		return items, err
	}
	ids := make([]int64, len(items))
	for index, item := range items {
		ids[index] = item.InstrumentID
	}
	values := s.closed.Latest(ctx, ids)
	for index := range items {
		items[index].ClosedIndicators = values[items[index].InstrumentID]
	}
	return items, nil
}
func (s *Service) Analyze(ctx context.Context, userID int64, request analysis.SearchRequest) (analysis.SearchResult, error) {
	symbols, err := s.store.ListFavoriteSymbols(ctx, userID)
	if err != nil {
		return analysis.SearchResult{}, err
	}
	if s.analyzer == nil {
		return analysis.SearchResult{}, fmt.Errorf("favorites analyzer is unavailable")
	}
	return s.analyzer.SearchSymbols(ctx, request, symbols)
}
func (s *Service) Add(ctx context.Context, userID int64, symbol string) (Favorite, error) {
	item, err := s.store.AddFavorite(ctx, userID, symbol)
	if err == nil && s.changed != nil {
		s.changed()
	}
	return item, err
}
func (s *Service) Remove(ctx context.Context, userID int64, symbol string, confirm bool) (int, error) {
	count, err := s.store.RemoveFavorite(ctx, userID, symbol, confirm)
	if err == nil && s.changed != nil {
		s.changed()
	}
	return count, err
}
