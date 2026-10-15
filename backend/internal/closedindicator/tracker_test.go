package closedindicator_test

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/closedindicator"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/indicator/candle"
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

type emptyCatalog struct{}

func (emptyCatalog) ChartCatalog(market.CandleInterval) []chart.CatalogIndicator { return nil }

// An EMA 200 over its lookback alone is its seed SMA; the tracker lets it
// settle to the value of a much longer history.
func TestTrackerSettlesRecursiveIndicators(t *testing.T) {
	registry := testRegistry(t)
	long := testHistory(3 * market.SyncDepth)
	target := closedindicator.Target{Interval: market.IntervalHour, Selection: selection(t, registry, "ema", 200)}
	reference, err := registry.CalculateCandles(market.IntervalHour, long, []indicator.Selection{target.Selection})
	if err != nil {
		t.Fatal(err)
	}
	points := reference[0].Series[0].Points
	want := points[len(points)-1].Value

	tracker := newTracker(t, registry, long[len(long)-market.SyncDepth:], target)
	got := tracker.Latest(context.Background(), []int64{1})[1][0]
	if len(got.Outputs) != 1 || !settled(got.Outputs[0].Value, want) {
		t.Fatalf("tracked EMA 200 = %+v, want %v from the long history", got.Outputs, want)
	}
}

// The tracker reads the last closed point of the chart, whatever else the
// chart shows and however many points a strategy keeps.
func TestTrackerAgreesWithChartsAndPoints(t *testing.T) {
	registry := testRegistry(t)
	history := testHistory(market.SyncDepth)
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
	tracker, err := closedindicator.New(historyStore{history}, registry, targets, slog.New(slog.DiscardHandler))
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

// A synchronization round recalculates only its own interval, so changes of
// another interval still being synchronized wait for that round.
func TestTrackerRecalculatesOnlyTheSyncedInterval(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		registry, err := indicator.NewRegistry(candle.New())
		if err != nil {
			t.Fatal(err)
		}
		target := func(interval market.CandleInterval) closedindicator.Target {
			return closedindicator.Target{Interval: interval, Selection: indicator.Selection{Type: candle.Type, Parameters: indicator.Parameters{}}}
		}
		store := &intervalStore{candles: map[market.CandleInterval][]market.Candle{}}
		bar := func(interval market.CandleInterval, open time.Time) market.Candle {
			return market.Candle{InstrumentID: 1, Interval: interval, OpenTime: open, CloseTime: interval.NextOpenTime(open).Add(-time.Millisecond), Open: 1, High: 2, Low: 1, Close: 2}
		}
		start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		for _, interval := range []market.CandleInterval{market.IntervalHour, market.IntervalDay} {
			store.add(bar(interval, start))
		}
		tracker, err := closedindicator.New(store, registry, nil, slog.New(slog.DiscardHandler))
		if err != nil {
			t.Fatal(err)
		}
		tracker.Subscribe("test", []closedindicator.Subscription{{InstrumentID: 1, Target: target(market.IntervalHour)}, {InstrumentID: 1, Target: target(market.IntervalDay)}})
		var batches [][]market.CandleInterval
		tracker.Listen(func(changes []closedindicator.Change) {
			var intervals []market.CandleInterval
			for _, change := range changes {
				intervals = append(intervals, change.Current.Target.Interval)
			}
			batches = append(batches, intervals)
		})
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error)
		go func() { done <- tracker.Run(ctx) }()
		synctest.Wait()

		next := []market.Candle{bar(market.IntervalHour, start.Add(time.Hour)), bar(market.IntervalDay, start.AddDate(0, 0, 1))}
		for _, item := range next {
			store.add(item)
		}
		tracker.HistoryChanged(next)
		tracker.HistorySynced(market.IntervalHour)
		synctest.Wait()
		tracker.HistorySynced(market.IntervalDay)
		synctest.Wait()
		cancel()
		<-done
		want := []market.CandleInterval{market.IntervalHour, market.IntervalDay}
		if len(batches) != 3 || !slices.Equal(batches[1], want[:1]) || !slices.Equal(batches[2], want[1:]) {
			t.Fatalf("batches = %v, want the initial one, then 1h, then 1d", batches)
		}
	})
}

// intervalStore serves the candles of each interval.
type intervalStore struct {
	mu      sync.Mutex
	candles map[market.CandleInterval][]market.Candle
	reads   int
	// reading, when set, receives each read before it completes, which
	// waits for release; failures fail that many reads.
	reading  chan struct{}
	release  chan struct{}
	failures int
}

