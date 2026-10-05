package strategy

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"slices"
	"strconv"
	"testing"
	"time"

	"crypto-scanner/internal/closedindicator"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/indicator/candle"
	indicatortalib "crypto-scanner/internal/indicator/talib"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/scannerindicator"
)

var backtestStart = time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)

func TestBacktestReplaysAlertTransitions(t *testing.T) {
	hour := func(index int) time.Time { return backtestStart.Add(time.Duration(index) * time.Hour) }
	// Hour 5 is missing, so prev is unknown at hour 6.
	closes := map[int]float64{0: 6, 1: 7, 2: 8, 3: 2, 4: 9, 6: 9, 7: 9, 8: 1, 9: 9}
	var hourly []market.Candle
	for index := range 10 {
		if value, ok := closes[index]; ok {
			hourly = append(hourly, testCandle(1, market.IntervalHour, hour(index), value))
		}
	}
	store := newBacktestStore(market.IntervalHour, hourly)
	service := newBacktestService(t, store, Strategy{ID: 1, Name: "Breakout", Expression: "h_close > 5 && prev(h_close) > 0"})

	got, err := service.Backtest(context.Background(), 1, "BTCUSDT")
	if err != nil {
		t.Fatal(err)
	}
	// Hour 0 is unknown, hours 2 and 7 keep matching, and hour 6 is unknown
	// while matching.
	want := Backtest{Interval: market.IntervalHour, From: hour(0), To: hour(9), Alerts: []time.Time{hour(1), hour(4), hour(9)}}
	if !backtestEqual(got, want) {
		t.Fatalf("Backtest() = %+v, want %+v", got, want)
	}
}

// A full kept history is pruned, so its oldest candles lack the warm-up the
// tracker loads and are not evaluated.
func TestBacktestSkipsCandlesBeforeTheWarmUpOfAPrunedHistory(t *testing.T) {
	hour := func(index int) time.Time { return backtestStart.Add(time.Duration(index) * time.Hour) }
	hourly := make([]market.Candle, market.HistoryDepth)
	for index := range hourly {
		hourly[index] = testCandle(1, market.IntervalHour, hour(index), 9)
	}
	service := newBacktestService(t, newBacktestStore(market.IntervalHour, hourly), Strategy{ID: 1, Name: "Close", Expression: "h_close > 5"})

	got, err := service.Backtest(context.Background(), 1, "BTCUSDT")
	if err != nil {
		t.Fatal(err)
	}
	depth, err := closedindicator.Depth(testRegistry(t), CandleTarget(market.IntervalHour), 1)
	if err != nil {
		t.Fatal(err)
	}
	first := hour(depth - 1)
	want := Backtest{Interval: market.IntervalHour, From: first, To: hour(market.HistoryDepth - 1), Alerts: []time.Time{first}}
	if !backtestEqual(got, want) {
		t.Fatalf("Backtest() = %+v, want %+v", got, want)
	}
}

// ETHUSDT has history but is not among the administrator's favorites, so the
// values read through of stay unknown and nothing alerts.
func TestBacktestLeavesCoinsOutsideTheFavoritesUnknown(t *testing.T) {
	var hourly, other []market.Candle
	for index := range 5 {
		open := backtestStart.Add(time.Duration(index) * time.Hour)
		hourly = append(hourly, testCandle(1, market.IntervalHour, open, 9))
		other = append(other, testCandle(2, market.IntervalHour, open, 9))
	}
	store := newBacktestStore(market.IntervalHour, hourly)
	store.candles[market.IntervalHour][2] = other
	service := newBacktestService(t, store, Strategy{ID: 1, Name: "Pair", Expression: `h_close > 5 && of("ETHUSDT", h_close) > 5`})

	got, err := service.Backtest(context.Background(), 1, "BTCUSDT")
	if err != nil {
		t.Fatal(err)
	}
	want := Backtest{Interval: market.IntervalHour, From: backtestStart, To: backtestStart.Add(4 * time.Hour)}
	if !backtestEqual(got, want) {
		t.Fatalf("Backtest() = %+v, want %+v", got, want)
	}
}

func TestBacktestWithoutHistoryEvaluatesNothing(t *testing.T) {
	service := newBacktestService(t, newBacktestStore(market.IntervalDay, nil), Strategy{ID: 1, Name: "Daily", Expression: "d_close > 5"})

	got, err := service.Backtest(context.Background(), 1, "BTCUSDT")
	if err != nil {
		t.Fatal(err)
	}
	if want := (Backtest{Interval: market.IntervalDay}); !backtestEqual(got, want) {
		t.Fatalf("Backtest() = %+v, want %+v", got, want)
	}
}

