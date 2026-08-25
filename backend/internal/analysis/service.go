package analysis

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"crypto-scanner/internal/market"
)

var ErrMarketDataUnavailable = errors.New("market data unavailable")
var ErrSymbolNotFound = errors.New("symbol not found")

type Store interface {
	GetSyncState(context.Context, market.SyncProfile) (market.SyncState, error)
	ListActiveInstruments(context.Context) ([]market.Instrument, error)
	ListLatestCandlesByInterval(context.Context, int64, string, int) ([]market.Candle, error)
}

type SymbolRequest struct {
	Symbol   string
	Criteria []CriterionConfig
}
type SymbolResult struct {
	Symbol      string
	Matched     bool
	Evaluations []Evaluation
}
type SearchRequest struct{ Criteria []CriterionConfig }
type SearchItem struct {
	Symbol        string
	Matched       bool
	Evaluations   []Evaluation
	orderingScore *float64
}
type SearchResult struct {
	MatchedCount          int
	AnalyzedCount         int
	InsufficientDataCount int
	Items                 []SearchItem
}

type Service struct {
	store     Store
	factories map[string]Factory
}

// NewService validates and registers the explicitly composed criterion factories.
func NewService(store Store, factories ...Factory) (*Service, error) {
	if len(factories) == 0 {
		return nil, fmt.Errorf("criterion factories: %w", ErrInvalidArgument)
	}
	registry := make(map[string]Factory, len(factories))
	for _, factory := range factories {
		if factory == nil || factory.Name() == "" {
			return nil, fmt.Errorf("criterion factory: %w", ErrInvalidArgument)
		}
		if _, exists := registry[factory.Name()]; exists {
			return nil, fmt.Errorf("duplicate criterion factory %q: %w", factory.Name(), ErrInvalidArgument)
		}
		registry[factory.Name()] = factory
	}
	return &Service{store: store, factories: registry}, nil
}

func (service *Service) AnalyzeSymbol(ctx context.Context, request SymbolRequest) (SymbolResult, error) {
	criteria, requirements, err := service.prepare(request.Criteria)
	if err != nil {
		return SymbolResult{}, err
	}
	if err := service.requireMarketData(ctx, requirements); err != nil {
		return SymbolResult{}, err
	}
	instruments, err := service.store.ListActiveInstruments(ctx)
	if err != nil {
		return SymbolResult{}, fmt.Errorf("list active instruments: %w", err)
	}
	for _, instrument := range instruments {
		if instrument.Symbol == request.Symbol {
			result, err := service.evaluate(ctx, instrument, criteria, requirements)
			if err != nil {
				return SymbolResult{}, fmt.Errorf("analyze %s: %w", request.Symbol, err)
			}
			return result, nil
		}
	}
	return SymbolResult{}, ErrSymbolNotFound
}

func (service *Service) Search(ctx context.Context, request SearchRequest) (SearchResult, error) {
	criteria, requirements, err := service.prepare(request.Criteria)
	if err != nil {
		return SearchResult{}, err
	}
	if err := service.requireMarketData(ctx, requirements); err != nil {
		return SearchResult{}, err
	}
	instruments, err := service.store.ListActiveInstruments(ctx)
	if err != nil {
		return SearchResult{}, fmt.Errorf("list active instruments: %w", err)
	}
	result := SearchResult{Items: make([]SearchItem, 0)}
	for _, instrument := range instruments {
		item, evaluateErr := service.evaluate(ctx, instrument, criteria, requirements)
		var insufficient *InsufficientHistoryError
		if errors.As(evaluateErr, &insufficient) {
			result.InsufficientDataCount++
			continue
		}
		if evaluateErr != nil {
			return SearchResult{}, fmt.Errorf("analyze %s: %w", instrument.Symbol, evaluateErr)
		}
		result.AnalyzedCount++
		if item.Matched {
			result.Items = append(result.Items, SearchItem{Symbol: item.Symbol, Matched: true, Evaluations: item.Evaluations, orderingScore: firstScore(item.Evaluations)})
		}
	}
	sort.Slice(result.Items, func(i, j int) bool {
		a, b := result.Items[i].orderingScore, result.Items[j].orderingScore
		if a != nil && b != nil && *a != *b {
			return *a > *b
		}
		return result.Items[i].Symbol < result.Items[j].Symbol
	})
	result.MatchedCount = len(result.Items)
	return result, nil
}