func (store *intervalStore) readCount() int {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.reads
}

func (store *intervalStore) add(item market.Candle) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.candles[item.Interval] = append(store.candles[item.Interval], item)
}

func (store *intervalStore) ListLatestCandles(_ context.Context, ids []int64, interval market.CandleInterval, limit int) (map[int64][]market.Candle, error) {
	store.mu.Lock()
	store.reads++
	candles := slices.Clone(store.candles[interval])
	failed := store.failures > 0
	if failed {
		store.failures--
	}
	store.mu.Unlock()
	if failed {
		return nil, errors.New("database unavailable")
	}
	if store.reading != nil {
		store.reading <- struct{}{}
		<-store.release
	}
	return map[int64][]market.Candle{1: candles[max(0, len(candles)-limit):]}, nil
}

// Pairs stay current while any subscriber keeps them; a pair nobody keeps is
// dropped.
func TestTrackerKeepsThePairsOfEverySubscriber(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tracker, _, pair, _ := newWindowTracker(t)
		defer tracker.stop()
		tracked := func() bool {
			synctest.Wait()
			_, missing := tracker.Snapshot([]closedindicator.Subscription{pair})
			return len(missing) == 0
		}
		tracker.Subscribe("first", []closedindicator.Subscription{pair})
		tracker.Subscribe("second", []closedindicator.Subscription{pair})
		if !tracked() {
			t.Fatal("subscribed pair is not tracked")
		}
		tracker.Subscribe("first", nil)
		if !tracked() {
			t.Fatal("a pair another subscriber keeps was dropped")
		}
		tracker.Subscribe("second", nil)
		if tracked() {
			t.Fatal("a pair nobody keeps is still tracked")
		}
	})
}

// A failed load leaves the window unloaded, without a timer: the next round
// of its interval loads it, though no window of the interval is ready.
func TestTrackerLoadsAFailedWindowOnTheNextStep(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tracker, store, pair, latest := newWindowTracker(t)
		defer tracker.stop()
		store.failures = 1
		tracker.Subscribe("test", []closedindicator.Subscription{pair})
		if open, _ := latest(); !open.IsZero() || store.readCount() != 1 {
			t.Fatalf("after the failure: %s, reads %d", open, store.readCount())
		}
		time.Sleep(time.Hour)
		if latest(); store.readCount() != 1 {
			t.Fatalf("reads without a step = %d, want 1", store.readCount())
		}
		tracker.HistorySynced(market.IntervalHour)
		if open, close := latest(); !open.Equal(windowStart) || close != 2 || store.readCount() != 2 {
			t.Fatalf("after the next round: %s close %v, reads %d", open, close, store.readCount())
		}
	})
}

// A failed calculation keeps its histories dirty, without a timer: the next
// round of their interval recalculates them.
func TestTrackerRecalculatesAFailedCalculationOnTheNextRound(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fail := &atomic.Bool{}
		registry, err := indicator.NewRegistry(failingModule{candle.New(), fail})
		if err != nil {
			t.Fatal(err)
		}
		tracker, store, pair, latest := runWindowTracker(t, registry)
		defer tracker.stop()
		tracker.Subscribe("test", []closedindicator.Subscription{pair})
		latest()
		next := windowStart.Add(time.Hour)
		store.add(windowBar(next, 3))
		fail.Store(true)
		tracker.HistoryChanged([]market.Candle{windowBar(next, 3)})
		tracker.HistorySynced(market.IntervalHour)
		if open, _ := latest(); !open.Equal(windowStart) {
			t.Fatalf("after the failure: %s", open)
		}
		fail.Store(false)
		time.Sleep(time.Hour)
		if open, _ := latest(); !open.Equal(windowStart) {
			t.Fatalf("recalculated without a round: %s", open)
		}
		tracker.HistorySynced(market.IntervalHour)
		if open, close := latest(); !open.Equal(next) || close != 3 || store.readCount() != 1 {
			t.Fatalf("after the next round: %s close %v, reads %d", open, close, store.readCount())
		}
	})
}

// failingModule fails every calculation while fail is set.
type failingModule struct {
	indicator.Implementation
	fail *atomic.Bool
}

func (module failingModule) Calculate(parameters indicator.Parameters, inputs indicator.Inputs) (indicator.Result, error) {
	if module.fail.Load() {
		return indicator.Result{}, errors.New("calculation failed")
	}
	return module.Implementation.Calculate(parameters, inputs)
}

