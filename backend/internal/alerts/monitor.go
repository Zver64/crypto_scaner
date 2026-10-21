package alerts

import (
	"context"
	"log/slog"
	"math/big"
	"slices"
	"time"

	markettrade "crypto-scanner/internal/market/trade"
	"crypto-scanner/internal/platform/backoff"
	"crypto-scanner/internal/platform/delivery"
	"crypto-scanner/internal/platform/logging"
)

const (
	sendWorkers = 4
	// retryDelay and maxRetryDelay space the retries of a failed load.
	retryDelay    = 10 * time.Second
	maxRetryDelay = 5 * time.Minute
)

type TriggerStore interface {
	ListEnabledAlerts(context.Context) ([]Alert, error)
	FireAlert(context.Context, int64, int64) (bool, error)
}

type MonitorStore interface {
	TriggerStore
	ListMonitoredSymbols(context.Context) ([]string, error)
}
type AlertSender interface {
	SendPriceAlert(context.Context, Fired) error
}
type liveOperation struct {
	apply                *Alert
	removeUser, removeID int64
}
type symbolState struct {
	lastPrice     string
	last          *big.Rat // lastPrice, parsed
	lastTradeID   int64
	lastEventTime time.Time
	epoch         int64
	fresh         bool
	alerts        map[int64]Alert
	// armedAfterBaseline holds the alerts applied after the current baseline
	// trade; the next trade checks them without the baseline price, which
	// precedes them.
	armedAfterBaseline map[int64]struct{}
	// targets caches parsed alert targets, which are compared on every trade.
	targets map[string]*big.Rat
}

// reset drops the trade baseline; the next trade establishes a new one.
func (s *symbolState) reset() {
	s.fresh, s.lastPrice, s.last, s.lastTradeID, s.lastEventTime = false, "", nil, 0, time.Time{}
	s.armedAfterBaseline = nil
}

// target returns the parsed target of a, or nil when it is not a decimal.
func (s *symbolState) target(a Alert) *big.Rat {
	if value, ok := s.targets[a.Target]; ok {
		return value
	}
	value, ok := new(big.Rat).SetString(a.Target)
	if !ok {
		value = nil
	}
	if s.targets == nil {
		s.targets = map[string]*big.Rat{}
	}
	s.targets[a.Target] = value
	return value
}

type Monitor struct {
	store   MonitorStore
	feed    markettrade.Feed
	sender  AlertSender
	logger  *slog.Logger
	ops     chan liveOperation
	refresh chan struct{}
	sends   chan Fired
	// loadFailure warns once while loads of the alerts keep failing.
	loadFailure *logging.Failure
}

func NewMonitor(store MonitorStore, feed markettrade.Feed, sender AlertSender, logger *slog.Logger) *Monitor {
	logger = logger.With("module", "alerts")
	return &Monitor{store: store, feed: feed, sender: sender, logger: logger, loadFailure: logging.NewFailure(logger), ops: make(chan liveOperation, 128), refresh: make(chan struct{}, 1), sends: make(chan Fired, 128)}
}
func (m *Monitor) Apply(a Alert) { copy := a; m.enqueue(liveOperation{apply: &copy}) }
func (m *Monitor) Remove(userID, id int64) {
	m.enqueue(liveOperation{removeUser: userID, removeID: id})
}

// enqueue never blocks the caller's request. A full queue, or a stopped
// monitor, falls back to a full reload: it reflects the committed change,
// including an immediate fire for a target equal to the current price.
func (m *Monitor) enqueue(op liveOperation) {
	select {
	case m.ops <- op:
	default:
		m.Changed()
	}
}

// Changed reloads the alerts and the monitored symbols, such as after access,
// favorites, or the active instruments change.
func (m *Monitor) Changed() {
	select {
	case m.refresh <- struct{}{}:
	default:
	}
}