func TestBacktestReadsDailyValuesOnlyAfterTheDayCloses(t *testing.T) {
	day := backtestStart
	daily := []market.Candle{testCandle(1, market.IntervalDay, day.AddDate(0, 0, -1), 100), testCandle(1, market.IntervalDay, day, 10)}
	var hourly []market.Candle
	for index := range 30 {
		hourly = append(hourly, testCandle(1, market.IntervalHour, day.Add(time.Duration(index)*time.Hour), 50))
	}
	store := newBacktestStore(market.IntervalHour, hourly)
	store.candles[market.IntervalDay] = map[int64][]market.Candle{1: daily}
	service := newBacktestService(t, store, Strategy{ID: 1, Name: "Daily", Expression: "h_close > d_close"})

	got, err := service.Backtest(context.Background(), 1, "BTCUSDT")
	if err != nil {
		t.Fatal(err)
	}
	// The hourly candle opened at 23:00 closes with the day.
	want := Backtest{Interval: market.IntervalHour, From: day, To: day.Add(29 * time.Hour), Alerts: []time.Time{day.Add(23 * time.Hour)}}
	if !backtestEqual(got, want) {
		t.Fatalf("Backtest() = %+v, want %+v", got, want)
	}
}

func TestBacktestRejectsUnknownAndInvalidStrategiesAndSymbols(t *testing.T) {
	store := newBacktestStore(market.IntervalHour, nil)
	service := newBacktestService(t, store, Strategy{ID: 1, Name: "Broken", Expression: "h_unknown > 1"})
	for _, test := range []struct {
		id     int64
		symbol string
		want   error
	}{
		{id: 2, symbol: "BTCUSDT", want: ErrNotFound},
		{id: 1, symbol: "BTCUSDT", want: ErrInvalidArgument},
	} {
		if _, err := service.Backtest(context.Background(), test.id, test.symbol); !errors.Is(err, test.want) {
			t.Fatalf("Backtest(%d, %s) error = %v, want %v", test.id, test.symbol, err, test.want)
		}
	}
	valid := newBacktestService(t, store, Strategy{ID: 1, Name: "Valid", Expression: "h_close > 1"})
	if _, err := valid.Backtest(context.Background(), 1, "ETHUSDT"); !errors.Is(err, market.ErrInstrumentNotFound) {
		t.Fatalf("Backtest(ETHUSDT) error = %v, want %v", err, market.ErrInstrumentNotFound)
	}
}

