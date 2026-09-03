package analysis_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"crypto-scanner/internal/analysis"
	"crypto-scanner/internal/analysis/criteria/market_cap"
	"crypto-scanner/internal/analysis/criteria/volatility"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/marketcap"
)

func TestMarketScanPipelineRetainsOnlySequentialSurvivors(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		t.Run(map[bool]string{true: "with Market Cap", false: "without Market Cap"}[enabled], func(t *testing.T) {
			store := &storeStub{
				instruments: []market.Instrument{
					{ID: 1, Symbol: "DAILYDROPUSDT", BaseAsset: "DAILYDROP"},
					{ID: 2, Symbol: "HOURLYDROPUSDT", BaseAsset: "HOURLYDROP"},
					{ID: 3, Symbol: "KEEPUSDT", BaseAsset: "KEEP"},
					{ID: 4, Symbol: "LOWCAPUSDT", BaseAsset: "LOWCAP"},
					{ID: 5, Symbol: "UNKNOWNUSDT", BaseAsset: "UNKNOWN"},
				},
				candlesByInstrument: map[int64]map[string][]market.Candle{
					1: {"1d": {testCandle(4)}},
					2: {"1d": {testCandle(6)}, "1h": {testCandle(1)}},
					3: {"1d": {testCandle(8)}, "1h": {testCandle(3), testCandle(3)}},
					4: {"1d": {testCandle(7)}, "1h": {testCandle(2)}},
					5: {"1d": {testCandle(6)}, "1h": {testCandle(2)}},
				},
			}
			caps := &pipelineCapStore{}
			service, err := analysis.NewService(store, volatility.New(), market_cap.New(marketcap.New(caps, unavailableCapProvider{})))
			if err != nil {
				t.Fatal(err)
			}
			criteria := []analysis.CriterionConfig{
				{Key: "daily_volatility", Label: "Daily Volatility", Name: "volatility", Parameters: map[string]any{"unit": "days", "period": float64(30), "percentile": float64(80), "minimum_range_percent": float64(5)}},
				{Key: "hourly_volatility", Label: "Hourly Volatility", Name: "volatility", Parameters: map[string]any{"unit": "hours", "period": float64(60), "percentile": float64(80), "minimum_range_percent": float64(2)}},
			}
			if enabled {
				criteria = append(criteria, analysis.CriterionConfig{Key: "market_cap", Label: "Market Cap", Name: "market_cap", Parameters: map[string]any{"min_market_cap_usd": float64(500_000_000)}})
			}
			result, err := service.Search(context.Background(), analysis.SearchRequest{Criteria: criteria})
			if err != nil {
				t.Fatal(err)
			}
			if result.InsufficientDataCount != 0 || store.loads["1h"] != 4 {
				t.Fatalf("daily rejection reached hourly analysis: result=%+v loads=%v", result, store.loads)
			}
			wantSymbols := []string{"KEEPUSDT", "LOWCAPUSDT", "UNKNOWNUSDT"}
			if enabled {
				wantSymbols = []string{"KEEPUSDT"}
			}
			var symbols []string
			for _, item := range result.Items {
				symbols = append(symbols, item.Symbol)
				if len(item.Evaluations) != len(criteria) {
					t.Fatalf("evaluations=%+v", item.Evaluations)
				}
				for i, evaluation := range item.Evaluations {
					if evaluation.Key != criteria[i].Key || evaluation.Label != criteria[i].Label || evaluation.Name != criteria[i].Name || !evaluation.Matched {
						t.Fatalf("evaluation=%+v criterion=%+v", evaluation, criteria[i])
					}
				}
			}
			if !reflect.DeepEqual(symbols, wantSymbols) || result.MatchedCount != len(wantSymbols) {
				t.Fatalf("result=%+v", result)
			}
			daily, hourly := result.Items[0].Evaluations[0], result.Items[0].Evaluations[1]
			if daily.Metrics["range_percent"] != 8 || daily.CandleCount != 1 || hourly.Metrics["range_percent"] != 3 || hourly.CandleCount != 2 {
				t.Fatalf("daily=%+v hourly=%+v", daily, hourly)
			}
			if enabled {
				slices.Sort(caps.requested)
				if !reflect.DeepEqual(caps.requested, []string{"KEEP", "LOWCAP", "UNKNOWN"}) {
					t.Fatalf("Market Cap candidates=%v", caps.requested)
				}
				if result.AnalyzedCount != 4 || len(result.Unresolved) != 1 || result.Unresolved[0].Symbol != "UNKNOWNUSDT" || result.Unresolved[0].Code != "mapping_not_found" {
					t.Fatalf("result=%+v", result)
				}
				if len(result.Warnings) != 1 || result.Warnings[0].Code != "market_cap_provider_unavailable" || result.Items[0].Evaluations[2].Metrics["market_cap_usd"] != 500_000_000 {
					t.Fatalf("cached Market Cap response=%+v", result)
				}
			} else if result.AnalyzedCount != 5 || len(result.Unresolved) != 0 || len(result.Warnings) != 0 || len(caps.requested) != 0 {
				t.Fatalf("disabled Market Cap was evaluated: result=%+v requests=%v", result, caps.requested)
			}
		})
	}
}

