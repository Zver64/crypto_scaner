package strategy

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"crypto-scanner/internal/closedindicator"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/telegramhtml"
)

// Events of one round share a title per kind, followed by the message and a
// line per coin; a message too long for Telegram ends at a whole line.
func TestAlertTextGroupsEventsByKind(t *testing.T) {
	entry := Entry{Strategy: Strategy{Name: "Trend", Direction: DirectionLong, Message: "Check volume"}, Interval: market.IntervalHour}
	buy := func(symbol string, number int) alert {
		return alert{Instrument{Symbol: symbol}, TradeEvent{Kind: TradeBuy, Buy: number, Close: 1.5, Trade: TradeState{TakeProfit: 2, StopLoss: 1}}}
	}
	sell := alert{Instrument{Symbol: "SOLUSDT"}, TradeEvent{Kind: TradeSell, Reason: ExitTakeProfit, Price: 152, Return: 0.038, Trade: TradeState{Buys: 2, Filled: 2, Quantity: 0.01}}}
	got := alertText(entry, []alert{buy("BTCUSDT", 1), sell, buy("ETHUSDT", 2)})
	want := "<b>🟢 Trend · 1h · buy</b>\nCheck volume\n<a href=\"https://www.binance.com/en/trade/BTC_USDT?type=spot\">BTCUSDT</a> at 1.5 · TP 2 · SL 1\n<a href=\"https://www.binance.com/en/trade/ETH_USDT?type=spot\">ETHUSDT</a> #2 at 1.5 · TP 2 · SL 1\n\n" +
		"<b>🔴 Trend · 1h · sell</b>\nCheck volume\n<a href=\"https://www.binance.com/en/trade/SOL_USDT?type=spot\">SOLUSDT</a> at 152 by take profit · +3.80% · entry 200 · 2 buys"
	if got != want {
		t.Fatalf("alertText() = %q, want %q", got, want)
	}

	many := make([]alert, 200)
	for i := range many {
		many[i] = buy("BTCUSDT", 1)
	}
	got = alertText(entry, many)
	if len(got) > telegramhtml.MaxLength || !strings.HasSuffix(got, "SL 1\n…message too long") {
		t.Fatalf("long alertText() ends %q after %d bytes", got[max(0, len(got)-40):], len(got))
	}
}

// A halt disables only the revision that failed: a strategy changed since
// stays enabled.
func TestDisableAtRevisionKeepsAChangedStrategy(t *testing.T) {
	store := &disablingStore{backtestStore: newBacktestStore(market.IntervalHour, nil)}
	service := newBacktestService(t, store.backtestStore, Strategy{ID: 1, Name: "Hold", Direction: DirectionLong, Expression: "h_close > 5", Enabled: true, Revision: 3})
	service.store = store
	if disabled, err := service.DisableAtRevision(context.Background(), 1, 2); err != nil || disabled || store.calls != 0 || !service.List()[0].Enabled {
		t.Fatalf("stale revision: disabled %v, error %v, store calls %d", disabled, err, store.calls)
	}
	if disabled, err := service.DisableAtRevision(context.Background(), 1, 3); err != nil || !disabled || store.calls != 1 || service.List()[0].Enabled || service.List()[0].Revision != 4 {
		t.Fatalf("current revision: disabled %v, error %v, entry %+v", disabled, err, service.List()[0])
	}
}

type disablingStore struct {
	*backtestStore
	calls int
}

func (store *disablingStore) DisableStrategyAtRevision(_ context.Context, _, revision int64) (int64, bool, error) {
	store.calls++
	return revision + 1, true, nil
}

