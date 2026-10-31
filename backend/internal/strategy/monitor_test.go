package strategy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"math"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"crypto-scanner/internal/closedindicator"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/scannerindicator"
	"crypto-scanner/internal/telegramhtml"

	"github.com/google/uuid"
)

// testCoins links coins to the Mini App of a test bot.
var testCoins = telegramhtml.NewCoins("scanner_bot")

// Events of one round share a title per kind, followed by the message and a
// line per coin; a message too long for Telegram ends at a whole line.
func TestAlertTextGroupsEventsByKind(t *testing.T) {
	entry := Entry{Strategy: Strategy{Name: "Trend", Direction: DirectionLong, Message: "Check volume"}, Interval: market.IntervalHour}
	buy := func(symbol string, number int) alert {
		return alert{instrument: Instrument{Symbol: symbol}, event: TradeEvent{Kind: TradeBuy, Buy: number, Close: 1.5, Trade: TradeState{TakeProfit: 2, StopLoss: 1}}}
	}
	sell := alert{instrument: Instrument{Symbol: "SOLUSDT"}, event: TradeEvent{Kind: TradeSell, Reason: ExitTakeProfit, Price: 152, Return: 0.038, Trade: TradeState{Buys: 2, Filled: 2, Quantity: 0.01}}}
	got := alertText(entry, []alert{buy("BTCUSDT", 1), sell, buy("ETHUSDT", 2)}, testCoins, noFavorites)
	want := "<b>🟢 Trend · 1h · buy</b>\nCheck volume\n<a href=\"https://t.me/scanner_bot?startapp=BTCUSDT\">BTCUSDT</a> · <a href=\"https://www.binance.com/en/trade/BTC_USDT?type=spot\">Binance</a> at 1.5 · TP 2 · SL 1\n<a href=\"https://t.me/scanner_bot?startapp=ETHUSDT\">ETHUSDT</a> · <a href=\"https://www.binance.com/en/trade/ETH_USDT?type=spot\">Binance</a> #2 at 1.5 · TP 2 · SL 1\n\n" +
		"<b>🔴 Trend · 1h · sell</b>\nCheck volume\n<a href=\"https://t.me/scanner_bot?startapp=SOLUSDT\">SOLUSDT</a> · <a href=\"https://www.binance.com/en/trade/SOL_USDT?type=spot\">Binance</a> at 152 by take profit · +3.80% · entry 200 · 2 buys"
	if got != want {
		t.Fatalf("alertText() = %q, want %q", got, want)
	}

	many := make([]alert, 200)
	for i := range many {
		many[i] = buy("BTCUSDT", 1)
	}
	got = alertText(entry, many, testCoins, noFavorites)
	if len(got) > telegramhtml.MaxLength || !strings.HasSuffix(got, "SL 1\n…message too long") {
		t.Fatalf("long alertText() ends %q after %d bytes", got[max(0, len(got)-40):], len(got))
	}
}

// Caught-up events go in the order of their candles, whatever their coins,
// and those of the latest candle last; consecutive events of one kind share
// a title.
func TestAlertTextOrdersEventsByCandle(t *testing.T) {
	entry := Entry{Strategy: Strategy{Name: "Trend", Direction: DirectionLong}, Interval: market.IntervalHour}
	event := func(symbol string, kind TradeEventKind, past time.Time) alert {
		return alert{instrument: Instrument{Symbol: symbol}, event: TradeEvent{Kind: kind, Close: 1}, past: past}
	}
	got := alertText(entry, []alert{
		event("ETHUSDT", TradeSell, backtestHour(3)),
		event("BTCUSDT", TradeBuy, time.Time{}),
		event("BTCUSDT", TradeBuy, backtestHour(1)),
		event("SOLUSDT", TradeBuy, backtestHour(2)),
		event("BTCUSDT", TradeSell, backtestHour(4)),
	}, testCoins, noFavorites)
	var order []string
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "<b>") {
			order = append(order, line[strings.LastIndex(line, " ")+1:len(line)-len("</b>")])
		} else if line != "" {
			order = append(order, line[strings.Index(line, ">")+1:strings.Index(line, "</a>")])
		}
	}
	if want := []string{"buy", "BTCUSDT", "SOLUSDT", "sell", "ETHUSDT", "BTCUSDT", "buy", "BTCUSDT"}; !slices.Equal(order, want) {
		t.Fatalf("alertText() order = %v, want %v\n%s", order, want, got)
	}
}