func TestMarketScanPreservesStoreOrder(t *testing.T) {
	for _, position := range []string{"first", "last", "disabled"} {
		t.Run(position, func(t *testing.T) {
			store := &storeStub{
				instruments: []market.Instrument{
					{ID: 1, Symbol: "AAAZEROUSDT", BaseAsset: "ZERO"},
					{ID: 2, Symbol: "SMALLUSDT", BaseAsset: "SMALL"},
					{ID: 3, Symbol: "BIGUSDT", BaseAsset: "BIG"},
					{ID: 4, Symbol: "LARGEUSDT", BaseAsset: "LARGE"},
				},
				candlesByInstrument: map[int64]map[string][]market.Candle{
					1: {"1d": {testCandle(10)}},
					2: {"1d": {testCandle(9)}},
					3: {"1d": {testCandle(6)}},
					4: {"1d": {testCandle(7)}},
				},
			}
			caps := &pipelineCapStore{values: map[string]float64{"ZERO": 0, "SMALL": 100, "BIG": 1000, "LARGE": 1000}}
			service, err := analysis.NewService(store, volatility.New(), market_cap.New(marketcap.New(caps, unavailableCapProvider{})))
			if err != nil {
				t.Fatal(err)
			}
			criteria := []analysis.CriterionConfig{
				{Key: "daily", Name: "volatility", Label: "Daily", Parameters: map[string]any{"unit": "days", "period": float64(1), "percentile": float64(50), "minimum_range_percent": float64(5)}},
			}
			capConfig := analysis.CriterionConfig{Key: "custom_cap_key", Name: "market_cap", Label: "Capitalization", Parameters: map[string]any{"min_market_cap_usd": float64(0)}}
			switch position {
			case "first":
				criteria = append([]analysis.CriterionConfig{capConfig}, criteria...)
			case "last":
				criteria = append(criteria, capConfig)
			}
			result, err := service.Search(context.Background(), analysis.SearchRequest{Criteria: criteria})
			if err != nil {
				t.Fatal(err)
			}
			var symbols []string
			for _, item := range result.Items {
				symbols = append(symbols, item.Symbol)
			}
			want := []string{"AAAZEROUSDT", "SMALLUSDT", "BIGUSDT", "LARGEUSDT"}
			if !slices.Equal(symbols, want) {
				t.Fatalf("symbols = %v, want %v", symbols, want)
			}
			if position == "disabled" && len(caps.requested) != 0 {
				t.Fatalf("disabled Market Cap was loaded: %v", caps.requested)
			}
		})
	}
}

type pipelineCapStore struct {
	requested []string
	values    map[string]float64
}

func (*pipelineCapStore) BootstrapCompleted(context.Context) (bool, error)           { return true, nil }
func (*pipelineCapStore) ReplaceSnapshot(context.Context, []marketcap.Mapping) error { return nil }
func (s *pipelineCapStore) GetMapping(_ context.Context, base string) (marketcap.Mapping, error) {
	s.requested = append(s.requested, base)
	if base == "UNKNOWN" {
		return marketcap.Mapping{BaseAsset: base, Status: "unresolved", Reason: "mapping_not_found"}, nil
	}
	return marketcap.Mapping{BaseAsset: base, CoinID: base, Status: "resolved"}, nil
}
func (*pipelineCapStore) SaveMapping(context.Context, marketcap.Mapping) error { return nil }
func (s *pipelineCapStore) GetCap(_ context.Context, id string) (marketcap.Cap, error) {
	value := float64(500_000_000)
	if id == "LOWCAP" {
		value = 499_999_999
	}
	if s.values != nil {
		value = s.values[id]
	}
	return marketcap.Cap{CoinID: id, USD: value, Available: true, FetchedAt: time.Now().Add(-2 * time.Hour)}, nil
}
func (*pipelineCapStore) SaveCap(context.Context, marketcap.Cap) error { return nil }

type unavailableCapProvider struct{}

func (unavailableCapProvider) Tickers(context.Context, int) ([]marketcap.Ticker, error) {
	return nil, errors.New("provider unavailable")
}
func (unavailableCapProvider) Markets(context.Context, []string) ([]marketcap.Cap, error) {
	return nil, errors.New("provider unavailable")
}
