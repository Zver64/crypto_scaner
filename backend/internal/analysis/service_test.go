package analysis_test

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"crypto-scanner/internal/analysis"
	marketcapcriterion "crypto-scanner/internal/analysis/criteria/market_cap"
	"crypto-scanner/internal/analysis/criteria/volatility"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/marketcap"
)

func TestServiceCombinesCriteriaAndLoadsMergedRequirementsOnce(t *testing.T) {
	store := &storeStub{instruments: []market.Instrument{{ID: 1, Symbol: "BTCUSDT"}}, candles: map[string][]market.Candle{"1d": {testCandle(1), testCandle(2)}, "1h": {testCandle(1)}}, failRepeatedLoad: true}
	service, err := analysis.NewService(store, volatility.New(), fakeFactory{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.AnalyzeSymbol(context.Background(), analysis.SymbolRequest{Symbol: "BTCUSDT", Criteria: []analysis.CriterionConfig{{Key: "volatility", Name: "volatility", Label: "Volatility", Parameters: map[string]any{"unit": "days", "period": float64(2), "percentile": float64(50), "minimum_range_percent": float64(0)}}, {Key: "fake", Name: "fake", Label: "Fake", Parameters: map[string]any{}}}})
	if err != nil || result.Matched || len(result.Evaluations) != 2 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestAnalyzeSymbolBuildsThirtyDayGappedHourlyCandleHistory(t *testing.T) {
	window := market.ThirtyDayWindow(time.Now())
	first := window.From
	store := &storeStub{
		instruments: []market.Instrument{{ID: 1, Symbol: "BTCUSDT"}},
		candles:     map[string][]market.Candle{"1d": {testCandle(1)}},
		hourlyCandles: []market.HourlyCandle{
			{InstrumentID: 1, OpenTime: first, Open: 10, High: 12, Low: 9, Close: 11},
			{InstrumentID: 2, OpenTime: first.Add(time.Hour), Open: 20, High: 22, Low: 19, Close: 21},
			{InstrumentID: 1, OpenTime: first.Add(time.Hour + time.Minute), Open: 30, High: 32, Low: 29, Close: 31},
			{InstrumentID: 1, OpenTime: first.Add(2 * time.Hour), Open: 12, High: 14, Low: 11, Close: 13},
		},
	}
	service, err := analysis.NewService(store, volatility.New())
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.AnalyzeSymbol(context.Background(), analysis.SymbolRequest{Symbol: "BTCUSDT", Criteria: []analysis.CriterionConfig{{Key: "volatility", Name: "volatility", Label: "Volatility", Parameters: map[string]any{"unit": "days", "period": float64(1), "percentile": float64(50), "minimum_range_percent": float64(0)}}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.CandleHistory) != market.ThirtyDayPriceSlots || result.CandleHistory[0] == nil || result.CandleHistory[0].Close != 11 || result.CandleHistory[2] == nil || result.CandleHistory[2].Close != 13 {
		t.Fatalf("candle history = %+v", result.CandleHistory)
	}
	if result.CandleHistory[1] != nil || !result.PriceHistoryWindow.From.Equal(window.From) || !result.PriceHistoryWindow.To.Equal(window.To) {
		t.Fatalf("history window/gap = %+v / %+v", result.PriceHistoryWindow, result.CandleHistory[1])
	}
}

func TestAnalyzeSymbolShortCircuitsLaterCriteria(t *testing.T) {
	store := &storeStub{
		instruments:         []market.Instrument{{ID: 1, Symbol: "DROP"}},
		candlesByInstrument: map[int64]map[string][]market.Candle{1: {"1d": {testCandle(1)}}},
	}
	second := &trackingFactory{}
	service, err := analysis.NewService(store, firstFactory{}, second)
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
	if result.Matched || len(result.Evaluations) != 1 || second.prepared != 0 || second.evaluated != 0 {
		t.Fatalf("result=%+v prepared=%d evaluated=%d", result, second.prepared, second.evaluated)
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
	service, err := analysis.NewService(store, volatility.New())
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
	service, err := analysis.NewService(store, volatility.New())
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
	service, err := analysis.NewService(store, volatility.New(), hourlyMatchFactory{})
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
	if result.MatchedCount != 3 || result.AnalyzedCount != 5 || result.InsufficientDataCount != 1 {
		t.Fatalf("Search() counts = %+v", result)
	}
	if len(result.Items) != 3 || result.Items[0].Symbol != "ZZZUSDT" || result.Items[1].Symbol != "AAAUSDT" || result.Items[2].Symbol != "NEWUSDT" {
		t.Fatalf("Search() items = %+v", result.Items)
	}
	for _, item := range result.Items {
		if !item.Matched || len(item.Evaluations) != 2 || !item.Evaluations[0].Matched || !item.Evaluations[1].Matched {
			t.Fatalf("Search() item = %+v", item)
		}
	}
	if result.Items[2].Evaluations[0].CandleCount != 1 {
		t.Fatalf("NEWUSDT evaluation = %+v", result.Items[2].Evaluations[0])
	}
}
func TestSearchRefreshesMarketCapsBeforeDatabaseRanking(t *testing.T) {
	for _, test := range []struct {
		name string
		caps map[string]marketcap.Cap
	}{
		{name: "cold cache", caps: map[string]marketcap.Cap{}},
		{name: "stale cache", caps: map[string]marketcap.Cap{
			"a": {CoinID: "a", USD: 100, Available: true, FetchedAt: time.Now().Add(-2 * time.Hour)},
			"b": {CoinID: "b", USD: 90, Available: true, FetchedAt: time.Now().Add(-2 * time.Hour)},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := &rankingStore{
				instruments: []market.Instrument{{ID: 1, Symbol: "AUSDT", BaseAsset: "A"}, {ID: 2, Symbol: "BUSDT", BaseAsset: "B"}},
				mappings: map[string]marketcap.Mapping{
					"A": {BaseAsset: "A", CoinID: "a", Status: "resolved"},
					"B": {BaseAsset: "B", CoinID: "b", Status: "resolved"},
				},
				caps: test.caps,
			}
			provider := &rankingProvider{caps: []marketcap.Cap{
				{CoinID: "a", USD: 80, Available: true},
				{CoinID: "b", USD: 110, Available: true},
			}}
			service, err := analysis.NewService(store, marketcapcriterion.New(marketcap.New(store, provider)))
			if err != nil {
				t.Fatal(err)
			}
			result, err := service.Search(context.Background(), analysis.SearchRequest{
				Criteria: []analysis.CriterionConfig{{Key: "market_cap", Name: "market_cap", Label: "Market Cap", Parameters: map[string]any{"min_market_cap_usd": float64(0)}}},
				Limit:    1,
				Sort:     &analysis.SearchSort{Field: "market_cap_usd", Direction: "desc"},
			})
			if err != nil {
				t.Fatal(err)
			}
			if provider.calls != 1 || len(result.Items) != 1 || result.Items[0].Symbol != "BUSDT" {
				t.Fatalf("provider calls=%d items=%+v", provider.calls, result.Items)
			}
		})
	}
}

func TestSearchUsesBackendMarketCapSortAndLimit(t *testing.T) {
	store := &storeStub{
		rankedInstruments: []market.Instrument{
			{ID: 2, Symbol: "BTCUSDT"},
			{ID: 1, Symbol: "ETHUSDT"},
		},
	}
	service, err := analysis.NewService(store, marketCapTestFactory{})
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
	if store.selectionLimit != 1 || store.selectionDirection != "desc" || store.activeListCalls != 1 {
		t.Fatalf("selection limit=%d direction=%q active calls=%d", store.selectionLimit, store.selectionDirection, store.activeListCalls)
	}
}

func TestSearchUsesBackendLimitWithoutChangingStoreOrder(t *testing.T) {
	store := &storeStub{
		instruments: []market.Instrument{
			{ID: 1, Symbol: "ZZZUSDT"},
			{ID: 2, Symbol: "AAAUSDT"},
		},
	}
	service, err := analysis.NewService(store, marketCapTestFactory{})
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
	service, _ := analysis.NewService(store, volatility.New())
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
	service, _ := analysis.NewService(store, volatility.New())
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
	_, err := analysis.NewService(&storeStub{}, fakeFactory{}, fakeFactory{})
	if !errors.Is(err, analysis.ErrInvalidArgument) {
		t.Fatalf("err=%v", err)
	}
}
func TestSearchPreparesAndLoadsSecondCriterionOnlyForSurvivors(t *testing.T) {
	store := &storeStub{instruments: []market.Instrument{{ID: 1, Symbol: "DROP"}, {ID: 2, Symbol: "KEEP"}}, candlesByInstrument: map[int64]map[string][]market.Candle{1: {"1d": {testCandle(1)}}, 2: {"1d": {testCandle(2)}, "1h": {testCandle(1)}}}}
	second := &trackingFactory{}
	service, _ := analysis.NewService(store, firstFactory{}, second)
	result, err := service.Search(context.Background(), analysis.SearchRequest{Criteria: []analysis.CriterionConfig{{Key: "first", Name: "first", Label: "First", Parameters: map[string]any{}}, {Key: "second", Name: "second", Label: "Second", Parameters: map[string]any{}}}})
	if err != nil || result.MatchedCount != 1 || second.prepared != 1 || second.evaluated != 1 {
		t.Fatalf("result=%+v err=%v prepared=%d evaluated=%d", result, err, second.prepared, second.evaluated)
	}
	if store.loads["1h"] != 1 {
		t.Fatalf("second criterion candle loads=%v", store.loads)
	}
}
func TestSearchSkipsLaterCriteriaWhenNoCandidatesSurvive(t *testing.T) {
	store := &storeStub{instruments: []market.Instrument{{ID: 1, Symbol: "DROP"}}, candlesByInstrument: map[int64]map[string][]market.Candle{1: {"1d": {testCandle(1)}}}}
	second := &trackingFactory{}
	service, _ := analysis.NewService(store, firstFactory{}, second)
	result, err := service.Search(context.Background(), analysis.SearchRequest{Criteria: []analysis.CriterionConfig{{Key: "first", Name: "first", Label: "First", Parameters: map[string]any{}}, {Key: "second", Name: "second", Label: "Second", Parameters: map[string]any{}}}})
	if err != nil || result.MatchedCount != 0 || second.prepared != 0 || second.evaluated != 0 {
		t.Fatalf("result=%+v err=%v prepared=%d evaluated=%d", result, err, second.prepared, second.evaluated)
	}
}

func TestSearchDoesNotCountUnresolvedInstrumentsAsAnalyzed(t *testing.T) {
	store := &storeStub{instruments: []market.Instrument{{ID: 1, Symbol: "UNKNOWN"}}}
	service, _ := analysis.NewService(store, unresolvedFactory{})
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
func (firstCriterion) Prepare(context.Context, []market.Instrument) ([]analysis.Warning, error) {
	return nil, nil
}
func (firstCriterion) Evaluate(_ context.Context, input analysis.Input) (analysis.Evaluation, error) {
	return analysis.Evaluation{Matched: input.Instrument.ID == 2}, nil
}

type trackingFactory struct{ prepared, evaluated int }

func (t *trackingFactory) Name() string { return "second" }
func (t *trackingFactory) Build(map[string]any) (analysis.Criterion, error) {
	return &trackingCriterion{factory: t}, nil
}

type trackingCriterion struct{ factory *trackingFactory }

func (*trackingCriterion) Name() string { return "second" }
func (*trackingCriterion) Requirements() []analysis.CandleRequirement {
	return []analysis.CandleRequirement{{Unit: analysis.UnitHours, Count: 1}}
}
func (t *trackingCriterion) Prepare(_ context.Context, candidates []market.Instrument) ([]analysis.Warning, error) {
	t.factory.prepared = len(candidates)
	return nil, nil
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
func (unresolvedCriterion) Prepare(context.Context, []market.Instrument) ([]analysis.Warning, error) {
	return nil, nil
}
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
func (marketCapTestCriterion) Prepare(context.Context, []market.Instrument) ([]analysis.Warning, error) {
	return nil, nil
}
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
func (fakeCriterion) Prepare(context.Context, []market.Instrument) ([]analysis.Warning, error) {
	return nil, nil
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
func (hourlyMatchCriterion) Prepare(context.Context, []market.Instrument) ([]analysis.Warning, error) {
	return nil, nil
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

type rankingStore struct {
	instruments []market.Instrument
	mappings    map[string]marketcap.Mapping
	caps        map[string]marketcap.Cap
}

func (*rankingStore) GetSyncState(context.Context, market.SyncProfile) (market.SyncState, error) {
	return market.SyncState{}, nil
}
func (s *rankingStore) ListActiveInstruments(context.Context) ([]market.Instrument, error) {
	return s.instruments, nil
}
func (s *rankingStore) ListActiveInstrumentsLimited(_ context.Context, limit int) ([]market.Instrument, error) {
	return s.instruments[:min(limit, len(s.instruments))], nil
}
func (s *rankingStore) ListActiveInstrumentsSortedByMarketCap(_ context.Context, limit int, direction string) ([]market.Instrument, error) {
	items := append([]market.Instrument(nil), s.instruments...)
	sort.Slice(items, func(i, j int) bool {
		left := s.caps[s.mappings[items[i].BaseAsset].CoinID].USD
		right := s.caps[s.mappings[items[j].BaseAsset].CoinID].USD
		if direction == "asc" {
			return left < right
		}
		return left > right
	})
	if limit > 0 {
		items = items[:min(limit, len(items))]
	}
	return items, nil
}
func (*rankingStore) ListLatestCandlesByInterval(context.Context, int64, string, int) ([]market.Candle, error) {
	return nil, nil
}
func (*rankingStore) ListHourlyCandles(context.Context, int64, time.Time, time.Time) ([]market.HourlyCandle, error) {
	return nil, nil
}
func (*rankingStore) ListHourlyPrices(context.Context, []int64, time.Time, time.Time) ([]market.HourlyPrice, error) {
	return nil, nil
}
func (*rankingStore) BootstrapCompleted(context.Context) (bool, error)           { return true, nil }
func (*rankingStore) ReplaceSnapshot(context.Context, []marketcap.Mapping) error { return nil }
func (s *rankingStore) GetMapping(_ context.Context, base string) (marketcap.Mapping, error) {
	return s.mappings[base], nil
}
func (*rankingStore) SaveMapping(context.Context, marketcap.Mapping) error { return nil }
func (s *rankingStore) GetCap(_ context.Context, id string) (marketcap.Cap, error) {
	cap, ok := s.caps[id]
	if !ok {
		return marketcap.Cap{}, errors.New("missing cap")
	}
	return cap, nil
}
func (s *rankingStore) SaveCap(_ context.Context, cap marketcap.Cap) error {
	s.caps[cap.CoinID] = cap
	return nil
}

type rankingProvider struct {
	caps  []marketcap.Cap
	calls int
}

func (*rankingProvider) Tickers(context.Context, int) ([]marketcap.Ticker, error) { return nil, nil }
func (p *rankingProvider) Markets(context.Context, []string) ([]marketcap.Cap, error) {
	p.calls++
	return p.caps, nil
}

type storeStub struct {
	instruments         []market.Instrument
	rankedInstruments   []market.Instrument
	candles             map[string][]market.Candle
	candlesByInstrument map[int64]map[string][]market.Candle
	loads               map[string]int
	failRepeatedLoad    bool
	hourlyCandles       []market.HourlyCandle
	reads               int
	activeListCalls     int
	selectionLimit      int
	selectionDirection  string
}

func (s *storeStub) ListHourlyCandles(context.Context, int64, time.Time, time.Time) ([]market.HourlyCandle, error) {
	return s.hourlyCandles, nil
}

func (s *storeStub) ListHourlyPrices(context.Context, []int64, time.Time, time.Time) ([]market.HourlyPrice, error) {
	return nil, nil
}

func (s *storeStub) GetSyncState(context.Context, market.SyncProfile) (market.SyncState, error) {
	s.reads++
	now := time.Now()
	return market.SyncState{LastSucceededAt: &now}, nil
}
func (s *storeStub) ListActiveInstruments(context.Context) ([]market.Instrument, error) {
	s.reads++
	s.activeListCalls++
	return s.instruments, nil
}
func (s *storeStub) ListActiveInstrumentsLimited(_ context.Context, limit int) ([]market.Instrument, error) {
	s.reads++
	s.selectionLimit = limit
	return s.instruments[:min(limit, len(s.instruments))], nil
}
func (s *storeStub) ListActiveInstrumentsSortedByMarketCap(_ context.Context, limit int, direction string) ([]market.Instrument, error) {
	s.reads++
	s.selectionLimit = limit
	s.selectionDirection = direction
	items := s.rankedInstruments
	if limit > 0 {
		items = items[:min(limit, len(items))]
	}
	return items, nil
}
func (s *storeStub) ListLatestCandlesByInterval(_ context.Context, instrumentID int64, interval string, _ int) ([]market.Candle, error) {
	s.reads++
	if s.loads == nil {
		s.loads = map[string]int{}
	}
	s.loads[interval]++
	if s.failRepeatedLoad && s.loads[interval] > 1 {
		return nil, errors.New("interval loaded more than once")
	}
	candles := s.candles[interval]
	if s.candlesByInstrument != nil {
		candles = s.candlesByInstrument[instrumentID][interval]
	}
	return candles, nil
}
func testCandle(r float64) market.Candle {
	return market.Candle{OpenTime: time.Now(), Open: 100, High: 100 + r, Low: 100}
}