// Each message puts the coins among the recipient's favorites first and
// stars them, keeping the order of titles; a long summary cuts the others.
func TestMessagesPutFavoritesFirst(t *testing.T) {
	entry := Entry{Strategy: Strategy{Name: "Trend", Direction: DirectionLong, CoinSource: CoinsAll}, Interval: market.IntervalHour}
	favorite := Recipient{Favorites: []string{"ETHUSDT", "SOLUSDT"}}.favorite
	lines := func(text string) []string {
		var symbols []string
		for _, line := range strings.Split(text, "\n") {
			if end := strings.Index(line, "</a>"); end >= 0 {
				symbols = append(symbols, strings.TrimSuffix(line[:strings.Index(line, "<a ")], " ")+line[strings.Index(line, ">")+1:end])
			} else if strings.HasPrefix(line, "<b>") {
				symbols = append(symbols, line[strings.LastIndex(line, " ")+1:len(line)-len("</b>")])
			}
		}
		return symbols
	}
	event := func(symbol string, kind TradeEventKind) alert {
		return alert{instrument: Instrument{Symbol: symbol}, event: TradeEvent{Kind: kind, Close: 1}}
	}
	got := alertText(entry, []alert{event("BTCUSDT", TradeSell), event("ADAUSDT", TradeBuy), event("SOLUSDT", TradeBuy), event("ETHUSDT", TradeSell)}, testCoins, favorite)
	if want := []string{"sell", "⭐ETHUSDT", "BTCUSDT", "buy", "⭐SOLUSDT", "ADAUSDT"}; !slices.Equal(lines(got), want) {
		t.Fatalf("alertText() = %v, want %v\n%s", lines(got), want, got)
	}

	matching := []Instrument{{Symbol: "BTCUSDT"}, {Symbol: "SOLUSDT"}, {Symbol: "ADAUSDT"}, {Symbol: "ETHUSDT"}}
	got = summaryText(entry, matching, snapshot{}, testCoins, favorite)
	if want := []string{"⭐ETHUSDT", "⭐SOLUSDT", "ADAUSDT", "BTCUSDT"}; !slices.Equal(lines(got)[1:], want) {
		t.Fatalf("summaryText() = %v, want %v\n%s", lines(got), want, got)
	}

	many := []Instrument{{Symbol: "SOLUSDT"}}
	for i := range 200 {
		many = append(many, Instrument{Symbol: fmt.Sprintf("A%03dUSDT", i)})
	}
	got = summaryText(entry, many, snapshot{}, testCoins, favorite)
	if listed := lines(got); len(got) > telegramhtml.MaxLength || listed[1] != "⭐SOLUSDT" || !strings.HasSuffix(got, " more") {
		t.Fatalf("long summaryText() lists %v", listed[:2])
	}
}

// noFavorites stands for a recipient without favorites.
func noFavorites(string) bool { return false }

// A halt disables only the revision that failed: a strategy changed since
// stays enabled.
func TestDisableAtRevisionKeepsAChangedStrategy(t *testing.T) {
	store := &disablingStore{backtestStore: newBacktestStore(market.IntervalHour, nil)}
	service := newBacktestService(t, store.backtestStore, Strategy{ID: testID(1), Name: "Hold", Direction: DirectionLong, Expression: "h_close > 5", Enabled: true, Revision: 3})
	service.store = store
	if disabled, err := service.DisableAtRevision(context.Background(), testID(1), 2); err != nil || disabled || store.calls != 0 || !service.List()[0].Enabled {
		t.Fatalf("stale revision: disabled %v, error %v, store calls %d", disabled, err, store.calls)
	}
	if disabled, err := service.DisableAtRevision(context.Background(), testID(1), 3); err != nil || !disabled || store.calls != 1 || service.List()[0].Enabled || service.List()[0].Revision != 4 {
		t.Fatalf("current revision: disabled %v, error %v, entry %+v", disabled, err, service.List()[0])
	}
}