func (service *Service) prepare(configs []CriterionConfig) ([]Criterion, map[Unit]int, error) {
	if len(configs) == 0 {
		return nil, nil, ErrInvalidArgument
	}
	criteria := make([]Criterion, 0, len(configs))
	requirements := map[Unit]int{}
	selected := map[string]bool{}
	for _, config := range configs {
		factory, ok := service.factories[config.Name]
		if !ok || selected[config.Name] {
			return nil, nil, ErrInvalidArgument
		}
		criterion, err := factory.Build(config.Parameters)
		if err != nil {
			return nil, nil, fmt.Errorf("build criterion %s: %w", config.Name, err)
		}
		if criterion == nil || criterion.Name() != config.Name || len(criterion.Requirements()) == 0 {
			return nil, nil, ErrInvalidArgument
		}
		selected[config.Name] = true
		criteria = append(criteria, criterion)
		for _, requirement := range criterion.Requirements() {
			if (requirement.Unit != UnitDays && requirement.Unit != UnitHours) || requirement.Count < 1 {
				return nil, nil, ErrInvalidArgument
			}
			if requirement.Count > requirements[requirement.Unit] {
				requirements[requirement.Unit] = requirement.Count
			}
		}
	}
	return criteria, requirements, nil
}

func (service *Service) evaluate(ctx context.Context, instrument market.Instrument, criteria []Criterion, requirements map[Unit]int) (SymbolResult, error) {
	data := make(map[Unit][]market.Candle, len(requirements))
	for unit, count := range requirements {
		candles, err := service.store.ListLatestCandlesByInterval(ctx, instrument.ID, unit.Interval(), count)
		if err != nil {
			return SymbolResult{}, fmt.Errorf("list latest candles for %s: %w", instrument.Symbol, err)
		}
		data[unit] = candles
	}
	result := SymbolResult{Symbol: instrument.Symbol, Matched: true, Evaluations: make([]Evaluation, 0, len(criteria))}
	for _, criterion := range criteria {
		evaluation, err := criterion.Evaluate(ctx, data)
		if err != nil {
			var insufficient *InsufficientHistoryError
			if errors.As(err, &insufficient) && insufficient.Criterion == "" {
				insufficient.Criterion = criterion.Name()
			}
			return SymbolResult{}, fmt.Errorf("evaluate criterion %s: %w", criterion.Name(), err)
		}
		evaluation.Name = criterion.Name()
		result.Evaluations = append(result.Evaluations, evaluation)
		result.Matched = result.Matched && evaluation.Matched
	}
	return result, nil
}

func firstScore(evaluations []Evaluation) *float64 {
	if len(evaluations) == 0 {
		return nil
	}
	return evaluations[0].OrderingScore
}

var marketProfile = market.SyncProfile{Exchange: "binance", Market: "spot", QuoteAsset: "USDT", Interval: "1d", TimeZone: "UTC"}
var hourlyMarketProfile = market.SyncProfile{Exchange: "binance", Market: "spot", QuoteAsset: "USDT", Interval: "1h", TimeZone: "UTC"}

func (service *Service) requireMarketData(ctx context.Context, requirements map[Unit]int) error {
	for unit := range requirements {
		profile := marketProfile
		if unit == UnitHours {
			profile = hourlyMarketProfile
		}
		state, err := service.store.GetSyncState(ctx, profile)
		if err != nil {
			return fmt.Errorf("get market synchronization state: %w", err)
		}
		if state.LastSucceededAt == nil {
			return ErrMarketDataUnavailable
		}
	}
	return nil
}
