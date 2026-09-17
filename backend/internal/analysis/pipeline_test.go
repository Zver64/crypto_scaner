package analysis_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"crypto-scanner/internal/analysis"
	"crypto-scanner/internal/analysis/criteria/market_cap"
	"crypto-scanner/internal/analysis/criteria/volatility"
	"crypto-scanner/internal/market"
)

func TestSearchUsesPersistedSelectionBeforeLimitWithoutProviderPreparation(t *testing.T) {
	cap := 100.0
	store := &selectionStore{items: []market.Instrument{{ID: 2, Symbol: "BTCUSDT", BaseAsset: "BTC", MarketCapUSD: &cap}}, candles: map[int64][]market.Candle{2: {testCandle(8)}}}
	service, err := analysis.NewService(store, volatility.New(), market_cap.New())
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Search(context.Background(), analysis.SearchRequest{
		Criteria: []analysis.CriterionConfig{
			{Key: "cap", Name: "market_cap", Label: "Market Cap", Parameters: map[string]any{"min_market_cap_usd": float64(100)}},
			{Key: "daily", Name: "volatility", Label: "Daily", Parameters: map[string]any{"unit": "days", "period": float64(1), "percentile": float64(50), "minimum_range_percent": float64(1)}},
		},
		Limit: 1, Sort: &analysis.SearchSort{Field: "market_cap_usd", Direction: "desc"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(store.selection.Constraints) != 2 || store.selection.Limit != 1 || store.selection.SortFact != analysis.SelectionFactMarketCapUSD || store.selection.SortDirection != "desc" {
		t.Fatalf("selection = %+v", store.selection)
	}
	if stable, cap := store.selection.Constraints[0], store.selection.Constraints[1]; stable.Fact != analysis.SelectionFactStablecoin || stable.Boolean || cap.Fact != analysis.SelectionFactMarketCapUSD || cap.Number != 100 {
		t.Fatalf("constraints = %+v", store.selection.Constraints)
	}
	if len(result.Items) != 1 || result.Items[0].Symbol != "BTCUSDT" || result.Items[0].Evaluations[0].Metrics["market_cap_usd"] != 100 {
		t.Fatalf("result = %+v", result)
	}
}

func TestRepeatedMarketCapConstraintsUseMaximumBeforeAscendingLimit(t *testing.T) {
	for _, minimums := range [][2]float64{{100, 500}, {500, 100}} {
		t.Run(fmt.Sprintf("%v_then_%v", minimums[0], minimums[1]), func(t *testing.T) {
			cap := 500.0
			store := &selectionStore{items: []market.Instrument{{ID: 1, Symbol: "BTCUSDT", MarketCapUSD: &cap}}}
			service, err := analysis.NewService(store, market_cap.New())
			if err != nil {
				t.Fatal(err)
			}
			criteria := []analysis.CriterionConfig{
				{Key: "cap_one", Name: "market_cap", Label: "Cap one", Parameters: map[string]any{"min_market_cap_usd": minimums[0]}},
				{Key: "cap_two", Name: "market_cap", Label: "Cap two", Parameters: map[string]any{"min_market_cap_usd": minimums[1]}},
			}
			result, err := service.Search(context.Background(), analysis.SearchRequest{Criteria: criteria, Limit: 1, Sort: &analysis.SearchSort{Field: "market_cap_usd", Direction: "asc"}})
			if err != nil || result.MatchedCount != 1 {
				t.Fatalf("result=%+v error=%v", result, err)
			}
			if len(store.selection.Constraints) != 2 || store.selection.Constraints[1].Number != 500 || store.selection.SortDirection != "asc" || store.selection.Limit != 1 {
				t.Fatalf("selection=%+v", store.selection)
			}
		})
	}
}

func TestDirectStablecoinAnalysisDoesNotApplyMarketSearchDefault(t *testing.T) {
	cap := 100.0
	store := &selectionStore{items: []market.Instrument{{ID: 1, Symbol: "USDCUSDT", BaseAsset: "USDC", MarketCapUSD: &cap}}}
	service, err := analysis.NewService(store, market_cap.New())
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.AnalyzeSymbol(context.Background(), analysis.SymbolRequest{Symbol: "USDCUSDT", Criteria: []analysis.CriterionConfig{{Key: "cap", Name: "market_cap", Label: "Cap", Parameters: map[string]any{"min_market_cap_usd": float64(0)}}}})
	if err != nil || !result.Matched || len(store.selection.Constraints) != 0 || store.selection.Symbol != "USDCUSDT" {
		t.Fatalf("result=%+v selection=%+v error=%v", result, store.selection, err)
	}
}

func TestSearchRejectsMarketCapSortWithoutPersistedMarketCapConstraint(t *testing.T) {
	service, _ := analysis.NewService(&selectionStore{}, volatility.New())
	_, err := service.Search(context.Background(), analysis.SearchRequest{
		Criteria: []analysis.CriterionConfig{{Key: "daily", Name: "volatility", Label: "Daily", Parameters: map[string]any{"unit": "days", "period": float64(1), "percentile": float64(50), "minimum_range_percent": float64(0)}}},
		Sort:     &analysis.SearchSort{Field: "market_cap_usd", Direction: "desc"},
	})
	if err != analysis.ErrInvalidArgument {
		t.Fatalf("error = %v", err)
	}
}

type selectionStore struct {
	items     []market.Instrument
	candles   map[int64][]market.Candle
	selection analysis.Selection
}

func (s *selectionStore) SelectActiveInstruments(_ context.Context, selection analysis.Selection) ([]market.Instrument, error) {
	s.selection = selection
	return s.items, nil
}
func (*selectionStore) ListActiveInstruments(context.Context) ([]market.Instrument, error) {
	return nil, nil
}
func (*selectionStore) GetSyncState(context.Context, market.SyncProfile) (market.SyncState, error) {
	now := time.Now()
	return market.SyncState{LastSucceededAt: &now}, nil
}
func (s *selectionStore) ListLatestCandlesByInterval(_ context.Context, id int64, _ string, _ int) ([]market.Candle, error) {
	return s.candles[id], nil
}
func (*selectionStore) ListHourlyPrices(context.Context, []int64, time.Time, time.Time) ([]market.HourlyPrice, error) {
	return nil, nil
}