// Each strategy subscribes only the coins it evaluates, and a signal of all
// coins every active one.
func TestReadsOfSubscribesTheCoinsOfEachStrategy(t *testing.T) {
	service := newBacktestService(t, newBacktestStore(market.IntervalHour, nil),
		Strategy{ID: 1, Name: "Hourly", Expression: "h_close > 1", Coins: []string{"ETHUSDT", "LUNAUSDT"}},
		Strategy{ID: 2, Name: "Daily", Signal: true, Expression: "d_close > 1", CoinSource: CoinsAll},
	)
	active := []Instrument{{ID: 1, Symbol: "BTCUSDT"}, {ID: 2, Symbol: "ETHUSDT"}, {ID: 3, Symbol: "SOLUSDT"}}
	subscribed := map[market.CandleInterval][]int64{}
	for _, subscription := range readsOf(service.List(), Entry.instruments, active).subscriptions {
		subscribed[subscription.Target.Interval] = append(subscribed[subscription.Target.Interval], subscription.InstrumentID)
	}
	if want := map[market.CandleInterval][]int64{market.IntervalHour: {2}, market.IntervalDay: {1, 2, 3}}; !reflect.DeepEqual(subscribed, want) {
		t.Fatalf("subscribed = %v, want %v", subscribed, want)
	}
}

// Changing the coins of an enabled strategy starts it afresh, a rename keeps
// its trades, and a coin that is not active rejects the change.
func TestUpdateStartsAfreshOnlyWhenTheCoinsChange(t *testing.T) {
	store := &updatingStore{backtestStore: newBacktestStore(market.IntervalHour, nil)}
	var baselines []int64
	service := newBacktestService(t, store.backtestStore, Strategy{ID: 1, Name: "Hold", Direction: DirectionLong, Expression: "h_close > 5", Coins: []string{"ETHUSDT"}, CoinSource: CoinsSelected, Enabled: true})
	service.store, service.changed = store, func(ids []int64) { baselines = ids }
	service.InstrumentsChanged([]market.Instrument{{ID: 1, Symbol: "BTCUSDT"}, {ID: 2, Symbol: "ETHUSDT"}})
	for _, test := range []struct {
		coins   []string
		retrade bool
	}{{[]string{"ETHUSDT"}, false}, {[]string{"btcusdt"}, true}} {
		item := service.List()[0].Strategy
		item.Name, item.Coins = "Renamed", test.coins
		if _, err := service.Update(context.Background(), item); err != nil {
			t.Fatal(err)
		}
		if store.retrade != test.retrade || store.baseline != test.retrade || (len(baselines) > 0) != test.retrade {
			t.Fatalf("coins %v: retrade %v, baseline %v, baselines %v", test.coins, store.retrade, store.baseline, baselines)
		}
	}
	item := service.List()[0].Strategy
	item.Coins = []string{"LUNAUSDT"}
	if _, err := service.Update(context.Background(), item); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("inactive coin: %v", err)
	}
}

type updatingStore struct {
	*backtestStore
	retrade, baseline bool
}

func (store *updatingStore) UpdateStrategy(_ context.Context, _ Strategy, _ []int64, retrade, baseline bool) (int64, error) {
	store.retrade, store.baseline = retrade, baseline
	return 2, nil
}

// A coin a strategy evaluates or reads through of leaving the active coins
// disables the strategy and tells the administrator.
func TestMonitorHaltsStrategiesOfInactiveCoins(t *testing.T) {
	for _, item := range []Strategy{
		{ID: 1, Name: "Chosen", Expression: "h_close > 5", Coins: []string{"BTCUSDT", "LUNAUSDT"}, Enabled: true, Revision: 3},
		{ID: 1, Name: "Read", Expression: `h_close > 5 && of("LUNAUSDT", h_close) > 1`, Coins: []string{"BTCUSDT"}, Enabled: true, Revision: 3},
	} {
		disabling := &disablingStore{backtestStore: newBacktestStore(market.IntervalHour, nil)}
		service := newBacktestService(t, disabling.backtestStore, item)
		service.store = disabling
		monitor := NewMonitor(monitorStore{disabling.backtestStore}, noValues{}, service, nil, 7, slog.New(slog.DiscardHandler))
		monitor.evaluate(context.Background(), map[int64]map[int64]TradeState{})
		if disabling.calls != 1 || service.List()[0].Enabled {
			t.Fatalf("%s: disable calls %d, entry %+v", item.Name, disabling.calls, service.List()[0])
		}
		if sent := <-monitor.sends; sent.telegramID != 7 || sent.text != "<b>⛔ "+item.Name+" · 1h · disabled</b>\nnot an active coin: LUNAUSDT" {
			t.Fatalf("%s: sent %+v", item.Name, sent)
		}
	}
}

type monitorStore struct{ *backtestStore }

