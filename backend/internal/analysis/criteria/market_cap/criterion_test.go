package market_cap

import (
	"context"
	"errors"
	"testing"
	"time"

	"crypto-scanner/internal/analysis"
	"crypto-scanner/internal/market"
)

func TestValidationAndPersistedInclusiveBoundary(t *testing.T) {
	factory := New()
	if _, err := factory.Build(map[string]any{"min_market_cap_usd": float64(-1)}); !errors.Is(err, analysis.ErrInvalidArgument) {
		t.Fatalf("err=%v", err)
	}
	value := 100.0
	store := &storeStub{instrument: market.Instrument{ID: 1, Symbol: "BTCUSDT", BaseAsset: "BTC", QuoteAsset: "USDT", MarketCapUSD: &value}}
	service, _ := analysis.NewService(store, factory)
	result, err := service.AnalyzeSymbol(context.Background(), analysis.SymbolRequest{Symbol: "BTCUSDT", Criteria: []analysis.CriterionConfig{{Key: "market_cap", Name: "market_cap", Label: "Market Cap", Parameters: map[string]any{"min_market_cap_usd": float64(100)}}}})
	if err != nil || !result.Matched || result.Evaluations[0].Metrics["market_cap_usd"] != 100 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestMissingPersistedCapIsUnresolved(t *testing.T) {
	criterion, err := New().Build(map[string]any{"min_market_cap_usd": float64(0)})
	if err != nil {
		t.Fatal(err)
	}
	_, err = criterion.Evaluate(context.Background(), analysis.Input{Instrument: market.Instrument{BaseAsset: "MISSING"}})
	var unresolved *analysis.UnresolvedError
	if !errors.As(err, &unresolved) || unresolved.Code != "market_cap_missing" {
		t.Fatalf("error=%v", err)
	}
}

type storeStub struct{ instrument market.Instrument }

func (s *storeStub) GetSyncState(context.Context, market.SyncProfile) (market.SyncState, error) {
	return market.SyncState{}, nil
}
func (s *storeStub) ListActiveInstruments(context.Context) ([]market.Instrument, error) {
	return []market.Instrument{s.instrument}, nil
}
func (s *storeStub) SelectActiveInstruments(_ context.Context, selection analysis.Selection) ([]market.Instrument, error) {
	if selection.Symbol != "" && selection.Symbol != s.instrument.Symbol {
		return nil, nil
	}
	return []market.Instrument{s.instrument}, nil
}
func (s *storeStub) ListLatestCandlesByInterval(context.Context, int64, string, int) ([]market.Candle, error) {
	return nil, nil
}
func (s *storeStub) ListHourlyPrices(context.Context, []int64, time.Time, time.Time) ([]market.HourlyPrice, error) {
	return nil, nil
}
