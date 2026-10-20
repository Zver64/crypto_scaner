package strategy

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sync"
	"time"

	"crypto-scanner/internal/closedindicator"
	"crypto-scanner/internal/platform/backoff"

	"github.com/google/uuid"
)

const (
	sendWorkers = 4
	// retryDelay and maxRetryDelay space the rounds after a failed one.
	retryDelay    = 10 * time.Second
	maxRetryDelay = 5 * time.Minute
)

// Instrument is one active instrument strategies may evaluate.
type Instrument struct {
	ID     int64
	Symbol string
	// MarketCapUSD is the current market cap, nil when unknown.
	MarketCapUSD *float64
}

type MonitorStore interface {
	// ListStrategyInstruments returns the active instruments, among which
	// each strategy evaluates its own.
	ListStrategyInstruments(context.Context) ([]Instrument, error)
	// ListStrategyStates maps strategy ids to the trading states of their
	// instruments.
	ListStrategyStates(context.Context) (map[uuid.UUID]map[int64]TradeState, error)
	// ReplaceStrategyStates stores the baseline states of revision, which
	// hold no trades, and completes its pending baseline. It reports false,
	// changing nothing, when that revision no longer awaits a baseline.
	ReplaceStrategyStates(ctx context.Context, strategyID uuid.UUID, revision int64, states map[int64]TradeState) (bool, error)
	// SaveStrategyStates stores states of later candles than the stored ones
	// while revision is current and announced, and returns the instruments
	// stored.
	SaveStrategyStates(ctx context.Context, strategyID uuid.UUID, revision int64, states map[int64]TradeState) ([]int64, error)
	DeleteStrategyStates(ctx context.Context, strategyID uuid.UUID, instrumentIDs []int64) error
	// ListStrategyRecipients returns the Telegram IDs that receive strategy
	// messages: the administrator and users with strategy alerts.
	ListStrategyRecipients(ctx context.Context, administratorTelegramID int64) ([]int64, error)
}

// Values supplies the tracked closed indicator values.
type Values interface {
	// Listen receives the changes of each tracker step at once.
	Listen(func([]closedindicator.Change))
	// Snapshot returns the value of each subscription and also lists the
	// targets some subscribed instrument has no value for yet, or none
	// calculated with the subscribed points.
	Snapshot([]closedindicator.Subscription) ([]closedindicator.Value, []closedindicator.Target)
}

// Strategies supplies the current strategies. A change requests its
// baseline before List can return the changed strategy. DisableAtRevision
// disables a strategy that a rule error halts, only while it still has the
// revision that failed, and reports whether it did.
type Strategies interface {
	List() []Entry
	DisableAtRevision(ctx context.Context, id uuid.UUID, revision int64) (bool, error)
}

type Sender interface {
	SendStrategyMessage(ctx context.Context, telegramID int64, text string) error
}

// Monitor evaluates enabled strategies whenever indicator values of monitored
// instruments change, processes every closed candle of their interval once
// per instrument, and alerts on the buys and sells it signals.
type Monitor struct {
	store      MonitorStore
	values     Values
	strategies Strategies
	sender     Sender
	logger     *slog.Logger
	now        func() time.Time
	// administratorID always receives strategy messages.
	administratorID int64

	wake  chan struct{}
	sends chan message
	// resync reloads the stored states before the next round, after a state
	// write failed, conflicted, or was rejected; failed delays the next round
	// after a failed store call, but not after a conflict. Only Run's goroutine uses them.
	resync, failed bool

	mu sync.Mutex
	// pending are instruments with changed values; full requests evaluating
	// every instrument; baselines are strategies to start afresh.
	pending   map[int64]struct{}
	full      bool
	baselines map[uuid.UUID]struct{}
}

type message struct {
	telegramID int64
	text       string
}

// NewMonitor creates a monitor and subscribes it to value changes. Messages
// go to administratorID and to users with strategy alerts.
func NewMonitor(store MonitorStore, values Values, strategies Strategies, sender Sender, administratorID int64, logger *slog.Logger) *Monitor {
	monitor := &Monitor{
		store: store, values: values, strategies: strategies, sender: sender, administratorID: administratorID, logger: logger.With("module", "strategy"), now: time.Now,
		wake: make(chan struct{}, 1), sends: make(chan message, 256),
		pending: map[int64]struct{}{}, baselines: map[uuid.UUID]struct{}{},
	}
	// A tracker step, such as that of a synchronization round, is
	// evaluated as one round, so its alerts of a strategy go together.
	values.Listen(func(changes []closedindicator.Change) {
		monitor.mu.Lock()
		for _, change := range changes {
			monitor.pending[change.InstrumentID] = struct{}{}
		}
		monitor.mu.Unlock()
		monitor.signal()
	})
	return monitor
}

// StrategiesChanged schedules a full evaluation and starts the trading states
// of baselines afresh. It never blocks.
func (monitor *Monitor) StrategiesChanged(baselines []uuid.UUID) {
	monitor.mu.Lock()
	monitor.full = true
	for _, id := range baselines {
		monitor.baselines[id] = struct{}{}
	}
	monitor.mu.Unlock()
	monitor.signal()
}

// Changed schedules a full evaluation, such as after the catalog changed,
// which disables the strategies of delisted coins. It never blocks.
func (monitor *Monitor) Changed() { monitor.StrategiesChanged(nil) }

func (monitor *Monitor) signal() {
	select {
	case monitor.wake <- struct{}{}:
	default:
	}
}

