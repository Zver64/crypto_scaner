package markettable

import (
	"context"
	"errors"

	"crypto-scanner/internal/analysis"
	"crypto-scanner/internal/favorites"
)

// Result is a market analysis together with the table built from its items.
type Result struct {
	Search analysis.SearchResult
	Table  Table
}

type Analyzer interface {
	Search(context.Context, analysis.SearchRequest) (analysis.SearchResult, error)
	SearchSymbols(context.Context, analysis.SearchRequest, []string) (analysis.SearchResult, error)
}

// Service runs market and favorites analyses and presents the instruments as
// tables.
type Service struct {
	analyzer  Analyzer
	closed    analysis.ClosedIndicators
	market    Catalog
	favorites Catalog
}

// NewService builds market tables with market and favorites tables with
// favorites.
func NewService(analyzer Analyzer, closed analysis.ClosedIndicators, market, favorites Catalog) (*Service, error) {
	if analyzer == nil || closed == nil {
		return nil, errors.New("market table service dependencies are required")
	}
	return &Service{analyzer: analyzer, closed: closed, market: market, favorites: favorites}, nil
}

func (service *Service) Search(ctx context.Context, request analysis.SearchRequest) (Result, error) {
	search, err := service.analyzer.Search(ctx, request)
	if err != nil {
		return Result{}, err
	}
	rows := make([]Row, len(search.Items))
	for i, item := range search.Items {
		rows[i] = RowFromSearchItem(item)
	}
	return Result{Search: search, Table: service.market.Build(rows)}, nil
}

// Favorites analyzes the active favorites and returns a table row for every
// favorite, in favorites order. Favorites the analysis skipped or rejected keep
// the cells that do not depend on it, such as closed indicator values.
func (service *Service) Favorites(ctx context.Context, items []favorites.Favorite, request analysis.SearchRequest) (Result, error) {
	symbols := make([]string, 0, len(items))
	for _, item := range items {
		if item.Active {
			symbols = append(symbols, item.Symbol)
		}
	}
	search, err := service.analyzer.SearchSymbols(ctx, request, symbols)
	if err != nil {
		return Result{}, err
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
	closed := service.closed.Latest(ctx, missing)
	rows := make([]Row, len(items))
	for i, item := range items {
		row := Row{Symbol: item.Symbol, ClosedIndicators: closed[item.InstrumentID]}
		if analyzedItem, ok := analyzed[item.Symbol]; ok {
			row = RowFromSearchItem(analyzedItem)
		}
		row.AlertCount = item.AlertCount
		rows[i] = row
	}
	return Result{Search: search, Table: service.favorites.Build(rows)}, nil
}
