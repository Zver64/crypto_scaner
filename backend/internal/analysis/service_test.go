package analysis_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"crypto-scanner/internal/analysis"
	marketcapcriterion "crypto-scanner/internal/analysis/criteria/marketcap"
	"crypto-scanner/internal/analysis/criteria/volatility"
	"crypto-scanner/internal/market"
)

func TestServiceCombinesCriteriaAndLoadsMergedRequirementsOnce(t *testing.T) {
	store := &storeStub{instruments: []market.Instrument{{ID: 1, Symbol: "BTCUSDT"}}, candles: map[string][]market.Candle{"1d": {testCandle(1), testCandle(2)}, "1h": {testCandle(1)}}, failRepeatedLoad: true}
	service, err := analysis.NewService(store, nil, volatility.New(), fakeFactory{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.AnalyzeSymbol(context.Background(), analysis.SymbolRequest{Symbol: "BTCUSDT", Criteria: []analysis.CriterionConfig{{Key: "volatility", Name: "volatility", Label: "Volatility", Parameters: map[string]any{"unit": "days", "period": float64(2), "percentile": float64(50), "minimum_range_percent": float64(0)}}, {Key: "fake", Name: "fake", Label: "Fake", Parameters: map[string]any{}}}})
	if err != nil || result.Matched || len(result.Evaluations) != 2 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestAnalyzeSymbolShortCircuitsLaterCriteria(t *testing.T) {
	store := &storeStub{
		instruments:         []market.Instrument{{ID: 1, Symbol: "DROP"}},
		candlesByInstrument: map[int64]map[string][]market.Candle{1: {"1d": {testCandle(1)}}},
	}
	second := &trackingFactory{}
	service, err := analysis.NewService(store, nil, firstFactory{}, second)
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.AnalyzeSymbol(context.Background(), analysis.SymbolRequest{Symbol: "DROP", Criteria: []analysis.CriterionConfig{
		{Key: "first", Name: "first", Label: "First", Parameters: map[string]any{}},
		{Key: "second", Name: "second", Label: "Second", Parameters: map[string]any{}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Matched || len(result.Evaluations) != 1 || second.evaluated != 0 {
		t.Fatalf("result=%+v evaluated=%d", result, second.evaluated)
	}
	if store.loads["1d"] != 1 || store.loads["1h"] != 0 {
		t.Fatalf("candle loads = %v", store.loads)
	}
}

func TestServiceCorrelatesRepeatedCriterionTypesByInstanceIdentity(t *testing.T) {
	store := &storeStub{
		instruments: []market.Instrument{{ID: 1, Symbol: "BTCUSDT"}},
		candles: map[string][]market.Candle{
			"1d": {testCandle(5)},
			"1h": {testCandle(2)},
		},
	}
	service, err := analysis.NewService(store, nil, volatility.New())
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.AnalyzeSymbol(context.Background(), analysis.SymbolRequest{
		Symbol: "BTCUSDT",
		Criteria: []analysis.CriterionConfig{
			{Key: "daily_volatility", Name: "volatility", Label: "Daily Volatility", Parameters: map[string]any{"unit": "days", "period": float64(1), "percentile": float64(50), "minimum_range_percent": float64(0)}},
			{Key: "hourly_volatility", Name: "volatility", Label: "Hourly Volatility", Parameters: map[string]any{"unit": "hours", "period": float64(1), "percentile": float64(50), "minimum_range_percent": float64(0)}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Evaluations) != 2 {
		t.Fatalf("evaluations = %+v", result.Evaluations)
	}
	if first := result.Evaluations[0]; first.Key != "daily_volatility" || first.Name != "volatility" || first.Label != "Daily Volatility" {
		t.Fatalf("first evaluation = %+v", first)
	}
	if second := result.Evaluations[1]; second.Key != "hourly_volatility" || second.Name != "volatility" || second.Label != "Hourly Volatility" {
		t.Fatalf("second evaluation = %+v", second)
	}
}

func TestSearchEvaluatesRepeatedCriterionTypesOnlyForSurvivors(t *testing.T) {
	store := &storeStub{
		instruments: []market.Instrument{{ID: 1, Symbol: "DROP"}, {ID: 2, Symbol: "KEEP"}},
		candlesByInstrument: map[int64]map[string][]market.Candle{
			1: {"1d": {testCandle(2)}},
			2: {"1d": {testCandle(6)}, "1h": {testCandle(3)}},
		},
	}
	service, err := analysis.NewService(store, nil, volatility.New())
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.Search(context.Background(), analysis.SearchRequest{Criteria: []analysis.CriterionConfig{
		{Key: "daily_volatility", Name: "volatility", Label: "Daily Volatility", Parameters: map[string]any{"unit": "days", "period": float64(1), "percentile": float64(50), "minimum_range_percent": float64(5)}},
		{Key: "hourly_volatility", Name: "volatility", Label: "Hourly Volatility", Parameters: map[string]any{"unit": "hours", "period": float64(1), "percentile": float64(50), "minimum_range_percent": float64(0)}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if result.MatchedCount != 1 || len(result.Items) != 1 || result.Items[0].Symbol != "KEEP" {
		t.Fatalf("result = %+v", result)
	}
	if store.loads["1h"] != 1 {
		t.Fatalf("hourly candle loads = %d", store.loads["1h"])
	}
}

func TestServiceSearchCombinesCriteriaAndPreservesStoreOrder(t *testing.T) {
	store := &storeStub{
		instruments: []market.Instrument{
			{ID: 1, Symbol: "ZZZUSDT"}, {ID: 2, Symbol: "AAAUSDT"}, {ID: 3, Symbol: "LOWUSDT"},
			{ID: 4, Symbol: "FAILUSDT"}, {ID: 5, Symbol: "NEWUSDT"}, {ID: 6, Symbol: "EMPTYUSDT"},
		},
		candlesByInstrument: map[int64]map[string][]market.Candle{
			1: {"1d": {testCandle(5), testCandle(5)}, "1h": {testCandle(6)}},
			2: {"1d": {testCandle(5), testCandle(5)}, "1h": {testCandle(6)}},
			3: {"1d": {testCandle(2), testCandle(2)}, "1h": {testCandle(6)}},
			4: {"1d": {testCandle(6), testCandle(6)}, "1h": {testCandle(2)}},
			5: {"1d": {testCandle(5)}, "1h": {testCandle(6)}},
			6: {"1d": {}, "1h": {testCandle(6)}},
		},
	}
	service, err := analysis.NewService(store, nil, volatility.New(), hourlyMatchFactory{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Search(context.Background(), analysis.SearchRequest{Criteria: []analysis.CriterionConfig{
		{Key: "volatility", Name: "volatility", Label: "Volatility", Parameters: map[string]any{"unit": "days", "period": float64(2), "percentile": float64(50), "minimum_range_percent": float64(3)}},
		{Key: "hourly-match", Name: "hourly-match", Label: "Hourly Match", Parameters: map[string]any{}},
	}})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if result.MatchedCount != 2 || result.AnalyzedCount != 4 || result.InsufficientDataCount != 2 {
		t.Fatalf("Search() counts = %+v", result)
	}
	if len(result.Items) != 2 || result.Items[0].Symbol != "ZZZUSDT" || result.Items[1].Symbol != "AAAUSDT" {
		t.Fatalf("Search() items = %+v", result.Items)
	}
	for _, item := range result.Items {
		if !item.Matched || len(item.Evaluations) != 2 || !item.Evaluations[0].Matched || !item.Evaluations[1].Matched {
			t.Fatalf("Search() item = %+v", item)
		}
	}
}
func TestSearchDoesNotPrepareMarketCapsOnRequest(t *testing.T) {
	cap := 100.0
	store := &storeStub{instruments: []market.Instrument{{ID: 1, Symbol: "BTCUSDT", BaseAsset: "BTC", MarketCapUSD: &cap}}}
	service, err := analysis.NewService(store, nil, marketcapcriterion.New())
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Search(context.Background(), analysis.SearchRequest{Criteria: []analysis.CriterionConfig{{Key: "market_cap", Name: "market_cap", Label: "Market Cap", Parameters: map[string]any{"min_market_cap_usd": float64(100)}}}})
	if err != nil || result.MatchedCount != 1 || result.Items[0].Evaluations[0].Metrics["market_cap_usd"] != 100 || store.activeListCalls != 0 {
		t.Fatalf("result=%+v err=%v active lists=%d", result, err, store.activeListCalls)
	}
}

func TestSearchUsesBackendMarketCapSortAndLimit(t *testing.T) {
	store := &storeStub{
		rankedInstruments: []market.Instrument{
			{ID: 2, Symbol: "BTCUSDT"},
			{ID: 1, Symbol: "ETHUSDT"},
		},
	}
	service, err := analysis.NewService(store, nil, marketCapTestFactory{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Search(context.Background(), analysis.SearchRequest{
		Criteria: []analysis.CriterionConfig{{Key: "market_cap", Name: "market_cap", Label: "Market Cap", Parameters: map[string]any{}}},
		Limit:    1,
		Sort:     &analysis.SearchSort{Field: "market_cap_usd", Direction: "desc"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || result.Items[0].Symbol != "BTCUSDT" {
		t.Fatalf("items = %+v", result.Items)
	}
	if store.selectionLimit != 1 || store.selectionDirection != "desc" || store.activeListCalls != 0 {
		t.Fatalf("selection limit=%d direction=%q active calls=%d", store.selectionLimit, store.selectionDirection, store.activeListCalls)
	}
}

func TestSearchSymbolsReturnsValidatedEmptyResultWithoutMarketReadiness(t *testing.T) {
	store := &storeStub{syncUnavailable: true}
	service, err := analysis.NewService(store, nil, fakeFactory{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.SearchSymbols(context.Background(), analysis.SearchRequest{Criteria: []analysis.CriterionConfig{{Key: "fake", Name: "fake", Label: "Fake"}}}, nil)
	if err != nil || len(result.Items) != 0 || result.PriceHistoryWindow.From.IsZero() {
		t.Fatalf("empty favorite result = %+v, %v", result, err)
	}
	if store.reads != 0 {
		t.Fatalf("empty favorite analysis performed %d store reads", store.reads)
	}
}

func TestSearchSymbolsRestrictsPipelineAndIgnoresMarketLimit(t *testing.T) {
	store := &storeStub{instruments: []market.Instrument{
		{ID: 1, Symbol: "BTCUSDT"}, {ID: 2, Symbol: "ETHUSDT"}, {ID: 3, Symbol: "SOLUSDT"},
	}}
	service, err := analysis.NewService(store, nil, marketCapTestFactory{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.SearchSymbols(context.Background(), analysis.SearchRequest{
		Criteria: []analysis.CriterionConfig{{Key: "market_cap", Name: "market_cap", Label: "Market Cap", Parameters: map[string]any{}}},
		Limit:    1,
	}, []string{"BTCUSDT", "SOLUSDT"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 2 || result.Items[0].Symbol != "BTCUSDT" || result.Items[1].Symbol != "SOLUSDT" {
		t.Fatalf("favorite analysis items = %+v", result.Items)
	}
	if store.selectionLimit != 0 || len(store.selectionSymbols) != 2 || store.activeListCalls != 0 {
		t.Fatalf("selection limit=%d symbols=%v active calls=%d", store.selectionLimit, store.selectionSymbols, store.activeListCalls)
	}
}

func TestSearchUsesBackendLimitWithoutChangingStoreOrder(t *testing.T) {
	store := &storeStub{
		instruments: []market.Instrument{
			{ID: 1, Symbol: "ZZZUSDT"},
			{ID: 2, Symbol: "AAAUSDT"},
		},
	}
	service, err := analysis.NewService(store, nil, marketCapTestFactory{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Search(context.Background(), analysis.SearchRequest{
		Criteria: []analysis.CriterionConfig{{Key: "market_cap", Name: "market_cap", Label: "Market Cap", Parameters: map[string]any{}}},
		Limit:    1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || result.Items[0].Symbol != "ZZZUSDT" {
		t.Fatalf("items = %+v", result.Items)
	}
	if store.selectionLimit != 1 || store.selectionDirection != "" || store.activeListCalls != 0 {
		t.Fatalf("selection limit=%d direction=%q active calls=%d", store.selectionLimit, store.selectionDirection, store.activeListCalls)
	}
}

func TestServiceRejectsInvalidSelectionBeforeReads(t *testing.T) {
	store := &storeStub{}
	service, _ := analysis.NewService(store, nil, volatility.New())
	for _, request := range []analysis.SearchRequest{
		{},
		{Criteria: []analysis.CriterionConfig{{Key: "missing", Name: "missing", Label: "Missing"}}},
		{Criteria: []analysis.CriterionConfig{{Key: "volatility", Name: "volatility", Label: "Volatility", Parameters: map[string]any{"unit": "days", "period": float64(1), "percentile": float64(50), "minimum_range_percent": float64(0)}}}, Limit: -1},
		{Criteria: []analysis.CriterionConfig{{Key: "volatility", Name: "volatility", Label: "Volatility", Parameters: map[string]any{"unit": "days", "period": float64(1), "percentile": float64(50), "minimum_range_percent": float64(0)}}}, Limit: 101},
		{Criteria: []analysis.CriterionConfig{{Key: "volatility", Name: "volatility", Label: "Volatility", Parameters: map[string]any{"unit": "days", "period": float64(1), "percentile": float64(50), "minimum_range_percent": float64(0)}}}, Sort: &analysis.SearchSort{Field: "market_cap_usd", Direction: "sideways"}},
		{Criteria: []analysis.CriterionConfig{{Key: "volatility", Name: "volatility", Label: "Volatility", Parameters: map[string]any{"unit": "days", "period": float64(1), "percentile": float64(50), "minimum_range_percent": float64(0)}}}, Sort: &analysis.SearchSort{Field: "price", Direction: "desc"}},
		{Criteria: []analysis.CriterionConfig{{Key: "volatility", Name: "volatility", Label: "Volatility", Parameters: map[string]any{"unit": "days", "period": float64(1), "percentile": float64(50), "minimum_range_percent": float64(0)}}}, Sort: &analysis.SearchSort{Field: "market_cap_usd", Direction: "desc"}},
	} {
		_, err := service.Search(context.Background(), request)
		if !errors.Is(err, analysis.ErrInvalidArgument) {
			t.Fatalf("err=%v", err)
		}
	}
	if store.reads != 0 {
		t.Fatalf("reads=%d", store.reads)
	}
}

func TestServiceRejectsInvalidCriterionInstanceIdentityBeforeReads(t *testing.T) {
	store := &storeStub{}
	service, _ := analysis.NewService(store, nil, volatility.New())
	validParameters := map[string]any{"unit": "days", "period": float64(1), "percentile": float64(50), "minimum_range_percent": float64(0)}
	tests := []struct {
		name     string
		criteria []analysis.CriterionConfig
	}{
		{name: "empty key", criteria: []analysis.CriterionConfig{{Name: "volatility", Label: "Daily Volatility", Parameters: validParameters}}},
		{name: "blank key", criteria: []analysis.CriterionConfig{{Key: " ", Name: "volatility", Label: "Daily Volatility", Parameters: validParameters}}},
		{name: "empty label", criteria: []analysis.CriterionConfig{{Key: "daily_volatility", Name: "volatility", Parameters: validParameters}}},
		{name: "blank label", criteria: []analysis.CriterionConfig{{Key: "daily_volatility", Name: "volatility", Label: " ", Parameters: validParameters}}},
		{name: "duplicate key", criteria: []analysis.CriterionConfig{
			{Key: "volatility", Name: "volatility", Label: "Daily Volatility", Parameters: validParameters},
			{Key: "volatility", Name: "volatility", Label: "Hourly Volatility", Parameters: validParameters},
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.Search(context.Background(), analysis.SearchRequest{Criteria: test.criteria})
			if !errors.Is(err, analysis.ErrInvalidArgument) {
				t.Fatalf("err = %v", err)
			}
		})
	}
	if store.reads != 0 {
		t.Fatalf("reads = %d", store.reads)
	}
}
func TestServiceRejectsDuplicateFactoryNames(t *testing.T) {
	_, err := analysis.NewService(&storeStub{}, nil, fakeFactory{}, fakeFactory{})
	if !errors.Is(err, analysis.ErrInvalidArgument) {
		t.Fatalf("err=%v", err)
	}
}
func TestSearchPreparesAndLoadsSecondCriterionOnlyForSurvivors(t *testing.T) {
	store := &storeStub{instruments: []market.Instrument{{ID: 1, Symbol: "DROP"}, {ID: 2, Symbol: "KEEP"}}, candlesByInstrument: map[int64]map[string][]market.Candle{1: {"1d": {testCandle(1)}}, 2: {"1d": {testCandle(2)}, "1h": {testCandle(1)}}}}
	second := &trackingFactory{}
	service, _ := analysis.NewService(store, nil, firstFactory{}, second)
	result, err := service.Search(context.Background(), analysis.SearchRequest{Criteria: []analysis.CriterionConfig{{Key: "first", Name: "first", Label: "First", Parameters: map[string]any{}}, {Key: "second", Name: "second", Label: "Second", Parameters: map[string]any{}}}})
	if err != nil || result.MatchedCount != 1 || second.evaluated != 1 {
		t.Fatalf("result=%+v err=%v evaluated=%d", result, err, second.evaluated)
	}
	if store.loads["1h"] != 1 {
		t.Fatalf("second criterion candle loads=%v", store.loads)
	}
}
func TestSearchSkipsLaterCriteriaWhenNoCandidatesSurvive(t *testing.T) {
	store := &storeStub{instruments: []market.Instrument{{ID: 1, Symbol: "DROP"}}, candlesByInstrument: map[int64]map[string][]market.Candle{1: {"1d": {testCandle(1)}}}}
	second := &trackingFactory{}
	service, _ := analysis.NewService(store, nil, firstFactory{}, second)
	result, err := service.Search(context.Background(), analysis.SearchRequest{Criteria: []analysis.CriterionConfig{{Key: "first", Name: "first", Label: "First", Parameters: map[string]any{}}, {Key: "second", Name: "second", Label: "Second", Parameters: map[string]any{}}}})
	if err != nil || result.MatchedCount != 0 || second.evaluated != 0 {
		t.Fatalf("result=%+v err=%v evaluated=%d", result, err, second.evaluated)
	}
}

func TestSearchDoesNotCountUnresolvedInstrumentsAsAnalyzed(t *testing.T) {
	store := &storeStub{instruments: []market.Instrument{{ID: 1, Symbol: "UNKNOWN"}}}
	service, _ := analysis.NewService(store, nil, unresolvedFactory{})
	result, err := service.Search(context.Background(), analysis.SearchRequest{Criteria: []analysis.CriterionConfig{{Key: "unresolved", Name: "unresolved", Label: "Unresolved", Parameters: map[string]any{}}}})
	if err != nil || result.AnalyzedCount != 0 || len(result.Unresolved) != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

type firstFactory struct{}

func (firstFactory) Name() string                                     { return "first" }
func (firstFactory) Build(map[string]any) (analysis.Criterion, error) { return firstCriterion{}, nil }

type firstCriterion struct{}

func (firstCriterion) Name() string { return "first" }
func (firstCriterion) Requirements() []analysis.CandleRequirement {
	return []analysis.CandleRequirement{{Unit: analysis.UnitDays, Count: 1}}
}
func (firstCriterion) Evaluate(_ context.Context, input analysis.Input) (analysis.Evaluation, error) {
	return analysis.Evaluation{Matched: input.Instrument.ID == 2}, nil
}

type trackingFactory struct{ evaluated int }

func (t *trackingFactory) Name() string { return "second" }
func (t *trackingFactory) Build(map[string]any) (analysis.Criterion, error) {
	return &trackingCriterion{factory: t}, nil
}

type trackingCriterion struct{ factory *trackingFactory }

func (*trackingCriterion) Name() string { return "second" }
func (*trackingCriterion) Requirements() []analysis.CandleRequirement {
	return []analysis.CandleRequirement{{Unit: analysis.UnitHours, Count: 1}}
}
func (t *trackingCriterion) Evaluate(context.Context, analysis.Input) (analysis.Evaluation, error) {
	t.factory.evaluated++
	return analysis.Evaluation{Matched: true}, nil
}

type unresolvedFactory struct{}

func (unresolvedFactory) Name() string { return "unresolved" }
func (unresolvedFactory) Build(map[string]any) (analysis.Criterion, error) {
	return unresolvedCriterion{}, nil
}

type unresolvedCriterion struct{}

func (unresolvedCriterion) Name() string                               { return "unresolved" }
func (unresolvedCriterion) Requirements() []analysis.CandleRequirement { return nil }
func (unresolvedCriterion) Evaluate(context.Context, analysis.Input) (analysis.Evaluation, error) {
	return analysis.Evaluation{}, &analysis.UnresolvedError{Code: "missing", Message: "missing"}
}

type marketCapTestFactory struct{}

func (marketCapTestFactory) Name() string { return "market_cap" }
func (marketCapTestFactory) Build(map[string]any) (analysis.Criterion, error) {
	return marketCapTestCriterion{}, nil
}

type marketCapTestCriterion struct{}

func (marketCapTestCriterion) Name() string                               { return "market_cap" }
func (marketCapTestCriterion) Requirements() []analysis.CandleRequirement { return nil }
func (marketCapTestCriterion) MinimumMarketCapUSD() float64               { return 0 }
func (marketCapTestCriterion) Evaluate(_ context.Context, input analysis.Input) (analysis.Evaluation, error) {
	return analysis.Evaluation{Matched: true, Metrics: map[string]float64{"market_cap_usd": float64(input.Instrument.ID)}}, nil
}

type fakeFactory struct{}

func (fakeFactory) Name() string                                     { return "fake" }
func (fakeFactory) Build(map[string]any) (analysis.Criterion, error) { return fakeCriterion{}, nil }

type fakeCriterion struct{}

func (fakeCriterion) Name() string { return "fake" }
func (fakeCriterion) Requirements() []analysis.CandleRequirement {
	return []analysis.CandleRequirement{{Unit: analysis.UnitDays, Count: 1}, {Unit: analysis.UnitHours, Count: 1}}
}

type hourlyMatchFactory struct{}

func (hourlyMatchFactory) Name() string { return "hourly-match" }
func (hourlyMatchFactory) Build(map[string]any) (analysis.Criterion, error) {
	return hourlyMatchCriterion{}, nil
}

type hourlyMatchCriterion struct{}

func (hourlyMatchCriterion) Name() string { return "hourly-match" }
func (hourlyMatchCriterion) Requirements() []analysis.CandleRequirement {
	return []analysis.CandleRequirement{{Unit: analysis.UnitHours, Count: 1}}
}
func (hourlyMatchCriterion) Evaluate(_ context.Context, input analysis.Input) (analysis.Evaluation, error) {
	candles := input.Candles[analysis.UnitHours]
	if len(candles) == 0 {
		return analysis.Evaluation{}, &analysis.InsufficientHistoryError{Required: 1}
	}
	matched := candles[0].High >= 105
	return analysis.Evaluation{Matched: matched, Metrics: map[string]float64{}, CandleCount: 1}, nil
}

func (fakeCriterion) Evaluate(_ context.Context, input analysis.Input) (analysis.Evaluation, error) {
	if len(input.Candles[analysis.UnitDays]) != 2 || len(input.Candles[analysis.UnitHours]) != 1 {
		return analysis.Evaluation{}, analysis.ErrInvalidArgument
	}
	return analysis.Evaluation{Matched: false, Metrics: map[string]float64{}, CandleCount: 1}, nil
}

type storeStub struct {
	instruments         []market.Instrument
	rankedInstruments   []market.Instrument
	candles             map[string][]market.Candle
	candlesByInstrument map[int64]map[string][]market.Candle
	loads               map[string]int
	failRepeatedLoad    bool
	reads               int
	activeListCalls     int
	selectionLimit      int
	selectionDirection  string
	selectionSymbols    []string
	syncUnavailable     bool
}

func (s *storeStub) SelectActiveInstruments(_ context.Context, selection analysis.Selection) ([]market.Instrument, error) {
	s.reads++
	s.selectionLimit = selection.Limit
	s.selectionDirection = selection.SortDirection
	s.selectionSymbols = append([]string(nil), selection.Symbols...)
	items := s.instruments
	if selection.SortFact == analysis.SelectionFactMarketCapUSD && s.rankedInstruments != nil {
		items = s.rankedInstruments
	}
	if len(selection.Symbols) > 0 {
		allowed := make(map[string]bool, len(selection.Symbols))
		for _, symbol := range selection.Symbols {
			allowed[symbol] = true
		}
		filtered := make([]market.Instrument, 0, len(items))
		for _, item := range items {
			if allowed[item.Symbol] {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	if selection.Limit > 0 {
		items = items[:min(selection.Limit, len(items))]
	}
	return items, nil
}

func (s *storeStub) ListHourlyPrices(context.Context, []int64, time.Time, time.Time) ([]market.HourlyPrice, error) {
	return nil, nil
}

func (s *storeStub) GetSyncState(context.Context, market.SyncProfile) (market.SyncState, error) {
	s.reads++
	if s.syncUnavailable {
		return market.SyncState{}, nil
	}
	now := time.Now()
	return market.SyncState{LastSucceededAt: &now}, nil
}
func (s *storeStub) ListActiveInstruments(context.Context) ([]market.Instrument, error) {
	s.reads++
	s.activeListCalls++
	return s.instruments, nil
}
func (s *storeStub) ListLatestCandles(_ context.Context, instrumentIDs []int64, interval market.CandleInterval, _ int) (map[int64][]market.Candle, error) {
	s.reads++
	if s.loads == nil {
		s.loads = map[string]int{}
	}
	s.loads[string(interval)]++
	if s.failRepeatedLoad && s.loads[string(interval)] > 1 {
		return nil, errors.New("interval loaded more than once")
	}
	result := make(map[int64][]market.Candle, len(instrumentIDs))
	for _, instrumentID := range instrumentIDs {
		result[instrumentID] = s.candles[string(interval)]
		if s.candlesByInstrument != nil {
			result[instrumentID] = s.candlesByInstrument[instrumentID][string(interval)]
		}
	}
	return result, nil
}
func testCandle(r float64) market.Candle {
	return market.Candle{OpenTime: time.Now(), Open: 100, High: 100 + r, Low: 100}
}
