package chart_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/indicator"
	indicatortalib "crypto-scanner/internal/indicator/talib"
	"crypto-scanner/internal/market"
)

type storeStub struct {
	page         market.CandlePage
	resolveCalls int
	listCalls    int
	before       *time.Time
	limit        int
}

func (store *storeStub) GetActiveInstrumentBySymbol(_ context.Context, symbol string) (market.Instrument, error) {
	store.resolveCalls++
	return market.Instrument{ID: 1, Symbol: symbol, Active: true}, nil
}

func (store *storeStub) ListCandlePage(_ context.Context, _ int64, _ market.CandleInterval, before *time.Time, limit int) (market.CandlePage, error) {
	store.listCalls++
	store.before = before
	store.limit = limit
	return store.page, nil
}

func TestServiceUsesOnlyRequestedClosedRange(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := testCandles(start, 200)
	store := &storeStub{page: market.CandlePage{Candles: candles, HasMore: true}}
	registry, err := indicator.NewRegistry(indicatortalib.NewRSI())
	if err != nil {
		t.Fatal(err)
	}
	service, err := chart.NewService(store, registry)
	if err != nil {
		t.Fatal(err)
	}
	cursor := start.Add(-time.Hour)
	page, err := service.Build(context.Background(), chart.Request{
		Symbol: "btcusdt", Interval: market.IntervalHour, Before: &cursor, Limit: 200,
		Indicators: []chart.IndicatorConfig{{Type: indicatortalib.RSIType, Parameters: indicator.Parameters{"period": 14}}},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if store.resolveCalls != 1 || store.listCalls != 1 || store.limit != 200 || store.before != &cursor {
		t.Fatalf("store resolve calls=%d list calls=%d limit=%d before=%p, want one lookup and one candle read with limit 200 and cursor", store.resolveCalls, store.listCalls, store.limit, store.before)
	}
	if page.Symbol != "BTCUSDT" {
		t.Fatalf("page symbol = %q, want BTCUSDT", page.Symbol)
	}
	if len(page.Candles) != 200 || !page.Candles[0].OpenTime.Equal(start) || !page.HasMore || page.NextBefore == nil || !page.NextBefore.Equal(page.Candles[0].OpenTime) {
		t.Fatalf("page boundaries = candles %d, first %v, hasMore %v, next %v", len(page.Candles), page.Candles[0].OpenTime, page.HasMore, page.NextBefore)
	}
	points := page.Indicators[0].Series[0].Points
	if len(points) != 186 || !points[0].Time.Equal(start.Add(14*time.Hour)) {
		t.Fatalf("RSI points = %d starting %v, want 186 points after warmup", len(points), points[0].Time)
	}

	// Extending the same chart range changes the warmup and recalculates
	// every point, including those already present in the initial range.
	store.page = market.CandlePage{Candles: testCandles(start.Add(-200*time.Hour), 400)}
	extended, err := service.Build(context.Background(), chart.Request{
		Symbol: "BTCUSDT", Interval: market.IntervalHour, Limit: 400,
		Indicators: []chart.IndicatorConfig{{Type: indicatortalib.RSIType, Parameters: indicator.Parameters{"period": 14}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	extendedPoints := extended.Indicators[0].Series[0].Points
	if store.limit != 400 || len(extended.Candles) != 400 || len(extendedPoints) != 386 ||
		!extendedPoints[0].Time.Equal(start.Add(-186*time.Hour)) ||
		!extendedPoints[len(extendedPoints)-1].Time.Equal(points[len(points)-1].Time) {
		t.Fatalf("extended range: limit=%d candles=%d points=%d", store.limit, len(extended.Candles), len(extendedPoints))
	}
}

func TestServiceAllowsHistoryShorterThanRSIWarmup(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, count := range []int{0, 1, 13} {
		t.Run(fmt.Sprintf("%d candles", count), func(t *testing.T) {
			store := &storeStub{page: market.CandlePage{Candles: testCandles(start, count)}}
			registry, _ := indicator.NewRegistry(indicatortalib.NewRSI())
			service, _ := chart.NewService(store, registry)
			page, err := service.Build(context.Background(), chart.Request{
				Symbol: "BTCUSDT", Interval: market.IntervalDay, Limit: 200,
				Indicators: []chart.IndicatorConfig{{Type: indicatortalib.RSIType, Parameters: indicator.Parameters{"period": 14}}},
			})
			if err != nil {
				t.Fatalf("Build() error = %v", err)
			}
			if len(page.Candles) != count || len(page.Indicators[0].Series[0].Points) != 0 {
				t.Fatalf("page has %d candles and %d points, want %d and 0", len(page.Candles), len(page.Indicators[0].Series[0].Points), count)
			}
		})
	}
}

func TestServiceRejectsExcessIndicatorsBeforeStorageRead(t *testing.T) {
	store := &storeStub{}
	registry, _ := indicator.NewRegistry(indicatortalib.NewRSI())
	service, _ := chart.NewService(store, registry)
	configs := make([]chart.IndicatorConfig, 9)
	for index := range configs {
		configs[index] = chart.IndicatorConfig{Type: indicatortalib.RSIType, Parameters: indicator.Parameters{"period": 14}}
	}
	_, err := service.Build(context.Background(), chart.Request{
		Symbol: "BTCUSDT", Interval: market.IntervalHour, Limit: 200, Indicators: configs,
	})
	if !errors.Is(err, chart.ErrInvalidRequest) {
		t.Fatalf("Build() error = %v, want ErrInvalidRequest", err)
	}
	if store.resolveCalls != 0 || store.listCalls != 0 {
		t.Fatalf("store calls = resolve %d, list %d; want 0, 0", store.resolveCalls, store.listCalls)
	}
}

func TestServiceRejectsExcessLookbackBeforeStorageRead(t *testing.T) {
	store := &storeStub{}
	registry, _ := indicator.NewRegistry(indicatortalib.NewRSI())
	service, _ := chart.NewService(store, registry)
	_, err := service.Build(context.Background(), chart.Request{
		Symbol: "BTCUSDT", Interval: market.IntervalHour, Limit: 200,
		Indicators: []chart.IndicatorConfig{{Type: indicatortalib.RSIType, Parameters: indicator.Parameters{"period": 501}}},
	})
	if !errors.Is(err, chart.ErrInvalidRequest) {
		t.Fatalf("Build() error = %v, want ErrInvalidRequest", err)
	}
	if store.resolveCalls != 0 || store.listCalls != 0 {
		t.Fatalf("store calls = resolve %d, list %d; want 0, 0", store.resolveCalls, store.listCalls)
	}
}

func TestServiceAlignsShortHistoryAfterWarmup(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	store := &storeStub{page: market.CandlePage{Candles: testCandles(start, 18)}}
	registry, _ := indicator.NewRegistry(indicatortalib.NewRSI())
	service, _ := chart.NewService(store, registry)
	page, err := service.Build(context.Background(), chart.Request{
		Symbol: "BTCUSDT", Interval: market.IntervalHour, Limit: 200,
		Indicators: []chart.IndicatorConfig{{Type: indicatortalib.RSIType, Parameters: indicator.Parameters{"period": float64(14)}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	points := page.Indicators[0].Series[0].Points
	if len(page.Candles) != 18 || page.HasMore || page.NextBefore != nil || len(points) != 4 {
		t.Fatalf("page has %d candles, %d points, hasMore=%v, next=%v", len(page.Candles), len(points), page.HasMore, page.NextBefore)
	}
	if !points[0].Time.Equal(start.Add(14 * time.Hour)) {
		t.Fatalf("first point time = %v, want candle 14", points[0].Time)
	}
}

func TestServiceRestartsWarmupAfterHistoryGap(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := testCandles(start, 40)
	// Lose one closed candle in the middle of the returned range.
	candles = append(candles[:20:20], candles[21:]...)
	store := &storeStub{page: market.CandlePage{Candles: candles}}
	registry, _ := indicator.NewRegistry(indicatortalib.NewRSI())
	service, _ := chart.NewService(store, registry)
	page, err := service.Build(context.Background(), chart.Request{
		Symbol: "BTCUSDT", Interval: market.IntervalHour, Limit: 200,
		Indicators: []chart.IndicatorConfig{{Type: indicatortalib.RSIType, Parameters: indicator.Parameters{"period": 14}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	points := page.Indicators[0].Series[0].Points
	if len(points) != 11 || !points[5].Time.Equal(start.Add(19*time.Hour)) || !points[6].Time.Equal(start.Add(35*time.Hour)) {
		t.Fatalf("RSI across a gap: %+v", points)
	}
}

func testCandles(start time.Time, count int) []market.Candle {
	candles := make([]market.Candle, count)
	for index := range candles {
		open := start.Add(time.Duration(index) * time.Hour)
		close := 100 + float64((index%9)-4) + float64(index)/10
		candles[index] = market.Candle{OpenTime: open, CloseTime: open.Add(time.Hour - time.Millisecond), Open: close - 1, High: close + 1, Low: close - 2, Close: close}
	}
	return candles
}
