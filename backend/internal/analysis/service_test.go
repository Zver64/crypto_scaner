package analysis_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"crypto-scanner/internal/analysis"
	"crypto-scanner/internal/analysis/criteria/percentile"
	"crypto-scanner/internal/market"
)

func TestServiceCombinesCriteriaAndLoadsMergedRequirementsOnce(t *testing.T) {
	store := &storeStub{instruments: []market.Instrument{{ID: 1, Symbol: "BTCUSDT"}}, candles: map[string][]market.Candle{"1d": {testCandle(1), testCandle(2)}, "1h": {testCandle(1)}}, failRepeatedLoad: true}
	service, err := analysis.NewService(store, percentile.New(), fakeFactory{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.AnalyzeSymbol(context.Background(), analysis.SymbolRequest{Symbol: "BTCUSDT", Criteria: []analysis.CriterionConfig{{Name: "percentile", Parameters: map[string]any{"unit": "days", "period": float64(2), "percentile": float64(50), "minimum_range_percent": float64(0)}}, {Name: "fake", Parameters: map[string]any{}}}})
	if err != nil || result.Matched || len(result.Evaluations) != 2 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestServiceSearchCombinesCriteriaAndOrdersMatches(t *testing.T) {
	store := &storeStub{
		instruments: []market.Instrument{
			{ID: 1, Symbol: "ZZZUSDT"}, {ID: 2, Symbol: "AAAUSDT"}, {ID: 3, Symbol: "LOWUSDT"},
			{ID: 4, Symbol: "FAILUSDT"}, {ID: 5, Symbol: "NEWUSDT"},
		},
		candlesByInstrument: map[int64]map[string][]market.Candle{
			1: {"1d": {testCandle(5), testCandle(5)}, "1h": {testCandle(6)}},
			2: {"1d": {testCandle(5), testCandle(5)}, "1h": {testCandle(6)}},
			3: {"1d": {testCandle(2), testCandle(2)}, "1h": {testCandle(6)}},
			4: {"1d": {testCandle(6), testCandle(6)}, "1h": {testCandle(2)}},
			5: {"1d": {testCandle(5)}, "1h": {testCandle(6)}},
		},
	}
	service, err := analysis.NewService(store, percentile.New(), hourlyMatchFactory{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Search(context.Background(), analysis.SearchRequest{Criteria: []analysis.CriterionConfig{
		{Name: "percentile", Parameters: map[string]any{"unit": "days", "period": float64(2), "percentile": float64(50), "minimum_range_percent": float64(3)}},
		{Name: "hourly-match", Parameters: map[string]any{}},
	}})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if result.MatchedCount != 2 || result.AnalyzedCount != 4 || result.InsufficientDataCount != 1 {
		t.Fatalf("Search() counts = %+v", result)
	}
	if len(result.Items) != 2 || result.Items[0].Symbol != "AAAUSDT" || result.Items[1].Symbol != "ZZZUSDT" {
		t.Fatalf("Search() items = %+v", result.Items)
	}
	for _, item := range result.Items {
		if !item.Matched || len(item.Evaluations) != 2 || !item.Evaluations[0].Matched || !item.Evaluations[1].Matched {
			t.Fatalf("Search() item = %+v", item)
		}
	}
}
func TestServiceRejectsInvalidSelectionBeforeReads(t *testing.T) {
	store := &storeStub{}
	service, _ := analysis.NewService(store, percentile.New())
	for _, configs := range [][]analysis.CriterionConfig{nil, {{Name: "missing"}}, {{Name: "percentile"}, {Name: "percentile"}}} {
		_, err := service.Search(context.Background(), analysis.SearchRequest{Criteria: configs})
		if !errors.Is(err, analysis.ErrInvalidArgument) {
			t.Fatalf("err=%v", err)
		}
	}
	if store.reads != 0 {
		t.Fatalf("reads=%d", store.reads)
	}
}
func TestServiceRejectsDuplicateFactoryNames(t *testing.T) {
	_, err := analysis.NewService(&storeStub{}, fakeFactory{}, fakeFactory{})
	if !errors.Is(err, analysis.ErrInvalidArgument) {
		t.Fatalf("err=%v", err)
	}
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
func (hourlyMatchCriterion) Evaluate(_ context.Context, data map[analysis.Unit][]market.Candle) (analysis.Evaluation, error) {
	candles := data[analysis.UnitHours]
	if len(candles) == 0 {
		return analysis.Evaluation{}, &analysis.InsufficientHistoryError{Required: 1}
	}
	matched := candles[0].High >= 105
	return analysis.Evaluation{Matched: matched, Metrics: map[string]float64{}, CandleCount: 1}, nil
}

func (fakeCriterion) Evaluate(_ context.Context, data map[analysis.Unit][]market.Candle) (analysis.Evaluation, error) {
	if len(data[analysis.UnitDays]) != 2 || len(data[analysis.UnitHours]) != 1 {
		return analysis.Evaluation{}, analysis.ErrInvalidArgument
	}
	return analysis.Evaluation{Matched: false, Metrics: map[string]float64{}, CandleCount: 1}, nil
}

type storeStub struct {
	instruments         []market.Instrument
	candles             map[string][]market.Candle
	candlesByInstrument map[int64]map[string][]market.Candle
	loads               map[string]int
	failRepeatedLoad    bool
	reads               int
}

func (s *storeStub) GetSyncState(context.Context, market.SyncProfile) (market.SyncState, error) {
	s.reads++
	now := time.Now()
	return market.SyncState{LastSucceededAt: &now}, nil
}
func (s *storeStub) ListActiveInstruments(context.Context) ([]market.Instrument, error) {
	s.reads++
	return s.instruments, nil
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