// Run monitors price alerts until ctx is cancelled. It retries loading them
// until it succeeds.
func (m *Monitor) Run(ctx context.Context) error {
	states := map[string]*symbolState{}
	subscribed := []string(nil)
	for failures := 0; ; failures++ {
		err := m.reload(ctx, states, &subscribed)
		if err == nil {
			break
		}
		if ctx.Err() != nil {
			return nil
		}
		m.loadFailure.Warn(ctx, "initial price alerts load failed", "error", err)
		if backoff.Sleep(ctx, backoff.Exponential(retryDelay, maxRetryDelay, failures)) != nil {
			return nil
		}
	}
	m.loadFailure.Recover(ctx, "price alerts load recovered")
	// The alerts queued at shutdown are still fired and sent.
	waitSends := delivery.Start(ctx, sendWorkers, m.sends, m.deliver)
	defer waitSends()
	// A failed refresh is retried until one succeeds; a refresh that a
	// change requests meanwhile replaces the pending retry.
	retry := time.NewTimer(retryDelay)
	retry.Stop()
	defer retry.Stop()
	failures := 0
	refresh := func() {
		if m.refreshAll(ctx, states, &subscribed) {
			retry.Stop()
			failures = 0
			return
		}
		retry.Reset(backoff.Exponential(retryDelay, maxRetryDelay, failures))
		failures++
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-retry.C:
			refresh()
		case <-m.refresh:
			refresh()
		case op := <-m.ops:
			m.operation(ctx, states, op)
			m.syncSubscriptions(states, &subscribed)
		case status := <-m.feed.Statuses():
			if !status.Connected {
				for _, symbol := range status.Symbols {
					if s := states[symbol]; s != nil {
						s.reset()
					}
				}
			}
		case <-m.feed.Ready():
			for _, event := range m.feed.Drain() {
				m.trade(ctx, states, event)
			}
		}
	}
}

// refreshAll applies queued operations before reloading, so an operation
// enqueued before a newer committed change can never overwrite the reload.
// It reports whether the reload succeeded.
func (m *Monitor) refreshAll(ctx context.Context, states map[string]*symbolState, subscribed *[]string) bool {
	for drained := false; !drained; {
		select {
		case op := <-m.ops:
			m.operation(ctx, states, op)
		default:
			drained = true
		}
	}
	if err := m.reload(ctx, states, subscribed); err != nil {
		m.loadFailure.Warn(ctx, "refresh price alerts failed", "error", err)
		return false
	}
	m.loadFailure.Recover(ctx, "price alerts load recovered")
	return true
}

func (m *Monitor) reload(ctx context.Context, states map[string]*symbolState, subscribed *[]string) error {
	items, err := m.store.ListEnabledAlerts(ctx)
	if err != nil {
		return err
	}
	favorites, err := m.store.ListMonitoredSymbols(ctx)
	if err != nil {
		return err
	}
	desired := map[string]struct{}{}
	for _, v := range favorites {
		desired[v] = struct{}{}
	}
	next := map[string]map[int64]Alert{}
	for _, a := range items {
		if next[a.Symbol] == nil {
			next[a.Symbol] = map[int64]Alert{}
		}
		next[a.Symbol][a.ID] = a
	}
	for symbol := range desired {
		s := states[symbol]
		if s == nil {
			s = &symbolState{alerts: map[int64]Alert{}}
			states[symbol] = s
		}
		incoming := next[symbol]
		if incoming == nil {
			incoming = map[int64]Alert{}
		}
		s.targets = nil
		var changed []Alert
		for id, alert := range incoming {
			if existing, ok := s.alerts[id]; !ok || existing.Version != alert.Version {
				changed = append(changed, alert)
			}
		}
		s.alerts = incoming
		if len(changed) > 0 {
			m.applied(ctx, s, changed...)
		}
	}
	for symbol := range states {
		if _, ok := desired[symbol]; !ok {
			delete(states, symbol)
		}
	}
	m.syncSubscriptions(states, subscribed)
	return nil
}

func (m *Monitor) syncSubscriptions(states map[string]*symbolState, subscribed *[]string) {
	symbols := make([]string, 0, len(states))
	for symbol := range states {
		symbols = append(symbols, symbol)
	}
	slices.Sort(symbols)
	if slices.Equal(symbols, *subscribed) {
		return
	}
	m.feed.SetSymbols(symbols)
	*subscribed = symbols
}
func (m *Monitor) operation(ctx context.Context, states map[string]*symbolState, op liveOperation) {
	if op.apply != nil {
		a := *op.apply
		s := states[a.Symbol]
		if s == nil {
			s = &symbolState{alerts: map[int64]Alert{}}
			states[a.Symbol] = s
		}
		// Versions only grow; an operation older than a reload is stale.
		if existing, ok := s.alerts[a.ID]; ok && existing.Version > a.Version {
			return
		}
		s.alerts[a.ID] = a
		m.applied(ctx, s, a)
		return
	}
	for _, s := range states {
		if a, ok := s.alerts[op.removeID]; ok && a.UserID == op.removeUser {
			delete(s.alerts, op.removeID)
		}
	}
}

