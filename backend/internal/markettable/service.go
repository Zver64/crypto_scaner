package markettable

import (
	"context"
	"errors"
	"slices"

	"crypto-scanner/internal/analysis"
	"crypto-scanner/internal/favorites"
	"crypto-scanner/internal/market"
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

// CoinMPerpetuals lists the stored base assets of Binance COIN-M perpetual
// contracts.
type CoinMPerpetuals interface {
	ListCoinMPerpetuals(context.Context) ([]string, error)
}

// Service runs market and favorites analyses and presents the instruments as
// tables.
type Service struct {
	analyzer   Analyzer
	closed     analysis.ClosedIndicators
	perpetuals CoinMPerpetuals
	market     Catalog
	coinM      Catalog
	favorites  Catalog
}

// NewService builds market tables with market, COIN-M tables with coinM, and
// favorites tables with favorites.
func NewService(analyzer Analyzer, closed analysis.ClosedIndicators, perpetuals CoinMPerpetuals, market, coinM, favorites Catalog) (*Service, error) {
	if analyzer == nil || closed == nil || perpetuals == nil {
		return nil, errors.New("market table service dependencies are required")
	}
	return &Service{analyzer: analyzer, closed: closed, perpetuals: perpetuals, market: market, coinM: coinM, favorites: favorites}, nil
}

func (service *Service) Search(ctx context.Context, request analysis.SearchRequest) (Result, error) {
	search, err := service.analyzer.Search(ctx, request)
	if err != nil {
		return Result{}, err
	}
	coinM, err := service.CoinMSymbols(ctx)
	if err != nil {
		return Result{}, err
	}
	return searchResult(search, service.market, coinM), nil
}

// CoinMSymbols lists the Spot USDT symbols of the coins that have a Binance
// COIN-M perpetual contract, active or not.
func (service *Service) CoinMSymbols(ctx context.Context) ([]string, error) {
	assets, err := service.perpetuals.ListCoinMPerpetuals(ctx)
	if err != nil {
		return nil, err
	}
	symbols := make([]string, len(assets))
	for i, asset := range assets {
		symbols[i] = market.NormalizeSymbol(asset + market.BinanceSpotQuoteAsset)
	}
	return symbols, nil
}

// CoinM analyzes the Spot USDT instruments of the coins that have a Binance
// COIN-M perpetual contract.
func (service *Service) CoinM(ctx context.Context, request analysis.SearchRequest) (Result, error) {
	symbols, err := service.CoinMSymbols(ctx)
	if err != nil {
		return Result{}, err
	}
	search, err := service.analyzer.SearchSymbols(ctx, request, symbols)
	if err != nil {
		return Result{}, err
	}
	return searchResult(search, service.coinM, symbols), nil
}

// markets lists the markets the coin of a Spot USDT symbol trades on.
func markets(symbol string, coinM []string) []Market {
	if slices.Contains(coinM, symbol) {
		return []Market{MarketSpot, MarketCoinMPerpetual}
	}
	return []Market{MarketSpot}
}

func searchResult(search analysis.SearchResult, catalog Catalog, coinM []string) Result {
	rows := make([]Row, len(search.Items))
	for i, item := range search.Items {
		rows[i] = RowFromSearchItem(item)
		rows[i].Markets = markets(item.Symbol, coinM)
	}
	return Result{Search: search, Table: catalog.Build(rows)}
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
	coinM, err := service.CoinMSymbols(ctx)
	if err != nil {
		return Result{}, err
	}
	rows := make([]Row, len(items))
	for i, item := range items {
		row := Row{Symbol: item.Symbol, ClosedIndicators: closed[item.InstrumentID]}
		if analyzedItem, ok := analyzed[item.Symbol]; ok {
			row = RowFromSearchItem(analyzedItem)
		}
		row.Markets = markets(item.Symbol, coinM)
		row.AlertCount = item.AlertCount
		rows[i] = row
	}
	return Result{Search: search, Table: service.favorites.Build(rows)}, nil
}
