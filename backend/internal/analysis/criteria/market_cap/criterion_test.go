package market_cap

import (
	"context"
	"errors"
	"testing"
	"time"

	"crypto-scanner/internal/analysis"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/marketcap"
)

func TestValidationAndInclusiveBoundary(t *testing.T) {
	store := &storeStub{mapping: marketcap.Mapping{BaseAsset: "BTC", QuoteAsset: "USDT", CoinID: "bitcoin", Status: "resolved"}, cap: marketcap.Cap{CoinID: "bitcoin", USD: 100, Available: true, FetchedAt: time.Now()}}
	factory := New(marketcap.New(store, providerStub{}))
	if _, err := factory.Build(map[string]any{"min_market_cap_usd": float64(-1)}); !errors.Is(err, analysis.ErrInvalidArgument) {
		t.Fatalf("err=%v", err)
	}
	service, _ := analysis.NewService(store, factory)
	result, err := service.AnalyzeSymbol(context.Background(), analysis.SymbolRequest{Symbol: "BTCUSDT", Criteria: []analysis.CriterionConfig{{Key: "market_cap", Name: "market_cap", Label: "Market Cap", Parameters: map[string]any{"min_market_cap_usd": float64(100)}}}})
	if err != nil || !result.Matched {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

type storeStub struct {
	mapping marketcap.Mapping
	cap     marketcap.Cap
}

func (s *storeStub) BootstrapCompleted(context.Context) (bool, error)           { return true, nil }
func (s *storeStub) ReplaceSnapshot(context.Context, []marketcap.Mapping) error { return nil }
func (s *storeStub) GetMapping(context.Context, string) (marketcap.Mapping, error) {
	return s.mapping, nil
}
func (s *storeStub) SaveMapping(context.Context, marketcap.Mapping) error  { return nil }
func (s *storeStub) GetCap(context.Context, string) (marketcap.Cap, error) { return s.cap, nil }
func (s *storeStub) SaveCap(context.Context, marketcap.Cap) error          { return nil }
func (s *storeStub) GetSyncState(context.Context, market.SyncProfile) (market.SyncState, error) {
	return market.SyncState{}, nil
}
func (s *storeStub) ListActiveInstruments(context.Context) ([]market.Instrument, error) {
	return []market.Instrument{{ID: 1, Symbol: "BTCUSDT", BaseAsset: "BTC", QuoteAsset: "USDT"}}, nil
}
func (s *storeStub) ListLatestCandlesByInterval(context.Context, int64, string, int) ([]market.Candle, error) {
	return nil, nil
}
func (s *storeStub) ListHourlyPrices(context.Context, []int64, time.Time, time.Time) ([]market.HourlyPrice, error) {
	return nil, nil
}

type providerStub struct{}

func (providerStub) Tickers(context.Context, int) ([]marketcap.Ticker, error)   { return nil, nil }
func (providerStub) Markets(context.Context, []string) ([]marketcap.Cap, error) { return nil, nil }
