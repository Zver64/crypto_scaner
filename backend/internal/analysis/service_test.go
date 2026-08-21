package analysis_test

import (
	"context"
	"math"
	"testing"
	"time"

	"crypto-scanner/internal/analysis"
	"crypto-scanner/internal/analysis/percentile"
	"crypto-scanner/internal/market"
)

func TestServiceAnalyzesOneActiveSymbolFromSynchronizedCandles(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.July, 5, 0, 0, 0, 0, time.UTC)
	store := analysisStoreStub{
		synchronized: true,
		instruments:  []market.Instrument{{ID: 7, Symbol: "BTCUSDT", Active: true}},
		candles: map[int64][]market.Candle{
			7: {analysisCandle(start, 2), analysisCandle(start.AddDate(0, 0, 1), 6)},
		},
	}

	result, err := analysis.NewService(store, percentile.New()).AnalyzeSymbol(context.Background(), analysis.SymbolRequest{
		Symbol: "BTCUSDT", Unit: analysis.UnitDays, Period: 2, Percentile: 50,
	})
	if err != nil {
		t.Fatalf("AnalyzeSymbol() error = %v", err)
	}
	if result.Symbol != "BTCUSDT" || result.RangePercent != 4 || result.CandleCount != 2 {
		t.Fatalf("AnalyzeSymbol() result = %+v", result)
	}
	if !result.From.Equal(start) || !result.To.Equal(start.AddDate(0, 0, 1)) {
		t.Fatalf("AnalyzeSymbol() coverage = %s..%s", result.From, result.To)
	}
}

func TestServiceSearchesMarketUsingUnroundedThresholdAndDeterministicOrder(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.July, 5, 0, 0, 0, 0, time.UTC)
	store := analysisStoreStub{
		synchronized: true,
		instruments: []market.Instrument{
			{ID: 1, Symbol: "ZZZUSDT", Active: true},
			{ID: 2, Symbol: "AAAUSDT", Active: true},
			{ID: 3, Symbol: "NEWUSDT", Active: true},
			{ID: 4, Symbol: "LOWUSDT", Active: true},
		},
		candles: map[int64][]market.Candle{
			1: {analysisCandle(start, 4), analysisCandle(start.AddDate(0, 0, 1), 4)},
			2: {analysisCandle(start, 4), analysisCandle(start.AddDate(0, 0, 1), 4)},
			3: {analysisCandle(start, 9)},
			4: {analysisCandle(start, 3.00001), analysisCandle(start.AddDate(0, 0, 1), 3.00001)},
		},
	}

	result, err := analysis.NewService(store, percentile.New()).Search(context.Background(), analysis.SearchRequest{
		Unit: analysis.UnitDays, Period: 2, Percentile: 75, MinimumRangePercent: 3.000005,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if result.AnalyzedCount != 3 || result.InsufficientDataCount != 1 || result.MatchedCount != 3 {
		t.Fatalf("Search() counts = analyzed %d, insufficient %d, matched %d", result.AnalyzedCount, result.InsufficientDataCount, result.MatchedCount)
	}
	wantSymbols := []string{"AAAUSDT", "ZZZUSDT", "LOWUSDT"}
	for index, want := range wantSymbols {
		if result.Items[index].Symbol != want {
			t.Fatalf("Search() item %d symbol = %q, want %q; items = %+v", index, result.Items[index].Symbol, want, result.Items)
		}
	}
	if math.Abs(result.Items[2].RangePercent-3.00001) > 1e-10 {
		t.Fatalf("threshold item range = %.8f, want unrounded 3.00001", result.Items[2].RangePercent)
	}
}

func TestConcurrentAnalysisRequestsProceedIndependently(t *testing.T) {
	t.Parallel()

	store := &concurrentAnalysisStore{entered: make(chan struct{}, 2), release: make(chan struct{})}
	service := analysis.NewService(store, percentile.New())
	results := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := service.AnalyzeSymbol(context.Background(), analysis.SymbolRequest{
				Symbol: "BTCUSDT", Unit: analysis.UnitDays, Period: 1, Percentile: 50,
			})
			results <- err
		}()
	}
	for range 2 {
		select {
		case <-store.entered:
		case <-time.After(time.Second):
			t.Fatal("analysis requests were serialized by an application-wide lock")
		}
	}
	close(store.release)
	for range 2 {
		if err := <-results; err != nil {
			t.Fatalf("AnalyzeSymbol() error = %v", err)
		}
	}
}

func TestServiceAnalyzesHourlyPeriodUsingHourlyCandles(t *testing.T) {
	start := time.Date(2026, time.July, 5, 0, 0, 0, 0, time.UTC)
	candles := make([]market.Candle, 60)
	for index := range candles {
		candles[index] = analysisCandle(start.Add(time.Duration(index)*time.Hour), float64(index+1))
	}
	store := analysisStoreStub{
		synchronized: true,
		instruments:  []market.Instrument{{ID: 7, Symbol: "BTCUSDT", Active: true}},
		candles:      map[int64][]market.Candle{7: candles},
	}

	result, err := analysis.NewService(store, percentile.New()).AnalyzeSymbol(context.Background(), analysis.SymbolRequest{
		Symbol: "BTCUSDT", Unit: analysis.UnitHours, Period: 60, Percentile: 50,
	})
	if err != nil {
		t.Fatalf("AnalyzeSymbol() error = %v", err)
	}
	if result.CandleCount != 60 || !result.From.Equal(start) || !result.To.Equal(start.Add(59*time.Hour)) {
		t.Fatalf("hourly result = %+v, want 60 candles spanning one hour intervals", result)
	}
}

type analysisStoreStub struct {
	synchronized bool
	instruments  []market.Instrument
	candles      map[int64][]market.Candle
}

func (stub analysisStoreStub) GetSyncState(context.Context, market.SyncProfile) (market.SyncState, error) {
	state := market.SyncState{}
	if stub.synchronized {
		succeededAt := time.Date(2026, time.August, 5, 0, 0, 0, 0, time.UTC)
		state.LastSucceededAt = &succeededAt
	}
	return state, nil
}
func (stub analysisStoreStub) ListActiveInstruments(context.Context) ([]market.Instrument, error) {
	return append([]market.Instrument(nil), stub.instruments...), nil
}
func (stub analysisStoreStub) ListLatestCandlesByInterval(_ context.Context, instrumentID int64, _ string, limit int) ([]market.Candle, error) {
	items := append([]market.Candle(nil), stub.candles[instrumentID]...)
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func analysisCandle(openTime time.Time, rangePercent float64) market.Candle {
	return market.Candle{OpenTime: openTime, Open: 100, High: 100 + rangePercent, Low: 100}
}

type concurrentAnalysisStore struct {
	entered chan struct{}
	release chan struct{}
}

func (*concurrentAnalysisStore) GetSyncState(context.Context, market.SyncProfile) (market.SyncState, error) {
	succeededAt := time.Date(2026, time.August, 5, 0, 0, 0, 0, time.UTC)
	return market.SyncState{LastSucceededAt: &succeededAt}, nil
}
func (*concurrentAnalysisStore) ListActiveInstruments(context.Context) ([]market.Instrument, error) {
	return []market.Instrument{{ID: 1, Symbol: "BTCUSDT", Active: true}}, nil
}
func (store *concurrentAnalysisStore) ListLatestCandlesByInterval(context.Context, int64, string, int) ([]market.Candle, error) {
	store.entered <- struct{}{}
	<-store.release
	return []market.Candle{analysisCandle(time.Date(2026, time.August, 4, 0, 0, 0, 0, time.UTC), 3)}, nil
}
