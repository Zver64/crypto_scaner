package markettable

import (
	"context"
	"errors"
	"slices"
	"time"

	"crypto-scanner/internal/analysis"
	"crypto-scanner/internal/closedindicator"
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

// ClosedIndicators supplies indicator values at the latest closed candle.
type ClosedIndicators interface {
	// Latest never fails: unavailable values have no outputs.
	Latest(context.Context, []int64) map[int64][]closedindicator.Value
}

// Prices reads the closes price histories are made of.
type Prices interface {
	// ListClosePrices returns the closes of the interval candles opening
	// from through to.
	ListClosePrices(context.Context, []int64, market.CandleInterval, time.Time, time.Time) ([]market.ClosePrice, error)
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
	closed     ClosedIndicators
	prices     Prices
	perpetuals CoinMPerpetuals
	market     Catalog
	coinM      Catalog
	favorites  Catalog
}

// NewService builds market tables with market, COIN-M tables with coinM, and
// favorites tables with favorites.
func NewService(analyzer Analyzer, closed ClosedIndicators, prices Prices, perpetuals CoinMPerpetuals, market, coinM, favorites Catalog) (*Service, error) {
	if analyzer == nil || closed == nil || prices == nil || perpetuals == nil {
		return nil, errors.New("market table service dependencies are required")
	}
	return &Service{analyzer: analyzer, closed: closed, prices: prices, perpetuals: perpetuals, market: market, coinM: coinM, favorites: favorites}, nil
}

func (service *Service) Search(ctx context.Context, request analysis.SearchRequest) (Result, error) {
	windows := market.NewPriceWindows(time.Now())
	search, err := service.analyzer.Search(ctx, request)
	if err != nil {
		return Result{}, err
	}
	coinM, err := service.CoinMSymbols(ctx)
	if err != nil {
		return Result{}, err
	}
	return service.searchResult(ctx, search, service.market, coinM, windows)
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
	windows := market.NewPriceWindows(time.Now())
	search, err := service.analyzer.SearchSymbols(ctx, request, symbols)
	if err != nil {
		return Result{}, err
	}
	return service.searchResult(ctx, search, service.coinM, symbols, windows)
}

// markets lists the markets the coin of a Spot USDT symbol trades on.
func markets(symbol string, coinM []string) []Market {
	if slices.Contains(coinM, symbol) {
		return []Market{MarketSpot, MarketCoinMPerpetual}
	}
	return []Market{MarketSpot}
}

// searchResult presents the analyzed instruments of search as rows of
// catalog.
func (service *Service) searchResult(ctx context.Context, search analysis.SearchResult, catalog Catalog, coinM []string, windows market.PriceWindows) (Result, error) {
	ids := make([]int64, len(search.Items))
	for i, item := range search.Items {
		ids[i] = item.InstrumentID
	}
	rows, err := service.rows(ctx, ids, windows)
	if err != nil {
		return Result{}, err
	}
	for i, item := range search.Items {
		rows[i].Symbol, rows[i].Evaluations = item.Symbol, item.Evaluations
		rows[i].Markets = markets(item.Symbol, coinM)
	}
	return Result{Search: search, Table: catalog.Build(rows, windows)}, nil
}

// rows returns a row of each instrument, in ids order, with the cells that
// do not depend on an analysis: its price histories over windows and its
// closed indicator values.
func (service *Service) rows(ctx context.Context, ids []int64, windows market.PriceWindows) ([]Row, error) {
	histories, err := service.priceHistories(ctx, ids, windows)
	if err != nil {
		return nil, err
	}
	closed := service.closed.Latest(ctx, ids)
	rows := make([]Row, len(ids))
	for i, id := range ids {
		rows[i] = Row{PriceHistories: histories[id], ClosedIndicators: closed[id]}
	}
	return rows, nil
}

// Favorites analyzes the active favorites and returns a table row for every
// favorite, in favorites order. Favorites the analysis skipped or rejected keep
// the cells that do not depend on it, such as closed indicator values and
// price histories.
func (service *Service) Favorites(ctx context.Context, items []favorites.Favorite, request analysis.SearchRequest) (Result, error) {
	windows := market.NewPriceWindows(time.Now())
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
	ids := make([]int64, len(items))
	for i, item := range items {
		ids[i] = item.InstrumentID
	}
	rows, err := service.rows(ctx, ids, windows)
	if err != nil {
		return Result{}, err
	}
	coinM, err := service.CoinMSymbols(ctx)
	if err != nil {
		return Result{}, err
	}
	for i, item := range items {
		rows[i].Symbol = item.Symbol
		if analyzedItem, ok := analyzed[item.Symbol]; ok {
			rows[i].Evaluations = analyzedItem.Evaluations
		}
		rows[i].Markets = markets(item.Symbol, coinM)
		rows[i].AlertCount = item.AlertCount
	}
	return Result{Search: search, Table: service.favorites.Build(rows, windows)}, nil
}
