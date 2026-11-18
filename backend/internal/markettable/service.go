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

// Instruments lists the active instruments.
type Instruments interface {
	ListActiveInstruments(context.Context) ([]market.Instrument, error)
}

// Favorites lists the favorites of a user.
type Favorites interface {
	List(ctx context.Context, userID int64) ([]favorites.Favorite, error)
}

// Universe is a set of instruments a user analyzes or lists.
type Universe string

const (
	// UniverseSpot is every active Spot USDT instrument.
	UniverseSpot Universe = "spot"
	// UniverseCoinM is the active instruments of the coins that have a
	// Binance COIN-M perpetual contract.
	UniverseCoinM Universe = "coinm_perpetual"
	// UniverseFavorites is the user's favorites.
	UniverseFavorites Universe = "favorites"
)

// Service runs market and favorites analyses and presents the instruments as
// tables.
type Service struct {
	analyzer    Analyzer
	closed      ClosedIndicators
	prices      Prices
	perpetuals  CoinMPerpetuals
	instruments Instruments
	favorites   Favorites
	tables      Catalogs
}

// Catalogs are the tables of the universes.
type Catalogs struct {
	Market, CoinM, Favorites Catalog
}

// NewService builds the tables of each universe with its catalog.
func NewService(analyzer Analyzer, closed ClosedIndicators, prices Prices, perpetuals CoinMPerpetuals, instruments Instruments, favorites Favorites, tables Catalogs) (*Service, error) {
	if analyzer == nil || closed == nil || prices == nil || perpetuals == nil || instruments == nil || favorites == nil {
		return nil, errors.New("market table service dependencies are required")
	}
	return &Service{analyzer: analyzer, closed: closed, prices: prices, perpetuals: perpetuals, instruments: instruments, favorites: favorites, tables: tables}, nil
}

// Analyze analyzes the instruments of universe, spot when empty, for the
// user userID and presents them in the table of the universe.
func (service *Service) Analyze(ctx context.Context, userID int64, universe Universe, request analysis.SearchRequest) (Result, error) {
	switch universe {
	case UniverseCoinM:
		return service.coinM(ctx, request)
	case UniverseFavorites:
		items, err := service.favorites.List(ctx, userID)
		if err != nil {
			return Result{}, err
		}
		return service.favoritesTable(ctx, items, request)
	default:
		return service.search(ctx, request)
	}
}

// Symbols lists the symbols of the active instruments of universe, spot when
// empty, for the user userID: in symbol order, or the favorites order.
func (service *Service) Symbols(ctx context.Context, userID int64, universe Universe) ([]string, error) {
	if universe == UniverseFavorites {
		items, err := service.favorites.List(ctx, userID)
		if err != nil {
			return nil, err
		}
		var symbols []string
		for _, item := range items {
			if item.Active {
				symbols = append(symbols, item.Symbol)
			}
		}
		return symbols, nil
	}
	var coinM []string
	if universe == UniverseCoinM {
		var err error
		if coinM, err = service.coinMSymbols(ctx); err != nil {
			return nil, err
		}
	}
	instruments, err := service.instruments.ListActiveInstruments(ctx)
	if err != nil {
		return nil, err
	}
	symbols := make([]string, 0, len(instruments))
	for _, instrument := range instruments {
		if universe != UniverseCoinM || slices.Contains(coinM, instrument.Symbol) {
			symbols = append(symbols, instrument.Symbol)
		}
	}
	return symbols, nil
}

func (service *Service) search(ctx context.Context, request analysis.SearchRequest) (Result, error) {
	windows := market.NewPriceWindows(time.Now())
	search, err := service.analyzer.Search(ctx, request)
	if err != nil {
		return Result{}, err
	}
	coinM, err := service.coinMSymbols(ctx)
	if err != nil {
		return Result{}, err
	}
	return service.searchResult(ctx, search, service.tables.Market, coinM, windows)
}

// coinMSymbols lists the Spot USDT symbols of the coins that have a Binance
// COIN-M perpetual contract, active or not.
func (service *Service) coinMSymbols(ctx context.Context) ([]string, error) {
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

// coinM analyzes the Spot USDT instruments of the coins that have a Binance
// COIN-M perpetual contract.
func (service *Service) coinM(ctx context.Context, request analysis.SearchRequest) (Result, error) {
	symbols, err := service.coinMSymbols(ctx)
	if err != nil {
		return Result{}, err
	}
	windows := market.NewPriceWindows(time.Now())
	search, err := service.analyzer.SearchSymbols(ctx, request, symbols)
	if err != nil {
		return Result{}, err
	}
	return service.searchResult(ctx, search, service.tables.CoinM, symbols, windows)
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

// favoritesTable analyzes the active favorites and returns a table row for
// every favorite, in favorites order. Favorites the analysis skipped or
// rejected keep the cells that do not depend on it, such as closed indicator
// values and price histories.
func (service *Service) favoritesTable(ctx context.Context, items []favorites.Favorite, request analysis.SearchRequest) (Result, error) {
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
	coinM, err := service.coinMSymbols(ctx)
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
	return Result{Search: search, Table: service.tables.Favorites.Build(rows, windows)}, nil
}
