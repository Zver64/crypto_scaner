package analysis

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"crypto-scanner/internal/closedindicator"
	"crypto-scanner/internal/market"
)

var ErrMarketDataUnavailable = errors.New("market data unavailable")

// ErrSymbolNotFound reports an unknown or inactive symbol. It wraps
// market.ErrInstrumentNotFound, so callers may match either sentinel.
var ErrSymbolNotFound = fmt.Errorf("analysis symbol lookup: %w", market.ErrInstrumentNotFound)

type Store interface {
	GetSyncState(context.Context, market.SyncProfile) (market.SyncState, error)
	// SelectActiveInstruments applies all constraints before ordering and limiting.
	SelectActiveInstruments(context.Context, Selection) ([]market.Instrument, error)
	// ListLatestCandles returns up to limit latest candles per instrument in
	// chronological order.
	ListLatestCandles(context.Context, []int64, market.CandleInterval, int) (map[int64][]market.Candle, error)
	ListHourlyPrices(context.Context, []int64, time.Time, time.Time) ([]market.HourlyPrice, error)
}

// ClosedIndicators supplies indicator values at the latest closed candle.
type ClosedIndicators interface {
	// Latest never fails: unavailable values have no outputs.
	Latest(context.Context, []int64) map[int64][]closedindicator.Value
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
type SearchRequest struct {
	Criteria []CriterionConfig
	Limit    int
	Sort     *SearchSort
}
type SearchSort struct {
	Field     string
	Direction string
}
type SearchItem struct {
	PriceHistory     []*float64
	Symbol           string
	Matched          bool
	Evaluations      []Evaluation
	ClosedIndicators []closedindicator.Value
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
	store                Store
	closed               ClosedIndicators
	factories            map[string]Factory
	selectionFilters     map[string]SelectionFilter
	selectionSortFilters map[string]SelectionFilter
}

type criterionInstance struct {
	Criterion
	key   string
	label string
}

// NewService validates and registers the explicitly composed criterion
// factories. closed attaches closed indicator values to search items; it may
// be nil.
func NewService(store Store, closed ClosedIndicators, factories ...Factory) (*Service, error) {
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
	selectionFilters := selectionFilterModules()
	selectionRegistry := make(map[string]SelectionFilter, len(selectionFilters))
	sortRegistry := make(map[string]SelectionFilter, len(selectionFilters))
	for _, filter := range selectionFilters {
		if filter == nil || filter.Name() == "" || selectionRegistry[filter.Name()] != nil {
			return nil, fmt.Errorf("selection filter: %w", ErrInvalidArgument)
		}
		selectionRegistry[filter.Name()] = filter
		if field := filter.SortField(); field != "" {
			if sortRegistry[field] != nil {
				return nil, fmt.Errorf("selection sort filter: %w", ErrInvalidArgument)
			}
			sortRegistry[field] = filter
		}
	}
	return &Service{store: store, closed: closed, factories: registry, selectionFilters: selectionRegistry, selectionSortFilters: sortRegistry}, nil
}

func (service *Service) AnalyzeSymbol(ctx context.Context, request SymbolRequest) (SymbolResult, error) {
	criteria, requirements, err := service.prepare(request.Criteria)
	if err != nil {
		return SymbolResult{}, err
	}
	if err := service.requireMarketData(ctx, requirements); err != nil {
		return SymbolResult{}, err
	}
	// Direct symbol analysis intentionally does not activate market-wide backend
	// defaults such as stablecoin exclusion.
	instruments, err := service.store.SelectActiveInstruments(ctx, Selection{Symbol: request.Symbol})
	if err != nil {
		return SymbolResult{}, fmt.Errorf("select active instrument: %w", err)
	}
	for _, instrument := range instruments {
		if instrument.Symbol == request.Symbol {
			result, err := service.evaluate(ctx, instrument, criteria)
			if err != nil {
				return SymbolResult{}, fmt.Errorf("analyze %s: %w", request.Symbol, err)
			}
			return result, nil
		}
	}
	return SymbolResult{}, ErrSymbolNotFound
}

func (service *Service) Search(ctx context.Context, request SearchRequest) (SearchResult, error) {
	return service.search(ctx, request, nil, false)
}

// SearchSymbols runs the existing market-analysis pipeline over only the
// supplied symbols. Selection constraints, ordering, and limits still execute
// in PostgreSQL before criterion evaluation.
func (service *Service) SearchSymbols(ctx context.Context, request SearchRequest, symbols []string) (SearchResult, error) {
	return service.search(ctx, request, symbols, true)
}

func (service *Service) search(ctx context.Context, request SearchRequest, symbols []string, restrictSymbols bool) (SearchResult, error) {
	window := market.SevenDayWindow(time.Now())
	criteria, requirements, err := service.prepare(request.Criteria)
	if err != nil {
		return SearchResult{}, err
	}
	selection, err := service.selection(request, criteria)
	if err != nil {
		return SearchResult{}, err
	}
	if restrictSymbols {
		selection.Symbols = append([]string(nil), symbols...)
		// Favorites are not a Top-N search: all selected active instruments must
		// reach the analysis pipeline regardless of the caller's table limit.
		selection.Limit = 0
	}
	if restrictSymbols && len(symbols) == 0 {
		return SearchResult{PriceHistoryWindow: window, Items: []SearchItem{}, Unresolved: []UnresolvedItem{}}, nil
	}
	if err := service.requireMarketData(ctx, requirements); err != nil {
		return SearchResult{}, err
	}
	instruments, err := service.store.SelectActiveInstruments(ctx, selection)
	if err != nil {
		return SearchResult{}, fmt.Errorf("select active instruments: %w", err)
	}
	result := SearchResult{PriceHistoryWindow: window, Items: make([]SearchItem, 0), Unresolved: make([]UnresolvedItem, 0)}
	candidates := append([]market.Instrument(nil), instruments...)
	candleData := make(map[int64]map[Unit][]market.Candle, len(candidates))
	for _, instrument := range candidates {
		candleData[instrument.ID] = map[Unit][]market.Candle{}
	}
	results := make(map[int64]SymbolResult, len(candidates))
	for _, criterion := range criteria {
		if len(candidates) == 0 {
			break
		}
		next := make([]market.Instrument, 0, len(candidates))
		if err := service.loadCandleData(ctx, candidates, criterion.Requirements(), candleData); err != nil {
			return SearchResult{}, err
		}
		for _, instrument := range candidates {
			item, evaluateErr := service.evaluateCriterionWithData(ctx, instrument, criterion, candleData[instrument.ID], false)
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
	closed := map[int64][]closedindicator.Value{}
	if service.closed != nil && len(candidates) > 0 {
		ids := make([]int64, len(candidates))
		for index, instrument := range candidates {
			ids[index] = instrument.ID
		}
		closed = service.closed.Latest(ctx, ids)
	}
	for _, instrument := range candidates {
		item := results[instrument.ID]
		result.Items = append(result.Items, SearchItem{Symbol: item.Symbol, Matched: true, Evaluations: item.Evaluations, PriceHistory: histories[instrument.ID], ClosedIndicators: closed[instrument.ID]})
	}
	result.MatchedCount = len(result.Items)
	return result, nil
}

func (service *Service) loadCandleData(ctx context.Context, instruments []market.Instrument, requirements []CandleRequirement, data map[int64]map[Unit][]market.Candle) error {
	for _, requirement := range requirements {
		pending := make([]market.Instrument, 0, len(instruments))
		ids := make([]int64, 0, len(instruments))
		for _, instrument := range instruments {
			if len(data[instrument.ID][requirement.Unit]) >= requirement.Count {
				continue
			}
			pending = append(pending, instrument)
			ids = append(ids, instrument.ID)
		}
		if len(ids) == 0 {
			continue
		}
		candles, err := service.store.ListLatestCandles(ctx, ids, requirement.Unit.Interval(), requirement.Count)
		if err != nil {
			return fmt.Errorf("list latest %s candles: %w", requirement.Unit, err)
		}
		for _, instrument := range pending {
			data[instrument.ID][requirement.Unit] = candles[instrument.ID]
		}
	}
	return nil
}

func (service *Service) selection(request SearchRequest, criteria []criterionInstance) (Selection, error) {
	if request.Limit < 0 || request.Limit > 100 {
		return Selection{}, ErrInvalidArgument
	}
	selection := Selection{Limit: request.Limit}
	for _, filter := range service.selectionFilters {
		if filter.BackendDefault() {
			if err := filter.Apply(nil, &selection); err != nil {
				return Selection{}, err
			}
		}
	}
	for _, criterion := range criteria {
		if filter := service.selectionFilters[criterion.Name()]; filter != nil {
			if err := filter.Apply(criterion.Criterion, &selection); err != nil {
				return Selection{}, err
			}
		}
	}
	if request.Sort == nil {
		return selection, nil
	}
	if request.Sort.Direction != "asc" && request.Sort.Direction != "desc" {
		return Selection{}, ErrInvalidArgument
	}
	filter := service.selectionSortFilters[request.Sort.Field]
	if filter == nil {
		return Selection{}, ErrInvalidArgument
	}
	if err := filter.ApplySort(request.Sort.Direction, &selection); err != nil {
		return Selection{}, err
	}
	return selection, nil
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

func (service *Service) evaluate(ctx context.Context, instrument market.Instrument, criteria []criterionInstance) (SymbolResult, error) {
	result := SymbolResult{Symbol: instrument.Symbol, Matched: true, Evaluations: make([]Evaluation, 0, len(criteria))}
	data := make(map[Unit][]market.Candle)
	for _, criterion := range criteria {
		item, err := service.evaluateCriterionWithData(ctx, instrument, criterion, data, true)
		if err != nil {
			return SymbolResult{}, err
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

func (service *Service) evaluateCriterionWithData(ctx context.Context, instrument market.Instrument, criterion criterionInstance, data map[Unit][]market.Candle, loadMissing bool) (SymbolResult, error) {
	for _, requirement := range criterion.Requirements() {
		if existing, ok := data[requirement.Unit]; ok && len(existing) >= requirement.Count {
			continue
		}
		// Search preloads each requirement in batches, including empty/short
		// results. Never turn insufficient history into per-instrument retries.
		if !loadMissing {
			continue
		}
		candles, err := service.store.ListLatestCandles(ctx, []int64{instrument.ID}, requirement.Unit.Interval(), requirement.Count)
		if err != nil {
			return SymbolResult{}, fmt.Errorf("list latest candles for %s: %w", instrument.Symbol, err)
		}
		data[requirement.Unit] = candles[instrument.ID]
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

func (service *Service) requireMarketData(ctx context.Context, requirements map[Unit]int) error {
	for unit := range requirements {
		state, err := service.store.GetSyncState(ctx, market.BinanceSpotSyncProfile(unit.Interval()))
		if err != nil {
			return fmt.Errorf("get market synchronization state: %w", err)
		}
		if state.LastSucceededAt == nil {
			return ErrMarketDataUnavailable
		}
	}
	return nil
}
