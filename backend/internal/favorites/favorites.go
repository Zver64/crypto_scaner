// Package favorites contains per-user favorite instrument use cases.
package favorites

import (
	"context"
	"errors"
	"fmt"
	"time"

	"crypto-scanner/internal/analysis"
	"crypto-scanner/internal/markettable"
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
	table    markettable.Catalog
}

func New(store Store, changed func(), analyzer Analyzer, closed analysis.ClosedIndicators, table markettable.Catalog) *Service {
	return &Service{store: store, changed: changed, analyzer: analyzer, closed: closed, table: table}
}
func (s *Service) List(ctx context.Context, userID int64) ([]Favorite, error) {
	return s.store.ListFavorites(ctx, userID)
}

// Analyze analyzes the active favorites and returns a table row for every
// favorite, in favorites order. Favorites the analysis skipped or rejected keep
// the cells that do not depend on it, such as closed indicator values.
func (s *Service) Analyze(ctx context.Context, userID int64, request analysis.SearchRequest) (markettable.Result, error) {
	items, err := s.store.ListFavorites(ctx, userID)
	if err != nil {
		return markettable.Result{}, err
	}
	if s.analyzer == nil || s.closed == nil {
		return markettable.Result{}, fmt.Errorf("favorites analyzer is unavailable")
	}
	symbols := make([]string, 0, len(items))
	for _, item := range items {
		if item.Active {
			symbols = append(symbols, item.Symbol)
		}
	}
	search, err := s.analyzer.SearchSymbols(ctx, request, symbols)
	if err != nil {
		return markettable.Result{}, err
	}
	analyzed := make(map[string]analysis.SearchItem, len(search.Items))
	for _, item := range search.Items {
		analyzed[item.Symbol] = item
	}
	var missing []int64
	for _, item := range items {
		if _, ok := analyzed[item.Symbol]; !ok {
			missing = append(missing, item.InstrumentID)
		}
	}
	closed := s.closed.Latest(ctx, missing)
	rows := make([]markettable.Row, len(items))
	for i, item := range items {
		row := markettable.Row{Symbol: item.Symbol, ClosedIndicators: closed[item.InstrumentID]}
		if analyzedItem, ok := analyzed[item.Symbol]; ok {
			row = markettable.RowFromSearchItem(analyzedItem)
		}
		row.AlertCount = item.AlertCount
		rows[i] = row
	}
	return markettable.Result{Search: search, Table: s.table.Build(rows)}, nil
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
