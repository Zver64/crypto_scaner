package analysis

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"crypto-scanner/internal/market"
)

// ErrMarketDataUnavailable reports that synchronization has never produced a
// successful market dataset.
var ErrMarketDataUnavailable = errors.New("market data unavailable")

// ErrSymbolNotFound reports an unknown or inactive symbol.
var ErrSymbolNotFound = errors.New("symbol not found")

// Store is the synchronized market-data seam required by analysis.
type Store interface {
	GetSyncState(context.Context, market.SyncProfile) (market.SyncState, error)
	ListActiveInstruments(context.Context) ([]market.Instrument, error)
	ListLatestCandles(context.Context, int64, int) ([]market.Candle, error)
}

// SymbolRequest describes one-symbol percentile analysis.
type SymbolRequest struct {
	Symbol     string
	PeriodDays int
	Percentile float64
}

// SymbolResult contains an unrounded percentile and UTC candle coverage.
type SymbolResult struct {
	Symbol       string
	RangePercent float64
	CandleCount  int
	From         time.Time
	To           time.Time
}

// SearchRequest describes market-wide percentile filtering.
type SearchRequest struct {
	PeriodDays          int
	Percentile          float64
	MinimumRangePercent float64
}

// SearchItem is one active instrument meeting the market-search threshold.
type SearchItem struct {
	Symbol       string
	RangePercent float64
	CandleCount  int
}

// SearchResult contains matching instruments and market-wide analysis counts.
type SearchResult struct {
	MatchedCount          int
	AnalyzedCount         int
	InsufficientDataCount int
	Items                 []SearchItem
}

// Service assembles percentile analysis from synchronized market data.
type Service struct {
	store    Store
	analyzer Analyzer
}

// NewService constructs the application analysis service.
func NewService(store Store, analyzer Analyzer) *Service {
	return &Service{store: store, analyzer: analyzer}
}

// AnalyzeSymbol calculates a percentile for one active instrument.
func (service *Service) AnalyzeSymbol(ctx context.Context, request SymbolRequest) (SymbolResult, error) {
	if invalidAnalysisArguments(request.PeriodDays, request.Percentile) {
		return SymbolResult{}, ErrInvalidArgument
	}
	if err := service.requireMarketData(ctx); err != nil {
		return SymbolResult{}, err
	}
	instruments, err := service.store.ListActiveInstruments(ctx)
	if err != nil {
		return SymbolResult{}, fmt.Errorf("list active instruments: %w", err)
	}
	var instrument market.Instrument
	found := false
	for _, candidate := range instruments {
		if candidate.Symbol == request.Symbol {
			instrument = candidate
			found = true
			break
		}
	}
	if !found {
		return SymbolResult{}, ErrSymbolNotFound
	}
	candles, err := service.store.ListLatestCandles(ctx, instrument.ID, request.PeriodDays)
	if err != nil {
		return SymbolResult{}, fmt.Errorf("list latest candles for %s: %w", request.Symbol, err)
	}
	result, err := service.analyzer.Analyze(ctx, AnalysisInput{
		Candles: candles, PeriodDays: request.PeriodDays, Percentile: request.Percentile,
	})
	if err != nil {
		return SymbolResult{}, err
	}
	return SymbolResult{
		Symbol: request.Symbol, RangePercent: result.RangePercent, CandleCount: result.CandleCount,
		From: result.From, To: result.To,
	}, nil
}

// Search calculates a percentile for every active instrument and returns those
// meeting the unrounded minimum.
func (service *Service) Search(ctx context.Context, request SearchRequest) (SearchResult, error) {
	if invalidAnalysisArguments(request.PeriodDays, request.Percentile) ||
		math.IsNaN(request.MinimumRangePercent) || math.IsInf(request.MinimumRangePercent, 0) || request.MinimumRangePercent < 0 {
		return SearchResult{}, ErrInvalidArgument
	}
	if err := service.requireMarketData(ctx); err != nil {
		return SearchResult{}, err
	}
	instruments, err := service.store.ListActiveInstruments(ctx)
	if err != nil {
		return SearchResult{}, fmt.Errorf("list active instruments: %w", err)
	}
	result := SearchResult{Items: make([]SearchItem, 0)}
	for _, instrument := range instruments {
		candles, loadErr := service.store.ListLatestCandles(ctx, instrument.ID, request.PeriodDays)
		if loadErr != nil {
			return SearchResult{}, fmt.Errorf("list latest candles for %s: %w", instrument.Symbol, loadErr)
		}
		calculated, analyzeErr := service.analyzer.Analyze(ctx, AnalysisInput{
			Candles: candles, PeriodDays: request.PeriodDays, Percentile: request.Percentile,
		})
		var insufficient *InsufficientHistoryError
		if errors.As(analyzeErr, &insufficient) {
			result.InsufficientDataCount++
			continue
		}
		if analyzeErr != nil {
			return SearchResult{}, fmt.Errorf("analyze %s: %w", instrument.Symbol, analyzeErr)
		}
		result.AnalyzedCount++
		if calculated.RangePercent >= request.MinimumRangePercent {
			result.Items = append(result.Items, SearchItem{
				Symbol: instrument.Symbol, RangePercent: calculated.RangePercent, CandleCount: calculated.CandleCount,
			})
		}
	}
	sort.Slice(result.Items, func(i, j int) bool {
		if result.Items[i].RangePercent == result.Items[j].RangePercent {
			return result.Items[i].Symbol < result.Items[j].Symbol
		}
		return result.Items[i].RangePercent > result.Items[j].RangePercent
	})
	result.MatchedCount = len(result.Items)
	return result, nil
}

func invalidAnalysisArguments(periodDays int, percentile float64) bool {
	return periodDays < 1 || periodDays > 3650 || math.IsNaN(percentile) || math.IsInf(percentile, 0) || percentile < 0 || percentile > 100
}

var marketProfile = market.SyncProfile{
	Exchange: "binance", Market: "spot", QuoteAsset: "USDT", Interval: "1d", TimeZone: "UTC",
}

func (service *Service) requireMarketData(ctx context.Context) error {
	state, err := service.store.GetSyncState(ctx, marketProfile)
	if err != nil {
		return fmt.Errorf("get market synchronization state: %w", err)
	}
	if state.LastSucceededAt == nil {
		return ErrMarketDataUnavailable
	}
	return nil
}
