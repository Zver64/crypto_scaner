package alerts

import (
	"context"
	"testing"
	"time"

	markettrade "crypto-scanner/internal/market/trade"
)

type monitorStoreFake struct{ fired []int64 }

func (*monitorStoreFake) ListEnabledAlerts(context.Context) ([]Alert, error) { return nil, nil }
func (s *monitorStoreFake) FireAlert(_ context.Context, id, version int64) (bool, error) {
	s.fired = append(s.fired, id)
	return true, nil
}
func (*monitorStoreFake) ListMonitoredSymbols(context.Context) ([]string, error) { return nil, nil }

type tradeFeedFake struct {
	sets     int
	symbols  []string
	events   chan markettrade.Event
	statuses chan markettrade.Status
}

func newTradeFeedFake() *tradeFeedFake {
	return &tradeFeedFake{events: make(chan markettrade.Event), statuses: make(chan markettrade.Status)}
}
func (feed *tradeFeedFake) Events() <-chan markettrade.Event    { return feed.events }
func (feed *tradeFeedFake) Statuses() <-chan markettrade.Status { return feed.statuses }
func (feed *tradeFeedFake) SetSymbols(symbols []string) {
	feed.sets++
	feed.symbols = append([]string(nil), symbols...)
}

// Compile-time guard for accidental widening of the persistence seam.
var _ MonitorStore = (*monitorStoreFake)(nil)

func TestMonitorSkipsUnchangedSubscriptions(t *testing.T) {
	feed := newTradeFeedFake()
	monitor := NewMonitor(&monitorStoreFake{}, feed, nil, nil)
	states := map[string]*symbolState{"ETHUSDT": {}, "BTCUSDT": {}}
	var subscribed []string
	monitor.syncSubscriptions(states, &subscribed)
	monitor.syncSubscriptions(states, &subscribed)
	if feed.sets != 1 {
		t.Fatalf("SetSymbols calls = %d, want 1", feed.sets)
	}
	if len(feed.symbols) != 2 || feed.symbols[0] != "BTCUSDT" || feed.symbols[1] != "ETHUSDT" {
		t.Fatalf("symbols = %v", feed.symbols)
	}
}

func TestMonitorApplyFiresImmediateEqualityWithOwnerAndTradeTime(t *testing.T) {
	store := &monitorStoreFake{}
	monitor := NewMonitor(store, nil, nil, nil)
	tradeTime := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	states := map[string]*symbolState{"BTCUSDT": {
		lastPrice: "100", lastTradeID: 7, lastEventTime: tradeTime, epoch: 1, fresh: true,
		alerts: map[int64]Alert{},
	}}
	alert := Alert{ID: 1, UserID: 2, TelegramID: 12345, Version: 1, Symbol: "BTCUSDT", Target: "100"}
	monitor.operation(context.Background(), states, liveOperation{apply: &alert})
	select {
	case fired := <-monitor.sends:
		if fired.Alert.TelegramID != 12345 || !fired.EventTime.Equal(tradeTime) || fired.Price != "100" {
			t.Fatalf("fired alert = %+v", fired)
		}
	default:
		t.Fatal("equal live price did not fire immediately")
	}
}

func TestMonitorTradeCrossingAndEpochReset(t *testing.T) {
	store := &monitorStoreFake{}
	m := NewMonitor(store, nil, nil, nil)
	states := map[string]*symbolState{"BTCUSDT": {alerts: map[int64]Alert{1: {ID: 1, Version: 1, Symbol: "BTCUSDT", Target: "100"}}}}
	at := time.Now()
	m.trade(context.Background(), states, markettrade.Event{Symbol: "BTCUSDT", Price: "90", TradeID: 1, Epoch: 1, EventTime: at})
	if len(store.fired) != 0 {
		t.Fatal("first event crossed without a baseline")
	}
	m.trade(context.Background(), states, markettrade.Event{Symbol: "BTCUSDT", Price: "110", TradeID: 2, Epoch: 1, EventTime: at})
	if len(store.fired) != 1 {
		t.Fatalf("crossing fired %d alerts", len(store.fired))
	}
	states["BTCUSDT"].alerts[2] = Alert{ID: 2, Version: 1, Symbol: "BTCUSDT", Target: "100"}
	m.trade(context.Background(), states, markettrade.Event{Symbol: "BTCUSDT", Price: "90", TradeID: 3, Epoch: 2, EventTime: at})
	if len(store.fired) != 1 {
		t.Fatal("crossed across reconnect epoch")
	}
	m.trade(context.Background(), states, markettrade.Event{Symbol: "BTCUSDT", Price: "100", TradeID: 4, Epoch: 2, EventTime: at})
	if len(store.fired) != 2 {
		t.Fatal("equality did not fire")
	}
	m.trade(context.Background(), states, markettrade.Event{Symbol: "BTCUSDT", Price: "101", TradeID: 4, Epoch: 2, EventTime: at})
	if len(store.fired) != 2 {
		t.Fatal("duplicate trade ID fired")
	}
}