// A pair whose calculation fails leaves the other pairs of its step alone.
func TestTrackerReportsThePairsThatCalculate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		registry, err := indicator.NewRegistry(candle.New(), brokenModule{candle.New()})
		if err != nil {
			t.Fatal(err)
		}
		tracker, _, pair, _ := runWindowTracker(t, registry)
		defer tracker.stop()
		broken := pair
		broken.Target.Selection.Type = brokenType
		var reported []indicator.Type
		tracker.Listen(func(changes []closedindicator.Change) {
			for _, change := range changes {
				reported = append(reported, change.Current.Target.Selection.Type)
			}
		})
		tracker.Subscribe("test", []closedindicator.Subscription{pair, broken})
		synctest.Wait()
		if !slices.Equal(reported, []indicator.Type{candle.Type}) {
			t.Fatalf("reported %v, want only the candle pair", reported)
		}
		if _, missing := tracker.Snapshot([]closedindicator.Subscription{pair, broken}); len(missing) != 1 || missing[0].Selection.Type != brokenType {
			t.Fatalf("missing %v, want the broken pair", missing)
		}
	})
}

const brokenType indicator.Type = "broken"

// brokenModule is the candle module under brokenType, whose calculations
// always fail.
type brokenModule struct{ indicator.Implementation }

func (module brokenModule) Describe() indicator.Descriptor {
	descriptor := module.Implementation.Describe()
	descriptor.Type = brokenType
	return descriptor
}

func (brokenModule) Calculate(indicator.Parameters, indicator.Inputs) (indicator.Result, error) {
	return indicator.Result{}, errors.New("calculation failed")
}

// A step calculates the pairs of a synchronized round and those whose
// windows it loaded together, so the listener receives them in one call.
func TestTrackerReportsAStepInOneCall(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		registry, err := indicator.NewRegistry(candle.New())
		if err != nil {
			t.Fatal(err)
		}
		store := &intervalStore{candles: map[market.CandleInterval][]market.Candle{}}
		bar := func(interval market.CandleInterval, open time.Time) market.Candle {
			return market.Candle{InstrumentID: 1, Interval: interval, OpenTime: open, CloseTime: interval.NextOpenTime(open).Add(-time.Millisecond), Open: 1, High: 2, Low: 1, Close: 2}
		}
		subscription := func(interval market.CandleInterval) closedindicator.Subscription {
			return closedindicator.Subscription{InstrumentID: 1, Target: closedindicator.Target{Interval: interval, Selection: indicator.Selection{Type: candle.Type, Parameters: indicator.Parameters{}}}}
		}
		start := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
		for _, interval := range []market.CandleInterval{market.IntervalHour, market.IntervalDay, market.IntervalWeek} {
			store.add(bar(interval, start))
		}
		tracker, err := closedindicator.New(store, registry, nil, slog.New(slog.DiscardHandler))
		if err != nil {
			t.Fatal(err)
		}
		var batches [][]market.CandleInterval
		tracker.Listen(func(changes []closedindicator.Change) {
			var intervals []market.CandleInterval
			for _, change := range changes {
				intervals = append(intervals, change.Current.Target.Interval)
			}
			slices.Sort(intervals)
			batches = append(batches, intervals)
		})
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error)
		go func() { done <- tracker.Run(ctx) }()
		hour, day, week := subscription(market.IntervalHour), subscription(market.IntervalDay), subscription(market.IntervalWeek)
		tracker.Subscribe("test", []closedindicator.Subscription{hour})
		synctest.Wait()

		// The load of the day window holds its step, while a round of the
		// hour and a new week pair arrive for the next one.
		store.reading, store.release = make(chan struct{}, 2), make(chan struct{})
		tracker.Subscribe("test", []closedindicator.Subscription{hour, day})
		synctest.Wait()
		next := bar(market.IntervalHour, start.Add(time.Hour))
		store.add(next)
		tracker.HistoryChanged([]market.Candle{next})
		tracker.HistorySynced(market.IntervalHour)
		tracker.Subscribe("test", []closedindicator.Subscription{hour, day, week})
		close(store.release)
		synctest.Wait()
		cancel()
		<-done
		want := [][]market.CandleInterval{{market.IntervalHour}, {market.IntervalDay}, {market.IntervalHour, market.IntervalWeek}}
		if !slices.EqualFunc(batches, want, slices.Equal) {
			t.Fatalf("batches = %v, want %v", batches, want)
		}
	})
}