func (monitorStore) ListStrategyStates(context.Context) (map[int64]map[int64]TradeState, error) {
	return nil, nil
}

func (monitorStore) ListStrategyRecipients(context.Context, int64) ([]int64, error) {
	return []int64{7}, nil
}

func (monitorStore) ReplaceStrategyStates(context.Context, int64, int64, map[int64]TradeState) (bool, error) {
	panic("unexpected baseline")
}

func (monitorStore) SaveStrategyStates(context.Context, int64, int64, map[int64]TradeState) ([]int64, error) {
	panic("unexpected states")
}

func (monitorStore) DeleteStrategyStates(context.Context, int64, []int64) error { return nil }

type noValues struct{}

func (noValues) Listen(func([]closedindicator.Change)) {}

func (noValues) Snapshot(subscriptions []closedindicator.Subscription) ([]closedindicator.Value, []closedindicator.Target) {
	return make([]closedindicator.Value, len(subscriptions)), nil
}

// A signal of the favorites starts a coin new to them silently and alerts
// on coins it already evaluated.
func TestMonitorStartsCoinsNewToTheFavoritesSilently(t *testing.T) {
	store := newBacktestStore(market.IntervalHour, nil)
	store.favorites = []string{"BTCUSDT"}
	service := newBacktestService(t, store, Strategy{ID: 1, Name: "Watch", Signal: true, Direction: DirectionLong, Expression: "h_close > 1", CoinSource: CoinsFavorites, Enabled: true, Revision: 1})
	for _, test := range []struct {
		states map[int64]TradeState
		alert  bool
	}{{map[int64]TradeState{}, false}, {map[int64]TradeState{1: {}}, true}} {
		saving := &savingStore{monitorStore: monitorStore{store}}
		monitor := NewMonitor(saving, closingValues{}, service, nil, 7, slog.New(slog.DiscardHandler))
		monitor.full = true
		monitor.evaluate(context.Background(), map[int64]map[int64]TradeState{1: test.states})
		if !reflect.DeepEqual(saving.saved, []int64{1}) || (len(monitor.sends) > 0) != test.alert {
			t.Fatalf("states %v: saved %v, %d alerts", test.states, saving.saved, len(monitor.sends))
		}
	}
}

type savingStore struct {
	monitorStore
	saved []int64
}

func (store *savingStore) SaveStrategyStates(_ context.Context, _, _ int64, states map[int64]TradeState) ([]int64, error) {
	for id := range states {
		store.saved = append(store.saved, id)
	}
	return store.saved, nil
}

// closingValues reports every value fresh, each output 2.
type closingValues struct{}

func (closingValues) Listen(func([]closedindicator.Change)) {}

func (closingValues) Snapshot(subscriptions []closedindicator.Subscription) ([]closedindicator.Value, []closedindicator.Target) {
	values := make([]closedindicator.Value, len(subscriptions))
	for i, subscription := range subscriptions {
		values[i] = closedindicator.Value{Target: subscription.Target, OpenTime: subscription.Target.Interval.LastClosedOpenTime(time.Now())}
		for _, name := range []string{"open", "high", "low", "close"} {
			values[i].Outputs = append(values[i].Outputs, closedindicator.Output{Name: name, Value: 2})
		}
	}
	return values, nil
}

// A failed round is retried after a delay instead of at once.
func TestMonitorRetriesAFailedRoundLater(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service := newBacktestService(t, newBacktestStore(market.IntervalHour, nil), Strategy{ID: 1, Name: "Hold", Expression: "h_close > 5", Coins: []string{"BTCUSDT"}, Enabled: true})
		store := &failingStore{monitorStore: monitorStore{newBacktestStore(market.IntervalHour, nil)}}
		monitor := NewMonitor(store, noValues{}, service, nil, 7, slog.New(slog.DiscardHandler))
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error)
		go func() { done <- monitor.Run(ctx) }()
		synctest.Wait()
		if store.calls.Load() != 1 {
			t.Fatalf("calls after start = %d, want 1", store.calls.Load())
		}
		// Changes wait for the retry too.
		monitor.Changed()
		time.Sleep(retryDelay - time.Second)
		synctest.Wait()
		if store.calls.Load() != 1 {
			t.Fatalf("calls before the retry = %d, want 1", store.calls.Load())
		}
		time.Sleep(time.Second)
		synctest.Wait()
		if store.calls.Load() != 2 {
			t.Fatalf("calls after the retry = %d, want 2", store.calls.Load())
		}
		cancel()
		<-done
	})
}

