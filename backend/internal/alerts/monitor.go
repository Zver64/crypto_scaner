package alerts

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"sync"
	"time"

	markettrade "crypto-scanner/internal/market/trade"
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
	lastTradeID   int64
	lastEventTime time.Time
	epoch         int64
	fresh         bool
	alerts        map[int64]Alert
}

type Monitor struct {
	store   MonitorStore
	feed    markettrade.Feed
	sender  AlertSender
	logger  *slog.Logger
	ops     chan liveOperation
	refresh chan struct{}
	sends   chan Fired
}

func NewMonitor(store MonitorStore, feed markettrade.Feed, sender AlertSender, logger *slog.Logger) *Monitor {
	if logger == nil {
		logger = slog.Default()
	}
	return &Monitor{store: store, feed: feed, sender: sender, logger: logger, ops: make(chan liveOperation, 128), refresh: make(chan struct{}, 1), sends: make(chan Fired, 128)}
}
func (m *Monitor) Apply(a Alert)           { copy := a; m.ops <- liveOperation{apply: &copy} }
func (m *Monitor) Remove(userID, id int64) { m.ops <- liveOperation{removeUser: userID, removeID: id} }
func (m *Monitor) Changed() {
	select {
	case m.refresh <- struct{}{}:
	default:
	}
}
func (m *Monitor) Run(ctx context.Context) error {
	states := map[string]*symbolState{}
	subscribed := []string(nil)
	if err := m.reload(ctx, states, &subscribed); err != nil {
		return fmt.Errorf("load price alerts: %w", err)
	}
	var workers sync.WaitGroup
	for range 4 {
		workers.Add(1)
		go func() { defer workers.Done(); m.sendLoop(ctx) }()
	}
	defer workers.Wait()
	// Mutations trigger immediate refreshes. This infrequent poll is only a
	// safety net for external access and favorite changes.
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := m.reload(ctx, states, &subscribed); err != nil {
				m.logger.WarnContext(ctx, "refresh price alerts failed", "error", err)
			}
		case <-m.refresh:
			if err := m.reload(ctx, states, &subscribed); err != nil {
				m.logger.WarnContext(ctx, "refresh price alerts failed", "error", err)
			}
		case op := <-m.ops:
			m.operation(ctx, states, op)
			m.syncSubscriptions(states, &subscribed)
		case status := <-m.feed.Statuses():
			if !status.Connected {
				for _, symbol := range status.Symbols {
					if s := states[symbol]; s != nil {
						s.fresh = false
						s.lastPrice = ""
						s.lastTradeID = 0
						s.lastEventTime = time.Time{}
					}
				}
			}
		case event := <-m.feed.Events():
			m.trade(ctx, states, event)
		}
	}
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
		for id, alert := range incoming {
			if existing, ok := s.alerts[id]; !ok || existing.Version != alert.Version {
				s.fresh, s.lastPrice, s.lastTradeID, s.lastEventTime = false, "", 0, time.Time{}
				break
			}
		}
		s.alerts = incoming
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
	sort.Strings(symbols)
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
		wasFresh, price, eventTime := s.fresh, s.lastPrice, s.lastEventTime
		s.alerts[a.ID] = a
		if wasFresh {
			if cmp, err := Compare(a.Target, price); err == nil && cmp == 0 {
				m.fire(ctx, s, a, price, eventTime)
			}
		}
		// A newly created/versioned target must never use the pre-change side of
		// a crossing. The next trade establishes a new baseline.
		s.fresh, s.lastPrice, s.lastTradeID, s.lastEventTime = false, "", 0, time.Time{}
		return
	}
	for _, s := range states {
		if a, ok := s.alerts[op.removeID]; ok && a.UserID == op.removeUser {
			delete(s.alerts, op.removeID)
		}
	}
}
func (m *Monitor) trade(ctx context.Context, states map[string]*symbolState, e markettrade.Event) {
	s := states[e.Symbol]
	if s == nil {
		return
	}
	if s.epoch != e.Epoch {
		s.fresh, s.lastPrice, s.lastTradeID, s.lastEventTime, s.epoch = false, "", 0, time.Time{}, e.Epoch
	}
	if s.fresh && e.TradeID <= s.lastTradeID {
		return
	}
	previous, fresh := s.lastPrice, s.fresh
	s.lastPrice, s.lastTradeID, s.lastEventTime, s.fresh = e.Price, e.TradeID, e.EventTime, true
	for _, a := range s.alerts {
		currentCmp, e1 := Compare(a.Target, e.Price)
		if e1 != nil {
			continue
		}
		hit := currentCmp == 0
		if fresh && !hit {
			previousCmp, e2 := Compare(a.Target, previous)
			hit = e2 == nil && ((previousCmp < 0 && currentCmp > 0) || (previousCmp > 0 && currentCmp < 0))
		}
		if hit {
			m.fire(ctx, s, a, e.Price, e.EventTime)
		}
	}
}
func (m *Monitor) fire(ctx context.Context, s *symbolState, a Alert, price string, at time.Time) {
	ok, err := m.store.FireAlert(ctx, a.ID, a.Version)
	if err != nil {
		m.logger.WarnContext(ctx, "fire price alert failed", "alert_id", a.ID, "error", err)
		return
	}
	delete(s.alerts, a.ID)
	if !ok {
		return
	}
	select {
	case m.sends <- Fired{Alert: a, Price: price, EventTime: at}:
	case <-ctx.Done():
		m.logger.WarnContext(ctx, "price alert delivery cancelled during shutdown", "alert_id", a.ID)
	}
}
func (m *Monitor) sendLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case item := <-m.sends:
			sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := m.sender.SendPriceAlert(sendCtx, item)
			cancel()
			if err != nil {
				m.logger.WarnContext(ctx, "price alert Telegram delivery failed", "alert_id", item.Alert.ID, "error", err)
			}
		}
	}
}
