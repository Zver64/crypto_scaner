package alerts

import (
	"context"
	"log/slog"
	"slices"
	"sync"
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
	ready    chan struct{}
	statuses chan markettrade.Status
}

func newTradeFeedFake() *tradeFeedFake {
	return &tradeFeedFake{ready: make(chan struct{}), statuses: make(chan markettrade.Status)}
}
func (feed *tradeFeedFake) Ready() <-chan struct{}              { return feed.ready }
func (feed *tradeFeedFake) Drain() []markettrade.Event          { return nil }
func (feed *tradeFeedFake) Statuses() <-chan markettrade.Status { return feed.statuses }
func (feed *tradeFeedFake) SetSymbols(symbols []string) {
	feed.sets++
	feed.symbols = append([]string(nil), symbols...)
}

// Compile-time guard for accidental widening of the persistence seam.
var _ MonitorStore = (*monitorStoreFake)(nil)

func TestMonitorSkipsUnchangedSubscriptions(t *testing.T) {
	feed := newTradeFeedFake()
	monitor := NewMonitor(&monitorStoreFake{}, feed, nil, slog.New(slog.DiscardHandler))
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
	monitor := NewMonitor(store, nil, nil, slog.New(slog.DiscardHandler))
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
	m := NewMonitor(&monitorStoreFake{}, nil, nil, slog.New(slog.DiscardHandler))
	states := map[string]*symbolState{"BTCUSDT": {alerts: map[int64]Alert{1: {ID: 1, Version: 1, Symbol: "BTCUSDT", Target: "100"}}}}
	at := time.Now()
	m.trade(context.Background(), states, markettrade.Event{Symbol: "BTCUSDT", Price: "90", Low: "90", High: "90", TradeID: 1, Epoch: 1, EventTime: at})
	if len(m.sends) != 0 {
		t.Fatal("first event crossed without a baseline")
	}
	m.trade(context.Background(), states, markettrade.Event{Symbol: "BTCUSDT", Price: "110", Low: "110", High: "110", TradeID: 2, Epoch: 1, EventTime: at})
	if len(m.sends) != 1 {
		t.Fatalf("crossing fired %d alerts", len(m.sends))
	}
	states["BTCUSDT"].alerts[2] = Alert{ID: 2, Version: 1, Symbol: "BTCUSDT", Target: "100"}
	m.trade(context.Background(), states, markettrade.Event{Symbol: "BTCUSDT", Price: "90", Low: "90", High: "90", TradeID: 3, Epoch: 2, EventTime: at})
	if len(m.sends) != 1 {
		t.Fatal("crossed across reconnect epoch")
	}
	m.trade(context.Background(), states, markettrade.Event{Symbol: "BTCUSDT", Price: "100", Low: "100", High: "100", TradeID: 4, Epoch: 2, EventTime: at})
	if len(m.sends) != 2 {
		t.Fatal("equality did not fire")
	}
	m.trade(context.Background(), states, markettrade.Event{Symbol: "BTCUSDT", Price: "101", Low: "101", High: "101", TradeID: 4, Epoch: 2, EventTime: at})
	if len(m.sends) != 2 {
		t.Fatal("duplicate trade ID fired")
	}
}

// Editing one user's alert keeps the symbol's baseline for the others; only
// the edited alert ignores the price traded before it was armed.
func TestMonitorApplyKeepsOtherAlertsCrossing(t *testing.T) {
	m := NewMonitor(&monitorStoreFake{}, nil, nil, slog.New(slog.DiscardHandler))
	at := time.Now()
	states := map[string]*symbolState{"BTCUSDT": {alerts: map[int64]Alert{
		1: {ID: 1, UserID: 1, Version: 1, Symbol: "BTCUSDT", Target: "200"},
		2: {ID: 2, UserID: 2, Version: 1, Symbol: "BTCUSDT", Target: "100"},
	}}}
	m.trade(context.Background(), states, markettrade.Event{Symbol: "BTCUSDT", Price: "90", Low: "90", High: "90", TradeID: 1, Epoch: 1, EventTime: at})
	edited := Alert{ID: 1, UserID: 1, Version: 2, Symbol: "BTCUSDT", Target: "95"}
	m.operation(context.Background(), states, liveOperation{apply: &edited})
	m.trade(context.Background(), states, markettrade.Event{Symbol: "BTCUSDT", Price: "110", Low: "110", High: "110", TradeID: 2, Epoch: 1, EventTime: at})
	if len(m.sends) != 1 {
		t.Fatalf("fired %d alerts, want only user B's", len(m.sends))
	}
	if fired := <-m.sends; fired.Alert.ID != 2 {
		t.Fatalf("fired alert %d, want 2", fired.Alert.ID)
	}
}

// Changed reloads the monitored symbols, such as after an instrument is
// activated.
func TestMonitorChangedResubscribes(t *testing.T) {
	store := &reloadStoreFake{symbols: []string{"BTCUSDT"}}
	feed := &subscriptionFeedFake{tradeFeedFake: newTradeFeedFake(), sets: make(chan []string, 4)}
	monitor := NewMonitor(store, feed, nil, slog.New(slog.DiscardHandler))
	ctx, stop := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- monitor.Run(ctx) }()
	defer func() { stop(); <-done }()
	if got := <-feed.sets; !slices.Equal(got, []string{"BTCUSDT"}) {
		t.Fatalf("initial symbols = %v", got)
	}

	store.setSymbols([]string{"BTCUSDT", "ETHUSDT"})
	monitor.Changed()

	if got := <-feed.sets; !slices.Equal(got, []string{"BTCUSDT", "ETHUSDT"}) {
		t.Fatalf("symbols after Changed = %v", got)
	}
}

// Alerts queued when the monitor stops are still fired and sent.
func TestMonitorDeliversQueuedAlertsOnShutdown(t *testing.T) {
	store := &monitorStoreFake{}
	sender := &alertSenderFake{}
	monitor := NewMonitor(store, newTradeFeedFake(), sender, slog.New(slog.DiscardHandler))
	monitor.sends <- Fired{Alert: Alert{ID: 7, Version: 1, Symbol: "BTCUSDT", Target: "100"}, Price: "100"}
	ctx, stop := context.WithCancel(t.Context())
	stop()

	if err := monitor.Run(ctx); err != nil {
		t.Fatal(err)
	}

	if len(store.fired) != 1 || len(sender.sent) != 1 || sender.sent[0].Alert.ID != 7 {
		t.Fatalf("fired %v, sent %v, want alert 7 fired and sent", store.fired, sender.sent)
	}
}

type reloadStoreFake struct {
	monitorStoreFake
	mu      sync.Mutex
	symbols []string
}

func (s *reloadStoreFake) ListMonitoredSymbols(context.Context) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.symbols), nil
}
func (s *reloadStoreFake) setSymbols(symbols []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.symbols = symbols
}

type subscriptionFeedFake struct {
	*tradeFeedFake
	sets chan []string
}

func (feed *subscriptionFeedFake) SetSymbols(symbols []string) { feed.sets <- slices.Clone(symbols) }

// alertSenderFake fails deliveries whose context is already cancelled.
type alertSenderFake struct {
	mu   sync.Mutex
	sent []Fired
}

func (s *alertSenderFake) SendPriceAlert(ctx context.Context, fired Fired) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, fired)
	return nil
}