// applied handles new or re-versioned alerts of s: a target equal to the
// current price fires at once, and no target may use the pre-change side of a
// crossing, so the next trade checks them without the baseline price. The
// baseline itself stays for the other alerts of the symbol.
func (m *Monitor) applied(ctx context.Context, s *symbolState, alerts ...Alert) {
	if !s.fresh {
		return
	}
	if s.armedAfterBaseline == nil {
		s.armedAfterBaseline = map[int64]struct{}{}
	}
	for _, a := range alerts {
		s.armedAfterBaseline[a.ID] = struct{}{}
		if cmp, err := Compare(a.Target, s.lastPrice); err == nil && cmp == 0 {
			m.fire(ctx, s, a, s.lastPrice, s.lastEventTime)
		}
	}
}

// trade fires every alert whose target lies in the price range traded since
// the previous event, including the previous last price, so a target crossed
// between two trades fires even when no trade printed it.
func (m *Monitor) trade(ctx context.Context, states map[string]*symbolState, e markettrade.Event) {
	s := states[e.Symbol]
	if s == nil {
		return
	}
	if s.epoch != e.Epoch {
		s.reset()
		s.epoch = e.Epoch
	}
	if s.fresh && e.TradeID <= s.lastTradeID {
		return
	}
	low, lowOK := new(big.Rat).SetString(e.Low)
	high, highOK := new(big.Rat).SetString(e.High)
	last, lastOK := new(big.Rat).SetString(e.Price)
	if !lowOK || !highOK || !lastOK {
		m.logger.WarnContext(ctx, "invalid trade event", "symbol", e.Symbol)
		return
	}
	tradedLow, tradedHigh := low, high
	if s.fresh {
		if s.last.Cmp(low) < 0 {
			low = s.last
		}
		if s.last.Cmp(high) > 0 {
			high = s.last
		}
	}
	armedAfterBaseline := s.armedAfterBaseline
	s.lastPrice, s.last, s.lastTradeID, s.lastEventTime, s.fresh = e.Price, last, e.TradeID, e.EventTime, true
	s.armedAfterBaseline = nil
	for _, a := range s.alerts {
		from, to := low, high
		if _, ok := armedAfterBaseline[a.ID]; ok {
			from, to = tradedLow, tradedHigh
		}
		if target := s.target(a); target != nil && target.Cmp(from) >= 0 && target.Cmp(to) <= 0 {
			m.fire(ctx, s, a, e.Price, e.EventTime)
		}
	}
}

// fire hands a triggered alert to the send workers, which persist it before
// delivery. A failed persistence leaves the alert in the database, and the
// next reload re-arms it.
func (m *Monitor) fire(ctx context.Context, s *symbolState, a Alert, price string, at time.Time) {
	delete(s.alerts, a.ID)
	select {
	case m.sends <- Fired{Alert: a, Price: price, EventTime: at}:
	case <-ctx.Done():
		m.logger.InfoContext(ctx, "price alert delivery cancelled during shutdown", "alert_id", a.ID)
	}
}

// deliver persists the fired alert and sends it. The version check makes a
// repeated fire of one alert a no-op.
func (m *Monitor) deliver(ctx context.Context, item Fired) {
	fired, err := m.store.FireAlert(ctx, item.Alert.ID, item.Alert.Version)
	if err != nil {
		m.logger.WarnContext(ctx, "fire price alert failed", "alert_id", item.Alert.ID, "error", err)
		return
	}
	if !fired {
		return
	}
	if err := m.sender.SendPriceAlert(ctx, item); err != nil {
		m.logger.WarnContext(ctx, "price alert Telegram delivery failed", "alert_id", item.Alert.ID, "error", err)
	}
}
