package chart_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"testing"
	"time"

	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/chart/draw"
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

// emptyCatalog is a chart catalog without indicators; requests carry their own
// selections.
type emptyCatalog struct{}

func (emptyCatalog) ChartCatalog(market.CandleInterval) []chart.CatalogIndicator { return nil }

func TestServiceUsesOnlyRequestedClosedRange(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := testCandles(start, 200)
	store := &storeStub{page: market.CandlePage{Candles: candles, HasMore: true}}
	service := newTestService(t, store, emptyCatalog{})
	page, err := service.Build(context.Background(), chart.Request{
		Symbol: "btcusdt", Interval: market.IntervalHour, Limit: 200,
		Indicators: []indicator.Selection{{Type: indicatortalib.RSIType, Parameters: indicator.Parameters{"period": 14}}},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	warmup := rsiWindow(t) - 1
	if store.resolveCalls != 1 || store.listCalls != 1 || store.limit != 200+warmup || store.before != nil {
		t.Fatalf("store resolve calls=%d list calls=%d limit=%d before=%v, want one lookup and one latest candle read of the range plus RSI warm-up", store.resolveCalls, store.listCalls, store.limit, store.before)
	}
	if page.Symbol != "BTCUSDT" {
		t.Fatalf("page symbol = %q, want BTCUSDT", page.Symbol)
	}
	if len(page.Candles) != 200 || !page.Candles[0].OpenTime.Equal(start) || !page.HasMore || page.NextBefore == nil || !page.NextBefore.Equal(page.Candles[0].OpenTime) {
		t.Fatalf("page boundaries = candles %d, first %v, hasMore %v, next %v", len(page.Candles), page.Candles[0].OpenTime, page.HasMore, page.NextBefore)
	}
	points := rsiPoints(t, page)
	if len(points) != 186 || !points[0].Time.Equal(start.Add(14*time.Hour)) {
		t.Fatalf("RSI points = %d starting %v, want 186 points after warmup", len(points), points[0].Time)
	}

	// Extending the range recalculates every point; the store has no older
	// candles here, so the new range starts without warm-up history.
	store.page = market.CandlePage{Candles: testCandles(start.Add(-200*time.Hour), 400)}
	extended, err := service.Build(context.Background(), chart.Request{
		Symbol: "BTCUSDT", Interval: market.IntervalHour, Limit: 400,
		Indicators: []indicator.Selection{{Type: indicatortalib.RSIType, Parameters: indicator.Parameters{"period": 14}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	extendedPoints := rsiPoints(t, extended)
	if store.limit != 400+warmup || len(extended.Candles) != 400 || len(extendedPoints) != 386 ||
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
			service := newTestService(t, store, emptyCatalog{})
			page, err := service.Build(context.Background(), chart.Request{
				Symbol: "BTCUSDT", Interval: market.IntervalDay, Limit: 200,
				Indicators: []indicator.Selection{{Type: indicatortalib.RSIType, Parameters: indicator.Parameters{"period": 14}}},
			})
			if err != nil {
				t.Fatalf("Build() error = %v", err)
			}
			if len(page.Candles) != count || len(rsiPoints(t, page)) != 0 {
				t.Fatalf("page has %d candles and %d points, want %d and 0", len(page.Candles), len(rsiPoints(t, page)), count)
			}
		})
	}
}

func TestServiceRejectsExcessIndicatorsBeforeStorageRead(t *testing.T) {
	store := &storeStub{}
	service := newTestService(t, store, emptyCatalog{})
	configs := make([]indicator.Selection, 9)
	for index := range configs {
		configs[index] = indicator.Selection{Type: indicatortalib.RSIType, Parameters: indicator.Parameters{"period": 14}}
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
	service := newTestService(t, store, emptyCatalog{})
	_, err := service.Build(context.Background(), chart.Request{
		Symbol: "BTCUSDT", Interval: market.IntervalHour, Limit: 200,
		Indicators: []indicator.Selection{{Type: indicatortalib.RSIType, Parameters: indicator.Parameters{"period": 2001}}},
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
	service := newTestService(t, store, emptyCatalog{})
	page, err := service.Build(context.Background(), chart.Request{
		Symbol: "BTCUSDT", Interval: market.IntervalHour, Limit: 200,
		Indicators: []indicator.Selection{{Type: indicatortalib.RSIType, Parameters: indicator.Parameters{"period": float64(14)}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	points := rsiPoints(t, page)
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
	service := newTestService(t, store, emptyCatalog{})
	page, err := service.Build(context.Background(), chart.Request{
		Symbol: "BTCUSDT", Interval: market.IntervalHour, Limit: 200,
		Indicators: []indicator.Selection{{Type: indicatortalib.RSIType, Parameters: indicator.Parameters{"period": 14}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	points := rsiPoints(t, page)
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

// newTestService draws with the default drawer and the shared palette.
func newTestService(t *testing.T, store chart.Store, catalog chart.CatalogSource) *chart.Service {
	t.Helper()
	registry, err := indicator.NewRegistry(indicatortalib.New()...)
	if err != nil {
		t.Fatal(err)
	}
	service, err := chart.NewService(store, registry, testDrawers(t, registry), catalog, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func testDrawers(t *testing.T, registry *indicator.Registry) *chart.Drawers {
	t.Helper()
	drawers, err := chart.NewDrawers(registry, chart.Palette{Colors: []string{"yellow.5", "blue.5"}, Bullish: "green.6", Bearish: "red.6"}, draw.Default{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return drawers
}

// rsiPoints are the points of the line of the RSI, the first indicator of
// page.
func rsiPoints(t *testing.T, page chart.Page) []chart.LinePoint {
	t.Helper()
	index := slices.IndexFunc(page.Shapes, func(shape chart.Shape) bool { return shape.ID == "0/rsi" })
	if index < 0 {
		t.Fatalf("no RSI line among %d shapes", len(page.Shapes))
	}
	return page.Shapes[index].Line.Points
}