type disablingStore struct {
	*backtestStore
	calls int
}

func (store *disablingStore) DisableStrategyAtRevision(_ context.Context, _ uuid.UUID, revision int64) (int64, bool, error) {
	store.calls++
	return revision + 1, true, nil
}

// Each strategy subscribes only the coins it evaluates, and a signal of all
// coins every active one.
func TestReadsOfSubscribesTheCoinsOfEachStrategy(t *testing.T) {
	service := newBacktestService(t, newBacktestStore(market.IntervalHour, nil),
		Strategy{ID: testID(1), Name: "Hourly", Expression: "h_close > 1", Coins: []string{"ETHUSDT", "LUNAUSDT"}},
		Strategy{ID: testID(2), Name: "Daily", Signal: true, Expression: "d_close > 1", CoinSource: CoinsAll},
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
	var baselines []uuid.UUID
	service := newBacktestService(t, store.backtestStore, Strategy{ID: testID(1), Name: "Hold", Direction: DirectionLong, Expression: "h_close > 5", Coins: []string{"ETHUSDT"}, CoinSource: CoinsSelected, Enabled: true})
	service.store, service.changed = store, func(ids []uuid.UUID) { baselines = ids }
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
		{ID: testID(1), Name: "Chosen", Expression: "h_close > 5", Coins: []string{"BTCUSDT", "LUNAUSDT"}, Enabled: true, Revision: 3},
		{ID: testID(1), Name: "Read", Expression: `h_close > 5 && of("LUNAUSDT", h_close) > 1`, Coins: []string{"BTCUSDT"}, Enabled: true, Revision: 3},
	} {
		disabling := &disablingStore{backtestStore: newBacktestStore(market.IntervalHour, nil)}
		service := newBacktestService(t, disabling.backtestStore, item)
		service.store = disabling
		monitor := NewMonitor(monitorStore{disabling.backtestStore}, disabling.backtestStore, testRegistry(t), noValues{}, service, nil, testCoins, 7, slog.New(slog.DiscardHandler))
		monitor.evaluate(context.Background(), map[uuid.UUID]map[int64]TradeState{})
		if disabling.calls != 1 || service.List()[0].Enabled {
			t.Fatalf("%s: disable calls %d, entry %+v", item.Name, disabling.calls, service.List()[0])
		}
		if sent := <-monitor.sends; sent.telegramID != 7 || sent.text != "<b>⛔ "+item.Name+" · 1h · disabled</b>\nnot an active coin: LUNAUSDT" {
			t.Fatalf("%s: sent %+v", item.Name, sent)
		}
	}
}

type monitorStore struct{ *backtestStore }

func (monitorStore) ListStrategyStates(context.Context) (map[uuid.UUID]map[int64]TradeState, error) {
	return nil, nil
}

func (monitorStore) ListStrategyRecipients(context.Context, int64) ([]Recipient, error) {
	return []Recipient{{TelegramID: 7}}, nil
}

func (monitorStore) ReplaceStrategyStates(context.Context, uuid.UUID, int64, map[int64]TradeState) (bool, error) {
	panic("unexpected baseline")
}

func (monitorStore) SaveStrategyStates(context.Context, uuid.UUID, int64, map[int64]TradeState) ([]int64, error) {
	panic("unexpected states")
}