type failingStore struct {
	monitorStore
	calls atomic.Int32
}

func (store *failingStore) ListStrategyInstruments(context.Context) ([]Instrument, error) {
	store.calls.Add(1)
	return nil, errors.New("database unavailable")
}

// The service subscribes the tracker to what its enabled strategies and
// signals read, and again after any change of them, of the catalog, or of the
// favorites.
func TestServiceSubscribesTheTrackerToWhatItsStrategiesRead(t *testing.T) {
	store := newBacktestStore(market.IntervalHour, nil)
	store.strategies = []Strategy{
		{ID: 1, Name: "Hourly", Expression: "h_close > 1", Coins: []string{"BTCUSDT"}, Enabled: true},
		{ID: 2, Name: "Daily", Signal: true, Expression: "d_close > 1", CoinSource: CoinsAll, Enabled: true},
		{ID: 3, Name: "Off", Expression: "w_close > 1", Coins: []string{"BTCUSDT"}},
		{ID: 4, Name: "Weekly", Signal: true, Expression: "w_close > 1", CoinSource: CoinsFavorites, Enabled: true},
	}
	store.favorites = []string{"ETHUSDT"}
	tracker := &recordingTracker{}
	service, err := NewService(store, testIndicators{}, testRegistry(t), tracker, slog.New(slog.DiscardHandler), 0, func([]int64) {})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	// The store lists only BTCUSDT as active.
	want := map[market.CandleInterval][]int64{market.IntervalHour: {1}, market.IntervalDay: {1}}
	if got := tracker.subscribed(); !reflect.DeepEqual(got, want) {
		t.Fatalf("after load = %v, want %v", got, want)
	}
	service.InstrumentsChanged([]market.Instrument{{ID: 1, Symbol: "BTCUSDT"}, {ID: 2, Symbol: "ETHUSDT"}})
	want[market.IntervalDay] = []int64{1, 2}
	want[market.IntervalWeek] = []int64{2}
	if got := tracker.subscribed(); !reflect.DeepEqual(got, want) {
		t.Fatalf("after a listing = %v, want %v", got, want)
	}
	store.favorites = []string{"BTCUSDT"}
	// Only the administrator's favorites, user 0 here, count.
	service.FavoritesChanged(context.Background(), 5)
	if got := tracker.subscribed(); !reflect.DeepEqual(got, want) {
		t.Fatalf("after another user's favorites change = %v, want %v", got, want)
	}
	service.FavoritesChanged(context.Background(), 0)
	want[market.IntervalWeek] = []int64{1}
	if got := tracker.subscribed(); !reflect.DeepEqual(got, want) {
		t.Fatalf("after a favorites change = %v, want %v", got, want)
	}
	service.store = &disablingStore{backtestStore: store}
	if _, err := service.DisableAtRevision(context.Background(), 2, 0); err != nil {
		t.Fatal(err)
	}
	if got := tracker.subscribed(); !reflect.DeepEqual(got, map[market.CandleInterval][]int64{market.IntervalHour: {1}, market.IntervalWeek: {1}}) {
		t.Fatalf("after disabling the signal = %v", got)
	}
}

// recordingTracker keeps the subscription of trackerSubscriber; another
// subscriber would leave stale pairs behind.
type recordingTracker struct {
	subscriptions []closedindicator.Subscription
}

func (tracker *recordingTracker) Subscribe(subscriber string, subscriptions []closedindicator.Subscription) {
	if subscriber != trackerSubscriber {
		panic("unexpected subscriber " + subscriber)
	}
	tracker.subscriptions = subscriptions
}

// subscribed lists the subscribed instruments by interval.
func (tracker *recordingTracker) subscribed() map[market.CandleInterval][]int64 {
	result := map[market.CandleInterval][]int64{}
	for _, subscription := range tracker.subscriptions {
		result[subscription.Target.Interval] = append(result[subscription.Target.Interval], subscription.InstrumentID)
	}
	return result
}