// The window of a tracked history is read once; later candles and
// corrections update it without reading the store, and only a deeper
// subscription reads it again.
func TestTrackerKeepsWindowsCurrentWithoutTheStore(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tracker, store, pair, latest := newWindowTracker(t)
		defer tracker.stop()
		changed := func(candle market.Candle) {
			store.add(candle)
			tracker.HistoryChanged([]market.Candle{candle})
			tracker.HistorySynced(market.IntervalHour)
		}

		tracker.Subscribe("test", []closedindicator.Subscription{pair})
		if open, close := latest(); !open.Equal(windowStart) || close != 2 || store.readCount() != 1 {
			t.Fatalf("after subscribing: %s close %v, reads %d", open, close, store.readCount())
		}
		next := windowStart.Add(time.Hour)
		changed(windowBar(next, 3))
		if open, close := latest(); !open.Equal(next) || close != 3 || store.readCount() != 1 {
			t.Fatalf("after a candle: %s close %v, reads %d", open, close, store.readCount())
		}
		changed(windowBar(next, 4))
		if open, close := latest(); !open.Equal(next) || close != 4 || store.readCount() != 1 {
			t.Fatalf("after a correction: %s close %v, reads %d", open, close, store.readCount())
		}
		pair.Points = 3
		tracker.Subscribe("test", []closedindicator.Subscription{pair})
		if open, close := latest(); !open.Equal(next) || close != 4 || store.readCount() != 2 {
			t.Fatalf("after a deeper subscription: %s close %v, reads %d", open, close, store.readCount())
		}
	})
}

// A candle committed while a window loads, which the read may miss, still
// reaches the window.
func TestTrackerKeepsChangesCommittedWhileAWindowLoads(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tracker, store, pair, latest := newWindowTracker(t)
		defer tracker.stop()
		store.reading, store.release = make(chan struct{}), make(chan struct{})
		tracker.Subscribe("test", []closedindicator.Subscription{pair})
		<-store.reading
		next := windowStart.Add(time.Hour)
		// The read has taken its candles, so it misses this one.
		store.add(windowBar(next, 3))
		tracker.HistoryChanged([]market.Candle{windowBar(next, 3)})
		close(store.release)
		if open, value := latest(); !open.Equal(next) || value != 3 || store.readCount() != 1 {
			t.Fatalf("after the load: %s close %v, reads %d", open, value, store.readCount())
		}
	})
}

var windowStart = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func windowBar(open time.Time, close float64) market.Candle {
	return market.Candle{InstrumentID: 1, Interval: market.IntervalHour, OpenTime: open, CloseTime: open.Add(time.Hour - time.Millisecond), Open: 1, High: 9, Low: 1, Close: close}
}

// runningTracker is a tracker whose Run stops with stop.
type runningTracker struct {
	*closedindicator.Tracker
	stop func()
}

// newWindowTracker runs a tracker over a store holding one hourly candle and
// returns a pair of its candles and a reader of the pair's latest close.
func newWindowTracker(t *testing.T) (runningTracker, *intervalStore, closedindicator.Subscription, func() (time.Time, float64)) {
	t.Helper()
	registry, err := indicator.NewRegistry(candle.New())
	if err != nil {
		t.Fatal(err)
	}
	return runWindowTracker(t, registry)
}

// runWindowTracker is newWindowTracker over registry, which serves the
// candle fields.
func runWindowTracker(t *testing.T, registry *indicator.Registry) (runningTracker, *intervalStore, closedindicator.Subscription, func() (time.Time, float64)) {
	t.Helper()
	store := &intervalStore{candles: map[market.CandleInterval][]market.Candle{}}
	store.add(windowBar(windowStart, 2))
	tracker, err := closedindicator.New(store, registry, nil, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	pair := closedindicator.Subscription{InstrumentID: 1, Target: closedindicator.Target{Interval: market.IntervalHour, Selection: indicator.Selection{Type: candle.Type, Parameters: indicator.Parameters{}}}, Points: 2}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() { done <- tracker.Run(ctx) }()
	latest := func() (time.Time, float64) {
		synctest.Wait()
		values, _ := tracker.Snapshot([]closedindicator.Subscription{pair})
		for _, output := range values[0].Outputs {
			if output.Name == "close" {
				return values[0].OpenTime, output.Value
			}
		}
		return values[0].OpenTime, 0
	}
	return runningTracker{tracker, func() { cancel(); <-done }}, store, pair, latest
}
