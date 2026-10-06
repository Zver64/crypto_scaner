package closedindicator_test

import (
	"context"
	"log/slog"
	"math"
	"testing"
	"time"

	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/closedindicator"
	"crypto-scanner/internal/indicator"
	indicatortalib "crypto-scanner/internal/indicator/talib"
	"crypto-scanner/internal/market"
)

// settledTolerance is the relative difference a settled recursive value may
// keep from a value over a longer history: the slowest ones, such as EMA 200,
// keep a remainder of their seed below display precision.
const settledTolerance = 1e-5

// historyStore serves one instrument's kept history to the tracker and to
// charts.
type historyStore struct{ candles []market.Candle }

func (store historyStore) ListLatestCandles(_ context.Context, ids []int64, _ market.CandleInterval, limit int) (map[int64][]market.Candle, error) {
	result := map[int64][]market.Candle{}
	for _, id := range ids {
		result[id] = store.candles[max(0, len(store.candles)-limit):]
	}
	return result, nil
}

func (store historyStore) GetActiveInstrumentBySymbol(_ context.Context, symbol string) (market.Instrument, error) {
	return market.Instrument{ID: 1, Symbol: symbol, Active: true}, nil
}

func (store historyStore) ListCandlePage(_ context.Context, _ int64, _ market.CandleInterval, _ *time.Time, limit int) (market.CandlePage, error) {
	split := max(0, len(store.candles)-limit)
	return market.CandlePage{Candles: store.candles[split:], HasMore: split > 0}, nil
}

type noSubscriptions struct{}

func (noSubscriptions) Subscriptions(context.Context) ([]closedindicator.Subscription, error) {
	return nil, nil
}

type emptyCatalog struct{}

func (emptyCatalog) ChartCatalog(market.CandleInterval) []chart.CatalogIndicator { return nil }

// An EMA 200 over its lookback alone is its seed SMA; the tracker lets it
// settle to the value of a much longer history.
func TestTrackerSettlesRecursiveIndicators(t *testing.T) {
	registry := testRegistry(t)
	long := testHistory(3 * market.HistoryDepth)
	target := closedindicator.Target{Interval: market.IntervalHour, Selection: selection(t, registry, "ema", 200)}
	reference, err := registry.CalculateCandles(market.IntervalHour, long, []indicator.Selection{target.Selection})
	if err != nil {
		t.Fatal(err)
	}
	points := reference[0].Series[0].Points
	want := points[len(points)-1].Value

	tracker := newTracker(t, registry, long[len(long)-market.HistoryDepth:], target)
	got := tracker.Latest(context.Background(), []int64{1})[1][0]
	if len(got.Outputs) != 1 || !settled(got.Outputs[0].Value, want) {
		t.Fatalf("tracked EMA 200 = %+v, want %v from the long history", got.Outputs, want)
	}
}

// The tracker reads the last closed point of the chart, whatever else the
// chart shows and however many points a strategy keeps.
func TestTrackerAgreesWithChartsAndPoints(t *testing.T) {
	registry := testRegistry(t)
	history := testHistory(market.HistoryDepth)
	targets := []closedindicator.Target{
		{Interval: market.IntervalHour, Selection: selection(t, registry, "ema", 200)},
		{Interval: market.IntervalHour, Selection: selection(t, registry, "rsi", 14)},
		{Interval: market.IntervalHour, Selection: selection(t, registry, "macd", 0)},
	}
	service, err := chart.NewService(historyStore{history}, registry, emptyCatalog{}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	selections := make([]indicator.Selection, len(targets))
	for index, target := range targets {
		selections[index] = target.Selection
	}
	page, err := service.Build(context.Background(), chart.Request{Symbol: "BTCUSDT", Interval: market.IntervalHour, Limit: chart.DefaultRange, Indicators: selections})
	if err != nil {
		t.Fatal(err)
	}
	tracked := newTracker(t, registry, history, targets...).Latest(context.Background(), []int64{1})[1]
	for index, target := range targets {
		// Drawn alone, the indicator has the same points.
		alone, err := service.Build(context.Background(), chart.Request{Symbol: "BTCUSDT", Interval: market.IntervalHour, Limit: chart.DefaultRange, Indicators: selections[index : index+1]})
		if err != nil {
			t.Fatal(err)
		}
		depth, err := closedindicator.Depth(registry, target, 300)
		if err != nil {
			t.Fatal(err)
		}
		deep, err := closedindicator.Calculate(registry, target, history[len(history)-depth:], 300)
		if err != nil {
			t.Fatal(err)
		}
		for outputIndex, series := range page.Indicators[index].Series {
			want := series.Points[len(series.Points)-1].Value
			aloneSeries := alone.Indicators[0].Series[outputIndex]
			if got := aloneSeries.Points[len(aloneSeries.Points)-1].Value; got != want {
				t.Fatalf("%s %s drawn alone = %v, with others %v", target.Selection.Type, series.Name, got, want)
			}
			if got := tracked[index].Outputs[outputIndex].Value; !settled(got, want) {
				t.Fatalf("%s %s tracked = %v, chart %v", target.Selection.Type, series.Name, got, want)
			}
			if got := deep.Outputs[outputIndex].Value; !settled(got, want) {
				t.Fatalf("%s %s with 300 points = %v, chart %v", target.Selection.Type, series.Name, got, want)
			}
		}
	}
}

func newTracker(t *testing.T, registry *indicator.Registry, history []market.Candle, targets ...closedindicator.Target) *closedindicator.Tracker {
	t.Helper()
	tracker, err := closedindicator.New(historyStore{history}, registry, noSubscriptions{}, targets, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	return tracker
}

func testRegistry(t *testing.T) *indicator.Registry {
	t.Helper()
	registry, err := indicator.NewRegistry(indicatortalib.New()...)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// selection returns the canonical selection of kind; a zero period keeps the
// defaults.
func selection(t *testing.T, registry *indicator.Registry, kind indicator.Type, period int) indicator.Selection {
	t.Helper()
	parameters := indicator.Parameters{}
	if period > 0 {
		parameters["period"] = period
	}
	normalized, err := registry.Normalize(indicator.Selection{Type: kind, Parameters: parameters})
	if err != nil {
		t.Fatal(err)
	}
	return normalized
}

// testHistory is a continuous hourly history whose trend turns, so seeds
// differ clearly from settled values.
func testHistory(count int) []market.Candle {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := make([]market.Candle, count)
	for index := range candles {
		open := start.Add(time.Duration(index) * time.Hour)
		price := 100 + 30*math.Sin(float64(index)/150) + 5*math.Sin(float64(index)/7)
		candles[index] = market.Candle{
			InstrumentID: 1, Interval: market.IntervalHour, OpenTime: open, CloseTime: open.Add(time.Hour - time.Millisecond),
			Open: price - 1, High: price + 2, Low: price - 2, Close: price, Volume: 10,
		}
	}
	return candles
}

func settled(got, want float64) bool {
	return math.Abs(got-want) <= settledTolerance*math.Max(1, math.Abs(want))
}