func (monitorStore) DeleteStrategyStates(context.Context, uuid.UUID, []int64) error { return nil }

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
	service := newBacktestService(t, store, Strategy{ID: testID(1), Name: "Watch", Signal: true, Direction: DirectionLong, Expression: "h_close > 1", CoinSource: CoinsFavorites, Enabled: true, Revision: 1})
	for _, test := range []struct {
		states map[int64]TradeState
		alert  bool
	}{{map[int64]TradeState{}, false}, {map[int64]TradeState{1: {}}, true}} {
		saving := &savingStore{monitorStore: monitorStore{store}}
		monitor := NewMonitor(saving, store, testRegistry(t), closingValues{}, service, nil, testCoins, 7, slog.New(slog.DiscardHandler))
		monitor.full = true
		monitor.evaluate(context.Background(), map[uuid.UUID]map[int64]TradeState{testID(1): test.states})
		if !reflect.DeepEqual(saving.saved, []int64{1}) || (len(monitor.sends) > 0) != test.alert {
			t.Fatalf("states %v: saved %v, %d alerts", test.states, saving.saved, len(monitor.sends))
		}
	}
}

type savingStore struct {
	monitorStore
	saved []int64
}

func (store *savingStore) SaveStrategyStates(_ context.Context, _ uuid.UUID, _ int64, states map[int64]TradeState) ([]int64, error) {
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
		service := newBacktestService(t, newBacktestStore(market.IntervalHour, nil), Strategy{ID: testID(1), Name: "Hold", Expression: "h_close > 5", Coins: []string{"BTCUSDT"}, Enabled: true})
		store := &failingStore{monitorStore: monitorStore{newBacktestStore(market.IntervalHour, nil)}}
		monitor := NewMonitor(store, store.backtestStore, testRegistry(t), noValues{}, service, nil, testCoins, 7, slog.New(slog.DiscardHandler))
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
		{ID: testID(1), Name: "Hourly", Expression: "h_close > 1", Coins: []string{"BTCUSDT"}, Enabled: true},
		{ID: testID(2), Name: "Daily", Signal: true, Expression: "d_close > 1", CoinSource: CoinsAll, Enabled: true},
		{ID: testID(3), Name: "Off", Expression: "w_close > 1", Coins: []string{"BTCUSDT"}},
		{ID: testID(4), Name: "Weekly", Signal: true, Expression: "w_close > 1", CoinSource: CoinsFavorites, Enabled: true},
	}
	store.favorites = []string{"ETHUSDT"}
	tracker := &recordingTracker{}
	service, err := NewService(store, testIndicators{}, testRegistry(t), tracker, slog.New(slog.DiscardHandler), 0, func([]uuid.UUID) {})
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
	if _, err := service.DisableAtRevision(context.Background(), testID(2), 0); err != nil {
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

// A failed favorites read is retried by the monitor's rounds, so the
// committed change is not lost.
func TestFailedFavoritesReadIsReloadedLater(t *testing.T) {
	store := newBacktestStore(market.IntervalHour, nil)
	service := newBacktestService(t, store, Strategy{ID: testID(1), Name: "Watch", Signal: true, Direction: DirectionLong, Expression: "h_close > 1", CoinSource: CoinsFavorites, Enabled: true, Revision: 1})
	failing := &favoritesStore{backtestStore: store, err: errors.New("database unavailable")}
	service.store = failing
	store.favorites = []string{"BTCUSDT"}

	service.FavoritesChanged(context.Background(), 0)
	if err := service.ReloadFavorites(context.Background()); err == nil {
		t.Fatal("ReloadFavorites succeeded while the store fails")
	}
	failing.err = nil
	if err := service.ReloadFavorites(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := service.List()[0].favorites; !reflect.DeepEqual(got, []string{"BTCUSDT"}) {
		t.Fatalf("favorites = %v, want the changed ones", got)
	}
}

type favoritesStore struct {
	*backtestStore
	err error
}

func (store *favoritesStore) ListStrategyFavorites(ctx context.Context, userID int64) ([]string, error) {
	if store.err != nil {
		return nil, store.err
	}
	return store.backtestStore.ListStrategyFavorites(ctx, userID)
}

// Candles missed after the stored state, such as on a restart or when a
// candle was synchronized late, are stepped in order before the latest one,
// trading exactly as the backtest of the same history; their alerts name
// their candle in UTC, while those of the latest candle do not.
func TestMonitorCatchesUpMissedCandlesLikeTheBacktest(t *testing.T) {
	type bar struct{ open, high, low, close float64 }
	bars := []bar{
		{1, 1, 1, 1},
		{10, 10, 10, 10},    // signal: take profit 12, stop loss 9
		{10, 11, 9.5, 10},   // fills at 10
		{10, 12.5, 9.8, 12}, // take profit at 12
		{12, 12, 12, 12},    // signal: take profit 14.4, stop loss 10.8
		{12, 15, 10.5, 12},  // fills at 12 and reaches both: stop loss at 10.8
		{12, 12, 12, 12},    // signal: take profit 14.4, stop loss 10.8
		{10, 10, 10, 10},    // fills at 10, past the stop loss: sells at 10
		{1, 1, 1, 1},
	}
	candles := make([]market.Candle, len(bars))
	for index, bar := range bars {
		candles[index] = testCandle(1, market.IntervalHour, backtestHour(index), bar.close)
		candles[index].Open, candles[index].High, candles[index].Low = bar.open, bar.high, bar.low
	}
	item := Strategy{
		ID: testID(1), Name: "Range", Expression: "h_close > 5", TakeProfitExpression: "h_close * 1.2", StopLossExpression: "h_close * 0.9",
		Coins: []string{"BTCUSDT"}, Enabled: true, Revision: 1,
	}
	link := testCoins.Symbol("BTCUSDT")
	// signal is the close of the signal at hour 4, whose levels the stored
	// trade holds.
	signal := 12.0
	for _, test := range []struct {
		name string
		// latest is the index of the latest closed candle; the state is
		// stored after the candle at index stored.
		latest, stored int
		state          TradeState
		full           bool
		want           TradeState
		alert          string
	}{{
		name: "restart", latest: 8, stored: 0, state: TradeState{OpenTime: backtestHour(0)}, full: true,
		want: TradeState{OpenTime: backtestHour(8)},
		// Caught-up events go in the order of their candles.
		alert: "<b>🟢 Range · 1h · buy</b>\n" + link + " at 10 · TP 12 · SL 9 · candle 2026-03-02 01:00 UTC\n\n" +
			"<b>🔴 Range · 1h · sell</b>\n" + link + " at 12 by take profit · +20.00% · entry 10 · candle 2026-03-02 03:00 UTC\n\n" +
			"<b>🟢 Range · 1h · buy</b>\n" + link + " at 12 · TP 14.4 · SL 10.8 · candle 2026-03-02 04:00 UTC\n\n" +
			"<b>🔴 Range · 1h · sell</b>\n" + link + " at 10.8 by stop loss · -10.00% · entry 12 · candle 2026-03-02 05:00 UTC\n\n" +
			"<b>🟢 Range · 1h · buy</b>\n" + link + " at 12 · TP 14.4 · SL 10.8 · candle 2026-03-02 06:00 UTC\n\n" +
			"<b>🔴 Range · 1h · sell</b>\n" + link + " at 10 by stop loss · +0.00% · entry 10 · candle 2026-03-02 07:00 UTC",
	}, {
		name: "late candle", latest: 6, stored: 4, state: TradeState{OpenTime: backtestHour(4), Entry: true, Buys: 1, TakeProfit: signal * 1.2, StopLoss: signal * 0.9},
		want: TradeState{OpenTime: backtestHour(6), Entry: true, Buys: 1, TakeProfit: signal * 1.2, StopLoss: signal * 0.9},
		alert: "<b>🔴 Range · 1h · sell</b>\n" +
			link + " at 10.8 by stop loss · -10.00% · entry 12 · candle 2026-03-02 05:00 UTC\n\n" +
			"<b>🟢 Range · 1h · buy</b>\n" +
			link + " at 12 · TP 14.4 · SL 10.8",
	}} {
		t.Run(test.name, func(t *testing.T) {
			store := newBacktestStore(market.IntervalHour, candles[:test.latest+1])
			service := newBacktestService(t, store, item)
			// Every sell the monitor alerts on is a trade of the backtest.
			backtest, err := service.Backtest(context.Background(), testID(1), "BTCUSDT", time.Time{}, time.Time{}, false)
			if err != nil {
				t.Fatal(err)
			}
			for _, trade := range backtest.Trades {
				if trade.Open || !trade.ExitTime.After(backtestHour(test.stored)) {
					continue
				}
				sell := fmt.Sprintf(" at %s by %s · %+.2f%% · entry %s · candle %s", formatNumber(trade.ExitPrice), strings.ReplaceAll(string(trade.Reason), "_", " "),
					100*DirectionLong.gross(trade.EntryPrice, trade.ExitPrice), formatNumber(trade.EntryPrice), candleTime(trade.ExitTime))
				if !strings.Contains(test.alert, sell) {
					t.Fatalf("backtest trade %+v is not alerted as %q", trade, sell)
				}
			}
			saving := &statesStore{monitorStore: monitorStore{store}}
			monitor := NewMonitor(saving, store, testRegistry(t), storedValues{testRegistry(t), store}, service, nil, testCoins, 7, slog.New(slog.DiscardHandler))
			monitor.now = func() time.Time { return backtestHour(test.latest + 1) }
			monitor.full = test.full
			monitor.pending[1] = struct{}{}
			monitor.evaluate(context.Background(), map[uuid.UUID]map[int64]TradeState{testID(1): {1: test.state}})
			if got := saving.states[1]; !reflect.DeepEqual(got, test.want) {
				t.Fatalf("stored state = %+v, want %+v", got, test.want)
			}
			if len(monitor.sends) != 1 {
				t.Fatalf("%d alerts, want 1", len(monitor.sends))
			}
			if sent := <-monitor.sends; sent.text != test.alert {
				t.Fatalf("alert = %q, want %q", sent.text, test.alert)
			}
		})
	}
}

// statesStore stores every state it is given.
type statesStore struct {
	monitorStore
	states map[int64]TradeState
}

func (store *statesStore) SaveStrategyStates(_ context.Context, _ uuid.UUID, _ int64, states map[int64]TradeState) ([]int64, error) {
	store.states = states
	return slices.Collect(maps.Keys(states)), nil
}

// storedValues calculates every subscription over the latest stored
// candles, as the tracker does.
type storedValues struct {
	registry *indicator.Registry
	store    *backtestStore
}

func (storedValues) Listen(func([]closedindicator.Change)) {}

func (values storedValues) Snapshot(subscriptions []closedindicator.Subscription) ([]closedindicator.Value, []closedindicator.Target) {
	result := make([]closedindicator.Value, len(subscriptions))
	for index, subscription := range subscriptions {
		depth, err := closedindicator.Depth(values.registry, subscription.Target, subscription.Points)
		if err != nil {
			panic(err)
		}
		candles := values.store.candles[subscription.Target.Interval][subscription.InstrumentID]
		if result[index], err = closedindicator.Calculate(values.registry, subscription.Target, candles[max(0, len(candles)-depth):], subscription.Points); err != nil {
			panic(err)
		}
	}
	return result, nil
}

// A catch-up loads as deep as its span and the warm-up need, so a missed
// candle further back than the synchronized depth leaves warm is still
// stepped; candles whose stored history is truly too short are skipped,
// and the catch-up goes on from the first warm one, as the backtest does.
func TestMonitorCatchesUpDeepSpansFromTheFirstWarmCandle(t *testing.T) {
	registry := testRegistry(t)
	indicators := testIndicators{entries: []scannerindicator.Entry{testEMA(t, registry, 1, market.IntervalHour, 100)}}
	const missed = 1500
	for _, test := range []struct {
		name    string
		history int
		cold    bool
	}{{"deep", 3000, false}, {"partly cold", 2200, true}} {
		t.Run(test.name, func(t *testing.T) {
			candles := make([]market.Candle, test.history)
			for index := range candles {
				candles[index] = testCandle(1, market.IntervalHour, backtestHour(index), 100+10*math.Sin(float64(index)/100))
			}
			store := newBacktestStore(market.IntervalHour, candles)
			store.strategies = []Strategy{{ID: testID(1), Name: "Cross", Signal: true, Direction: DirectionSimple, Expression: "crosses_above(h_close, h_ema_100)", Coins: []string{"BTCUSDT"}, Enabled: true, Revision: 1}}
			service, err := NewService(store, indicators, registry, noTracker{}, slog.New(slog.DiscardHandler), 0, func([]uuid.UUID) {})
			if err != nil {
				t.Fatal(err)
			}
			if err := service.Load(context.Background()); err != nil {
				t.Fatal(err)
			}
			latest, stored := len(candles)-1, len(candles)-2-missed
			backtest, err := service.Backtest(context.Background(), testID(1), "BTCUSDT", time.Time{}, time.Time{}, false)
			if err != nil {
				t.Fatal(err)
			}
			if cold := backtest.From.After(backtestHour(stored + 1)); cold != test.cold {
				t.Fatalf("backtest starts at %s, cold missed candles %v, want %v", backtest.From, cold, test.cold)
			}
			var want []string
			for _, occurrence := range backtest.Signal.Occurrences {
				if occurrence.Time.After(backtestHour(stored)) && occurrence.Time.Before(backtestHour(latest)) {
					want = append(want, candleTime(occurrence.Time))
				}
			}
			if len(want) == 0 {
				t.Fatal("no missed signal to catch up")
			}

			saving := &statesStore{monitorStore: monitorStore{store}}
			monitor := NewMonitor(saving, store, registry, storedValues{registry, store}, service, nil, testCoins, 7, slog.New(slog.DiscardHandler))
			monitor.now = func() time.Time { return backtestHour(latest + 1) }
			monitor.full = true
			monitor.evaluate(context.Background(), map[uuid.UUID]map[int64]TradeState{testID(1): {1: {OpenTime: backtestHour(stored)}}})
			if got := saving.states[1].OpenTime; !got.Equal(backtestHour(latest)) {
				t.Fatalf("stored state at %s, want the latest candle", got)
			}
			var got []string
			for len(monitor.sends) > 0 {
				for _, line := range strings.Split((<-monitor.sends).text, "\n") {
					if _, at, ok := strings.Cut(line, " · candle "); ok {
						got = append(got, at)
					}
				}
			}
			if !slices.Equal(got, want) {
				t.Fatalf("caught-up signals at %v, want the backtest's %v", got, want)
			}
		})
	}
}

// A catch-up that fails, or processes no candle, waits for the retry delay
// of its instrument instead of being repeated every round, while the other
// instruments go on without delay and catch up within the limit of the
// round.
func TestMonitorRetriesACatchUpPerInstrument(t *testing.T) {
	for _, test := range []struct {
		name   string
		err    error
		values Values
	}{{"failed", errors.New("database unavailable"), closingValues{}}, {"stalled", nil, noValues{}}} {
		t.Run(test.name, func(t *testing.T) {
			store := newBacktestStore(market.IntervalHour, nil)
			service := newBacktestService(t, store, Strategy{ID: testID(1), Name: "Hold", Expression: "h_close > 1", Coins: []string{"BTCUSDT", "ETHUSDT"}, Enabled: true, Revision: 1})
			// Only the catch-ups of BTCUSDT fail.
			candles := &countingCandles{err: test.err}
			saving := &twoCoinsStore{statesStore: statesStore{monitorStore: monitorStore{store}}}
			monitor := NewMonitor(saving, candles, testRegistry(t), test.values, service, nil, testCoins, 7, slog.New(slog.DiscardHandler))
			monitor.catchUpLimit = 1
			start := time.Now()
			latest := market.IntervalHour.LastClosedOpenTime(start)
			state := map[uuid.UUID]map[int64]TradeState{testID(1): {1: {OpenTime: latest.Add(-5 * time.Hour)}, 2: {OpenTime: latest.Add(-time.Hour)}}}
			for round, now := range []time.Time{start, start.Add(retryDelay - time.Second), start.Add(retryDelay)} {
				if round == 1 {
					// ETHUSDT lags too, and BTCUSDT, waiting, leaves it the limit.
					state[testID(1)][2] = TradeState{OpenTime: latest.Add(-5 * time.Hour)}
				}
				saving.states = nil
				monitor.now = func() time.Time { return now }
				monitor.full = true
				monitor.evaluate(context.Background(), state)
				if want := [][]int64{{1}, {2}, {1}}[:round+1]; !reflect.DeepEqual(candles.loaded, want) {
					t.Fatalf("round %d: catch-ups loaded %v, want %v", round, candles.loaded, want)
				}
				if _, stored := saving.states[1]; stored || monitor.failed {
					t.Fatalf("round %d: stored %v, round failed %v", round, saving.states, monitor.failed)
				}
				if _, stored := saving.states[2]; round == 0 && test.err != nil && !stored {
					t.Fatal("ETHUSDT did not go on")
				}
			}
		})
	}
}

// countingCandles records the instruments of each catch-up load and fails
// those of BTCUSDT (1) with err.
type countingCandles struct {
	loaded [][]int64
	err    error
}

func (candles *countingCandles) ListLatestCandles(_ context.Context, ids []int64, _ market.CandleInterval, _ int) (map[int64][]market.Candle, error) {
	candles.loaded = append(candles.loaded, ids)
	if slices.Contains(ids, 1) {
		return nil, candles.err
	}
	return nil, nil
}

// A round catches up at most the limit of instruments, those that missed
// the fewest candles first, and the next round goes on with the rest.
func TestMonitorCatchesUpInChunks(t *testing.T) {
	store := newBacktestStore(market.IntervalHour, nil)
	for id := range int64(2) {
		for index := range 10 {
			store.candles[market.IntervalHour][id+1] = append(store.candles[market.IntervalHour][id+1], testCandle(id+1, market.IntervalHour, backtestHour(index), 2))
		}
	}
	service := newBacktestService(t, store, Strategy{ID: testID(1), Name: "Hold", Expression: "h_close > 1", Coins: []string{"BTCUSDT", "ETHUSDT"}, Enabled: true, Revision: 1})
	saving := &twoCoinsStore{statesStore: statesStore{monitorStore: monitorStore{store}}}
	monitor := NewMonitor(saving, store, testRegistry(t), storedValues{testRegistry(t), store}, service, nil, testCoins, 7, slog.New(slog.DiscardHandler))
	monitor.now = func() time.Time { return backtestHour(10) }
	monitor.catchUpLimit = 1
	monitor.full = true
	state := map[uuid.UUID]map[int64]TradeState{testID(1): {1: {OpenTime: backtestHour(2)}, 2: {OpenTime: backtestHour(5)}}}
	for round, want := range []int64{2, 1} {
		monitor.evaluate(context.Background(), state)
		if got := saving.states[want]; len(saving.states) != 1 || !got.OpenTime.Equal(backtestHour(9)) {
			t.Fatalf("round %d stored %v, want instrument %d at the latest candle", round, saving.states, want)
		}
		if round == 0 && len(monitor.wake) != 1 {
			t.Fatal("the rest of the catch-up was not scheduled")
		}
	}
}

// twoCoinsStore lists BTCUSDT (1) and ETHUSDT (2) as active.
type twoCoinsStore struct{ statesStore }

func (*twoCoinsStore) ListStrategyInstruments(context.Context) ([]Instrument, error) {
	return []Instrument{{ID: 1, Symbol: "BTCUSDT"}, {ID: 2, Symbol: "ETHUSDT"}}, nil
}