// Run evaluates strategies until ctx is cancelled.
func (monitor *Monitor) Run(ctx context.Context) error {
	state := map[uuid.UUID]map[int64]TradeState{}
	if err := monitor.loadStates(ctx, state); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}
	var workers sync.WaitGroup
	for range sendWorkers {
		workers.Add(1)
		go func() { defer workers.Done(); monitor.sendLoop(ctx) }()
	}
	defer workers.Wait()
	// Baselines requested before a restart start now.
	var pending []uuid.UUID
	for _, entry := range monitor.strategies.List() {
		if entry.BaselinePending && entry.Enabled {
			pending = append(pending, entry.ID)
		}
	}
	monitor.StrategiesChanged(pending)
	retry := time.NewTimer(retryDelay)
	retry.Stop()
	defer retry.Stop()
	// failures counts the failed rounds in a row; while any, only the retry
	// starts the next round.
	failures := 0
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-monitor.wake:
			if failures > 0 {
				continue
			}
		case <-retry.C:
		}
		monitor.round(ctx, state)
		if !monitor.failed {
			failures = 0
			continue
		}
		monitor.failed = false
		retry.Reset(backoff.Exponential(retryDelay, maxRetryDelay, failures))
		failures++
	}
}

// round reloads the stored states when they may differ and evaluates the
// pending work.
func (monitor *Monitor) round(ctx context.Context, state map[uuid.UUID]map[int64]TradeState) {
	if monitor.resync {
		if err := monitor.loadStates(ctx, state); err != nil {
			monitor.logger.WarnContext(ctx, "reload strategy states failed", "error", err)
			monitor.failed = true
			return
		}
		monitor.resync = false
	}
	monitor.evaluate(ctx, state)
}

// loadStates replaces state with the stored trading states.
func (monitor *Monitor) loadStates(ctx context.Context, state map[uuid.UUID]map[int64]TradeState) error {
	stored, err := monitor.store.ListStrategyStates(ctx)
	if err != nil {
		return fmt.Errorf("load strategy states: %w", err)
	}
	clear(state)
	maps.Copy(state, stored)
	return nil
}

// desync reloads the stored states and evaluates every instrument next
// round, so a state the store rejected or whose write outcome is unknown is
// neither lost nor repeated.
func (monitor *Monitor) desync() {
	monitor.resync = true
	monitor.Changed()
}

// storeFailed reloads the stored states after a failed state write, whose
// outcome is unknown, and delays the next round.
func (monitor *Monitor) storeFailed() {
	monitor.failed = true
	monitor.desync()
}

// evaluate applies one round of pending work. Work that fails before any
// change is stored stays queued for the next round.
func (monitor *Monitor) evaluate(ctx context.Context, state map[uuid.UUID]map[int64]TradeState) {
	monitor.mu.Lock()
	pending, full, baselines := monitor.pending, monitor.full, monitor.baselines
	monitor.pending, monitor.full, monitor.baselines = map[int64]struct{}{}, false, map[uuid.UUID]struct{}{}
	monitor.mu.Unlock()
	requeue := func() {
		monitor.mu.Lock()
		for id := range pending {
			monitor.pending[id] = struct{}{}
		}
		for id := range baselines {
			monitor.baselines[id] = struct{}{}
		}
		monitor.full = monitor.full || full
		monitor.mu.Unlock()
		monitor.failed = true
	}
	// Listed after draining: a strategy changed meanwhile has its baseline
	// queued again and waits for the next round instead of alerting now. An
	// entry listed before a change has a stale revision, and the store
	// rejects its writes. List runs outside mu, because changes request baselines while
	// holding the strategies lock.
	listed := monitor.strategies.List()
	var strategies []Entry
	monitor.mu.Lock()
	for _, entry := range listed {
		if _, queued := monitor.baselines[entry.ID]; entry.evaluated() && !queued {
			strategies = append(strategies, entry)
		}
	}
	monitor.mu.Unlock()
	// Disabled and deleted strategies forget their states in the store.
	for id := range state {
		if !slices.ContainsFunc(strategies, func(entry Entry) bool { return entry.ID == id }) {
			delete(state, id)
		}
	}
	if len(strategies) == 0 {
		return
	}
	instruments, err := monitor.store.ListStrategyInstruments(ctx)
	if err != nil {
		monitor.logger.WarnContext(ctx, "load monitored instruments failed", "error", err)
		requeue()
		return
	}
	// Recipients are known before any state is stored, so a stored signal is
	// never left without its alert.
	recipients, err := monitor.store.ListStrategyRecipients(ctx, monitor.administratorID)
	if err != nil {
		monitor.logger.WarnContext(ctx, "load strategy recipients failed", "error", err)
		requeue()
		return
	}
	active := make(map[int64]Instrument, len(instruments))
	for _, instrument := range instruments {
		active[instrument.ID] = instrument
	}
	var changed []Instrument
	for id := range pending {
		if instrument, ok := active[id]; ok {
			changed = append(changed, instrument)
		}
	}
	work := roundWork{instruments: instruments, changed: changed, full: full, baselines: baselines, recipients: recipients}
	work.current = snapshot{reads: readsOf(strategies, Entry.instruments, instruments), now: monitor.now(), bySymbol: bySymbol(instruments)}
	work.current.values, work.missing = monitor.values.Snapshot(work.current.reads.subscriptions)
	for _, entry := range strategies {
		monitor.check(ctx, state, entry, work)
	}
}

func (monitor *Monitor) send(ctx context.Context, recipients []int64, text string) {
	for _, telegramID := range recipients {
		select {
		case monitor.sends <- message{telegramID: telegramID, text: text}:
		case <-ctx.Done():
			return
		}
	}
}

func (monitor *Monitor) sendLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case item := <-monitor.sends:
			sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := monitor.sender.SendStrategyMessage(sendCtx, item.telegramID, item.text)
			cancel()
			if err != nil {
				monitor.logger.WarnContext(ctx, "strategy Telegram delivery failed", "error", err)
			}
		}
	}
}
