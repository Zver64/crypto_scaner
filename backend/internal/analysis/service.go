package analysis

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"crypto-scanner/internal/market"
)

var ErrMarketDataUnavailable = errors.New("market data unavailable")
var ErrMarketCapUnavailable = errors.New("market cap data unavailable")
var ErrSymbolNotFound = errors.New("symbol not found")

type Store interface {
	GetSyncState(context.Context, market.SyncProfile) (market.SyncState, error)
	ListActiveInstruments(context.Context) ([]market.Instrument, error)
	ListLatestCandlesByInterval(context.Context, int64, string, int) ([]market.Candle, error)
	ListHourlyPrices(context.Context, []int64, time.Time, time.Time) ([]market.HourlyPrice, error)
}

type SymbolRequest struct {
	Symbol   string
	Criteria []CriterionConfig
}
type SymbolResult struct {
	Symbol      string
	Matched     bool
	Evaluations []Evaluation
	Warnings    []Warning
}
type SearchRequest struct{ Criteria []CriterionConfig }
type SearchItem struct {
	PriceHistory  []*float64
	Symbol        string
	Matched       bool
	Evaluations   []Evaluation
	orderingScore *float64
}
type SearchResult struct {
	PriceHistoryWindow    market.PriceHistoryWindow
	MatchedCount          int
	AnalyzedCount         int
	InsufficientDataCount int
	Items                 []SearchItem
	Unresolved            []UnresolvedItem
	Warnings              []Warning
}
type UnresolvedItem struct{ Symbol, Code, Message string }

type Service struct {
	store     Store
	factories map[string]Factory
}