// The replay at the latest candle reads what the tracker and the monitor
// read on the same history.
func TestBacktestReplayMatchesTrackedValuesAtTheLatestCandle(t *testing.T) {
	registry := testRegistry(t)
	indicators := testIndicators{entries: []scannerindicator.Entry{
		testEMA(t, registry, 1, market.IntervalHour, 100),
		testEMA(t, registry, 2, market.IntervalDay, 3),
	}}
	var hourly, other, daily []market.Candle
	for index := range 300 {
		open := backtestStart.Add(time.Duration(index) * time.Hour)
		hourly = append(hourly, testCandle(1, market.IntervalHour, open, 100+10*math.Sin(float64(index)/7)))
		other = append(other, testCandle(2, market.IntervalHour, open, 50+float64(index%13)))
	}
	for index := range 30 {
		daily = append(daily, testCandle(1, market.IntervalDay, backtestStart.AddDate(0, 0, index-18), 100+float64(index*index%17)))
	}
	store := newBacktestStore(market.IntervalHour, hourly)
	store.candles[market.IntervalHour][2] = other
	store.candles[market.IntervalDay] = map[int64][]market.Candle{1: daily}
	instruments := []Instrument{{ID: 1, Symbol: "BTCUSDT"}, {ID: 2, Symbol: "ETHUSDT"}}
	compiled, err := Compile(`h_ema_100 > prev(h_ema_100) && d_ema_3 < d_close && of("ETHUSDT", h_close) > 0`, Variables(indicators.entries))
	if err != nil {
		t.Fatal(err)
	}
	entry := Entry{Strategy: Strategy{ID: 1, Enabled: true}, Compiled: compiled}
	now := backtestStart.Add(300 * time.Hour)

	replay, err := newReplay(registry, entry, instruments[0], instruments)
	if err != nil {
		t.Fatal(err)
	}
	replay.histories = store.candles
	replayed, warm, err := replay.at(now)
	if err != nil || !warm {
		t.Fatalf("at() warm = %v, error = %v", warm, err)
	}

	tracked := snapshot{reads: readsOf([]Entry{entry}, instruments, instruments), bySymbol: bySymbol(instruments), now: now}
	tracker, err := closedindicator.New(store, registry, nil, slog.New(slog.DiscardHandler), subscriptionSource(tracked.reads.subscriptions))
	if err != nil {
		t.Fatal(err)
	}
	changed := make(chan struct{}, 1)
	tracker.Listen(func(closedindicator.Change) {
		select {
		case changed <- struct{}{}:
		default:
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- tracker.Run(ctx) }()
	defer func() { cancel(); <-done }()
	deadline := time.After(10 * time.Second)
	for {
		var missing []closedindicator.Target
		if tracked.values, missing = tracker.Snapshot(tracked.reads.subscriptions); len(missing) == 0 {
			break
		}
		select {
		case <-changed:
		case <-deadline:
			t.Fatalf("tracker values missing: %v", missing)
		}
	}

	for _, read := range compiled.Reads() {
		want, wantOK := tracked.output(entry, read, 1)
		got, gotOK := replayed.output(entry, read, 1)
		if !wantOK || !gotOK {
			t.Fatalf("%s: tracked fresh = %v, replayed fresh = %v", read.Variable.Name, wantOK, gotOK)
		}
		wantValue, wantKnown := want.At(read.Shift)
		gotValue, gotKnown := got.At(read.Shift)
		if wantValue != gotValue || wantKnown != gotKnown || !wantKnown {
			t.Fatalf("%s shift %d of %q = %v (%v), tracked %v (%v)", read.Variable.Name, read.Shift, read.Symbol, gotValue, gotKnown, wantValue, wantKnown)
		}
	}
	wantResult, wantKnown := tracked.match(entry, 1)
	gotResult, gotKnown := replayed.match(entry, 1)
	if gotResult != wantResult || gotKnown != wantKnown || !wantKnown {
		t.Fatalf("replayed match = %v (%v), tracked %v (%v)", gotResult, gotKnown, wantResult, wantKnown)
	}
}

func backtestEqual(left, right Backtest) bool {
	return left.Interval == right.Interval && left.From.Equal(right.From) && left.To.Equal(right.To) && slices.EqualFunc(left.Alerts, right.Alerts, time.Time.Equal)
}

func testCandle(instrumentID int64, interval market.CandleInterval, open time.Time, close float64) market.Candle {
	return market.Candle{
		InstrumentID: instrumentID, Interval: interval, OpenTime: open, CloseTime: interval.NextOpenTime(open).Add(-time.Millisecond),
		Open: close, High: close + 1, Low: close - 1, Close: close, Volume: 10,
	}
}

func testRegistry(t *testing.T) *indicator.Registry {
	t.Helper()
	registry, err := indicator.NewRegistry(append(indicatortalib.New(), candle.New())...)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func testEMA(t *testing.T, registry *indicator.Registry, id int64, interval market.CandleInterval, period int) scannerindicator.Entry {
	t.Helper()
	selection, err := registry.Normalize(indicator.Selection{Type: "ema", Parameters: indicator.Parameters{"period": period}})
	if err != nil {
		t.Fatal(err)
	}
	title := scannerindicator.IntervalPrefix(interval) + "-ema-" + strconv.Itoa(period)
	return scannerindicator.Entry{Indicator: scannerindicator.Indicator{ID: id, Interval: interval, Selection: selection}, Title: title, Outputs: []string{"ema"}}
}

func newBacktestService(t *testing.T, store *backtestStore, items ...Strategy) *Service {
	t.Helper()
	store.strategies = items
	service, err := NewService(store, testIndicators{}, testRegistry(t), 7, slog.New(slog.DiscardHandler), func([]int64) {})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	return service
}

type testIndicators struct{ entries []scannerindicator.Entry }

func (indicators testIndicators) List() []scannerindicator.Entry { return indicators.entries }
func (testIndicators) Preview(scannerindicator.Indicator) (scannerindicator.Entry, error) {
	return scannerindicator.Entry{}, errors.New("not configured")
}

type subscriptionSource []closedindicator.Subscription

func (source subscriptionSource) Subscriptions(context.Context, []closedindicator.Target) ([]closedindicator.Subscription, error) {
	return source, nil
}

// backtestStore holds BTCUSDT (1), the administrator's only favorite, and
// ETHUSDT (2), which is not active. Methods the backtest does not use panic.
type backtestStore struct {
	Store
	strategies []Strategy
	candles    map[market.CandleInterval]map[int64][]market.Candle
}

func newBacktestStore(interval market.CandleInterval, candles []market.Candle) *backtestStore {
	return &backtestStore{candles: map[market.CandleInterval]map[int64][]market.Candle{interval: {1: candles}}}
}

func (store *backtestStore) ListStrategies(context.Context) ([]Strategy, error) {
	return store.strategies, nil
}

func (store *backtestStore) ListStrategyInstruments(context.Context, int64) ([]Instrument, error) {
	return []Instrument{{ID: 1, Symbol: "BTCUSDT"}}, nil
}

func (store *backtestStore) GetActiveInstrumentBySymbol(_ context.Context, symbol string) (market.Instrument, error) {
	if symbol != "BTCUSDT" {
		return market.Instrument{}, market.ErrInstrumentNotFound
	}
	return market.Instrument{ID: 1, Symbol: symbol, Active: true}, nil
}

func (store *backtestStore) ListLatestCandles(_ context.Context, ids []int64, interval market.CandleInterval, limit int) (map[int64][]market.Candle, error) {
	result := map[int64][]market.Candle{}
	for _, id := range ids {
		if candles := store.candles[interval][id]; len(candles) > 0 {
			result[id] = candles[max(0, len(candles)-limit):]
		}
	}
	return result, nil
}
