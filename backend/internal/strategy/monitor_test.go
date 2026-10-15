package strategy

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"testing"

	"crypto-scanner/internal/closedindicator"
	"crypto-scanner/internal/market"
)

func TestAlertTextUsesMessage(t *testing.T) {
	entry := Entry{Strategy: Strategy{Name: "Breakout", Expression: "h_close > 1", Message: "Buy the breakout\nStop below 1"}}
	got := alertText(entry, Instrument{ID: 1, Symbol: "BTCUSDT"}, TradeEvent{Kind: TradeBuy, Buy: 2, Close: 1.5}, snapshot{})
	if want := "🟢 Breakout: BTCUSDT buy #2 at 1.5\nBuy the breakout\nStop below 1"; got != want {
		t.Fatalf("alertText() = %q, want %q", got, want)
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
		Strategy{ID: 2, Name: "Daily", Signal: true, Expression: "d_close > 1", AllCoins: true},
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
	service := newBacktestService(t, store.backtestStore, Strategy{ID: 1, Name: "Hold", Direction: DirectionLong, Expression: "h_close > 5", Coins: []string{"ETHUSDT"}, Enabled: true})
	service.store, service.changed = store, func(ids []int64) { baselines = ids }
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

func (store *updatingStore) ListStrategyInstruments(context.Context) ([]Instrument, error) {
	return []Instrument{{ID: 1, Symbol: "BTCUSDT"}, {ID: 2, Symbol: "ETHUSDT"}}, nil
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
		if sent := <-monitor.sends; sent.telegramID != 7 || sent.text != "⛔ "+item.Name+" is disabled: not an active coin: LUNAUSDT" {
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

func (noValues) Listen(func(closedindicator.Change)) {}

func (noValues) Snapshot(subscriptions []closedindicator.Subscription) ([]closedindicator.Value, []closedindicator.Target) {
	return make([]closedindicator.Value, len(subscriptions)), nil
}