type criterionInstance struct {
	Criterion
	key   string
	label string
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
	window := market.SevenDayWindow(time.Now())
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
	result := SearchResult{PriceHistoryWindow: window, Items: make([]SearchItem, 0), Unresolved: make([]UnresolvedItem, 0)}
	candidates := append([]market.Instrument(nil), instruments...)
	results := make(map[int64]SymbolResult, len(candidates))
	for _, criterion := range criteria {
		if len(candidates) == 0 {
			break
		}
		next := make([]market.Instrument, 0, len(candidates))
		warnings, prepareErr := criterion.Prepare(ctx, candidates)
		if prepareErr != nil {
			return SearchResult{}, fmt.Errorf("prepare criterion %s: %w", criterion.Name(), prepareErr)
		}
		result.Warnings = append(result.Warnings, warnings...)
		for _, instrument := range candidates {
			item, evaluateErr := service.evaluateCriterion(ctx, instrument, criterion)
			var insufficient *InsufficientHistoryError
			if errors.As(evaluateErr, &insufficient) {
				result.InsufficientDataCount++
				continue
			}
			var unresolved *UnresolvedError
			if errors.As(evaluateErr, &unresolved) {
				result.Unresolved = append(result.Unresolved, UnresolvedItem{Symbol: instrument.Symbol, Code: unresolved.Code, Message: unresolved.Message})
				continue
			}
			if evaluateErr != nil {
				return SearchResult{}, fmt.Errorf("analyze %s: %w", instrument.Symbol, evaluateErr)
			}
			previous := results[instrument.ID]
			previous.Symbol = instrument.Symbol
			previous.Evaluations = append(previous.Evaluations, item.Evaluations...)
			previous.Matched = item.Matched
			results[instrument.ID] = previous
			if item.Matched {
				next = append(next, instrument)
			}
		}
		candidates = next
	}
	result.AnalyzedCount = len(instruments) - result.InsufficientDataCount - len(result.Unresolved)
	histories, err := service.priceHistories(ctx, candidates, window)
	if err != nil {
		return SearchResult{}, err
	}
	for _, instrument := range candidates {
		item := results[instrument.ID]
		result.Items = append(result.Items, SearchItem{Symbol: item.Symbol, Matched: true, Evaluations: item.Evaluations, PriceHistory: histories[instrument.ID], orderingScore: firstScore(item.Evaluations)})
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

func (service *Service) prepare(configs []CriterionConfig) ([]criterionInstance, map[Unit]int, error) {
	if len(configs) == 0 {
		return nil, nil, ErrInvalidArgument
	}
	criteria := make([]criterionInstance, 0, len(configs))
	requirements := map[Unit]int{}
	selectedKeys := map[string]bool{}
	for _, config := range configs {
		if strings.TrimSpace(config.Key) == "" || strings.TrimSpace(config.Label) == "" || selectedKeys[config.Key] {
			return nil, nil, ErrInvalidArgument
		}
		factory, ok := service.factories[config.Name]
		if !ok {
			return nil, nil, ErrInvalidArgument
		}
		criterion, err := factory.Build(config.Parameters)
		if err != nil {
			return nil, nil, fmt.Errorf("build criterion %s: %w", config.Name, err)
		}
		if criterion == nil || criterion.Name() != config.Name {
			return nil, nil, ErrInvalidArgument
		}
		selectedKeys[config.Key] = true
		criteria = append(criteria, criterionInstance{Criterion: criterion, key: config.Key, label: config.Label})
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

func (service *Service) evaluate(ctx context.Context, instrument market.Instrument, criteria []criterionInstance, requirements map[Unit]int) (SymbolResult, error) {
	_ = requirements
	result := SymbolResult{Symbol: instrument.Symbol, Matched: true, Evaluations: make([]Evaluation, 0, len(criteria))}
	data := make(map[Unit][]market.Candle)
	for _, criterion := range criteria {
		warnings, err := criterion.Prepare(ctx, []market.Instrument{instrument})
		if err != nil {
			return SymbolResult{}, fmt.Errorf("prepare criterion %s: %w", criterion.Name(), err)
		}
		result.Warnings = append(result.Warnings, warnings...)
		item, err := service.evaluateCriterionWithData(ctx, instrument, criterion, data)
		if err != nil {
			var insufficient *InsufficientHistoryError
			if errors.As(err, &insufficient) && insufficient.Criterion == "" {
				insufficient.Criterion = criterion.Name()
			}
			return SymbolResult{}, fmt.Errorf("evaluate criterion %s: %w", criterion.Name(), err)
		}
		result.Evaluations = append(result.Evaluations, item.Evaluations...)
		result.Matched = result.Matched && item.Matched
		if !item.Matched {
			break
		}
	}
	return result, nil
}

// UnresolvedError means a non-candle criterion could not obtain a value. It is
// intentionally distinct from a failed comparison: unknown values are never zero.
type UnresolvedError struct{ Code, Message string }

func (e *UnresolvedError) Error() string { return e.Message }

func (service *Service) evaluateCriterion(ctx context.Context, instrument market.Instrument, criterion criterionInstance) (SymbolResult, error) {
	data := make(map[Unit][]market.Candle)
	return service.evaluateCriterionWithData(ctx, instrument, criterion, data)
}
func (service *Service) evaluateCriterionWithData(ctx context.Context, instrument market.Instrument, criterion criterionInstance, data map[Unit][]market.Candle) (SymbolResult, error) {
	for _, requirement := range criterion.Requirements() {
		if existing, ok := data[requirement.Unit]; ok && len(existing) >= requirement.Count {
			continue
		}
		candles, err := service.store.ListLatestCandlesByInterval(ctx, instrument.ID, requirement.Unit.Interval(), requirement.Count)
		if err != nil {
			return SymbolResult{}, fmt.Errorf("list latest candles for %s: %w", instrument.Symbol, err)
		}
		data[requirement.Unit] = candles
	}
	evaluation, err := criterion.Evaluate(ctx, Input{Instrument: instrument, Candles: data})
	if err != nil {
		var insufficient *InsufficientHistoryError
		if errors.As(err, &insufficient) && insufficient.Criterion == "" {
			insufficient.Criterion = criterion.Name()
		}
		return SymbolResult{}, fmt.Errorf("evaluate criterion %s: %w", criterion.Name(), err)
	}
	evaluation.Name = criterion.Name()
	evaluation.Key = criterion.key
	evaluation.Label = criterion.label
	return SymbolResult{Symbol: instrument.Symbol, Matched: evaluation.Matched, Evaluations: []Evaluation{evaluation}}, nil
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
