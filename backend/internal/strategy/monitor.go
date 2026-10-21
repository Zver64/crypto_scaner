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
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/platform/backoff"
	"crypto-scanner/internal/platform/delivery"
	"crypto-scanner/internal/platform/logging"

	"github.com/google/uuid"
)

const (
	sendWorkers = 4
	// retryDelay and maxRetryDelay space the rounds after a failed one.
	retryDelay    = 10 * time.Second
	maxRetryDelay = 5 * time.Minute
	// catchUpLimit bounds the instruments whose missed candles one round
	// catches up, over all strategies; the rest wait for the next rounds.
	catchUpLimit = 20
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
	// targets some subscription has no value for yet.
	Snapshot([]closedindicator.Subscription) ([]closedindicator.Value, []closedindicator.Target)
}

// Strategies supplies the current strategies. A change requests its
// baseline before List can return the changed strategy. DisableAtRevision
// disables a strategy that a rule error halts, only while it still has the
// revision that failed, and reports whether it did. ReloadFavorites rereads
// the administrator's favorites when their last read failed.
type Strategies interface {
	List() []Entry
	DisableAtRevision(ctx context.Context, id uuid.UUID, revision int64) (bool, error)
	ReloadFavorites(ctx context.Context) error
}

// CandleStore reads the stored candles that catch-ups and backtests replay.
type CandleStore interface {
	// ListLatestCandles returns up to limit latest closed candles per
	// instrument in chronological order.
	ListLatestCandles(context.Context, []int64, market.CandleInterval, int) (map[int64][]market.Candle, error)
}

type Sender interface {
	SendStrategyMessage(ctx context.Context, telegramID int64, text string) error
}

// Monitor evaluates enabled strategies whenever indicator values of monitored
// instruments change, processes every closed candle of their interval once
// per instrument, and alerts on the buys and sells it signals.
type Monitor struct {
	store      MonitorStore
	candles    CandleStore
	registry   *indicator.Registry
	values     Values
	strategies Strategies
	sender     Sender
	logger     *slog.Logger
	now        func() time.Time
	// catchUpLimit is catchUpLimit, which tests lower.
	catchUpLimit int
	// administratorID always receives strategy messages.
	administratorID int64

	wake  chan struct{}
	sends chan message
	// resync reloads the stored states before the next round, after a state
	// write failed, conflicted, or was rejected; failed delays the next round
	// after a failed store call, but not after a conflict. Only Run's goroutine uses them.
	resync, failed bool
	// catchUps are the catch-up retries of the instruments of each
	// strategy. Only Run's goroutine uses it.
	catchUps map[uuid.UUID]map[int64]*catchUpRetry
	// failures warns once about each failure that keeps recurring.
	failures *logging.Failures[failureKey]

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

// failureKey names a recurring failure of the monitor: an operation and the
// strategy and instrument it concerns, if any.
type failureKey struct {
	operation  string
	strategy   uuid.UUID
	instrument int64
}

// loadStatesFailure is the failure of loading the stored states.
var loadStatesFailure = failureKey{operation: "load states"}

// NewMonitor creates a monitor and subscribes it to value changes. Messages
// go to administratorID and to users with strategy alerts.
// Missed candles are replayed over the candles stored in candles with the
// indicators of registry.
func NewMonitor(store MonitorStore, candles CandleStore, registry *indicator.Registry, values Values, strategies Strategies, sender Sender, administratorID int64, logger *slog.Logger) *Monitor {
	logger = logger.With("module", "strategy")
	monitor := &Monitor{
		store: store, candles: candles, registry: registry, values: values, strategies: strategies, sender: sender, administratorID: administratorID,
		logger: logger, failures: logging.NewFailures[failureKey](logger), now: time.Now, catchUpLimit: catchUpLimit,
		wake: make(chan struct{}, 1), sends: make(chan message, 256),
		pending: map[int64]struct{}{}, baselines: map[uuid.UUID]struct{}{}, catchUps: map[uuid.UUID]map[int64]*catchUpRetry{},
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

// Run evaluates strategies until ctx is cancelled. It retries loading the
// stored states until it succeeds.
func (monitor *Monitor) Run(ctx context.Context) error {
	state := map[uuid.UUID]map[int64]TradeState{}
	for failures := 0; ; failures++ {
		err := monitor.loadStates(ctx, state)
		if err == nil {
			break
		}
		if ctx.Err() != nil {
			return nil
		}
		monitor.failures.Warn(ctx, loadStatesFailure, "initial strategy states load failed", "error", err)
		if backoff.Sleep(ctx, backoff.Exponential(retryDelay, maxRetryDelay, failures)) != nil {
			return nil
		}
	}
	monitor.failures.Recover(ctx, loadStatesFailure, "strategy states load recovered")
	// The alerts queued at shutdown, whose states are stored, are still
	// sent.
	waitSends := delivery.Start(ctx, sendWorkers, monitor.sends, monitor.deliver)
	defer waitSends()
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
	// catchUp fires at the earliest retry of a delayed catch-up.
	catchUp := time.NewTimer(retryDelay)
	catchUp.Stop()
	defer catchUp.Stop()
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
		case <-catchUp.C:
			// The retried catch-ups evaluate every instrument, once no
			// failed round delays them.
			monitor.Changed()
			continue
		}
		monitor.round(ctx, state)
		if at, ok := monitor.nextCatchUp(); ok {
			catchUp.Reset(at.Sub(monitor.now()))
		} else {
			catchUp.Stop()
		}
		if !monitor.failed {
			failures = 0
			continue
		}
		monitor.failed = false
		retry.Reset(backoff.Exponential(retryDelay, maxRetryDelay, failures))
		failures++
	}
}

// round reloads the stored states when they may differ and the favorites
// when their last read failed, and evaluates the pending work, with the
// favorites last read when they cannot be reloaded.
func (monitor *Monitor) round(ctx context.Context, state map[uuid.UUID]map[int64]TradeState) {
	// Stale favorites do not hold up the round; its failure, which the
	// strategies log, only retries it.
	favoritesErr := monitor.strategies.ReloadFavorites(ctx)
	defer func() {
		if favoritesErr != nil {
			monitor.failed = true
		}
	}()
	if monitor.resync {
		if err := monitor.loadStates(ctx, state); err != nil {
			monitor.failures.Warn(ctx, loadStatesFailure, "reload strategy states failed", "error", err)
			monitor.failed = true
			return
		}
		monitor.failures.Recover(ctx, loadStatesFailure, "strategy states load recovered")
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
	// Disabled and deleted strategies forget their states in the store,
	// and their catch-ups.
	evaluated := func(id uuid.UUID) bool {
		return slices.ContainsFunc(strategies, func(entry Entry) bool { return entry.ID == id })
	}
	maps.DeleteFunc(state, func(id uuid.UUID, _ map[int64]TradeState) bool { return !evaluated(id) })
	maps.DeleteFunc(monitor.catchUps, func(id uuid.UUID, _ map[int64]*catchUpRetry) bool { return !evaluated(id) })
	monitor.failures.Retain(func(key failureKey) bool { return key.strategy == uuid.Nil || evaluated(key.strategy) })
	if len(strategies) == 0 {
		return
	}
	instrumentsFailure := failureKey{operation: "load instruments"}
	instruments, err := monitor.store.ListStrategyInstruments(ctx)
	if err != nil {
		monitor.failures.Warn(ctx, instrumentsFailure, "load monitored instruments failed", "error", err)
		requeue()
		return
	}
	monitor.failures.Recover(ctx, instrumentsFailure, "load monitored instruments recovered")
	// Recipients are known before any state is stored, so a stored signal is
	// never left without its alert.
	recipientsFailure := failureKey{operation: "load recipients"}
	recipients, err := monitor.store.ListStrategyRecipients(ctx, monitor.administratorID)
	if err != nil {
		monitor.failures.Warn(ctx, recipientsFailure, "load strategy recipients failed", "error", err)
		requeue()
		return
	}
	monitor.failures.Recover(ctx, recipientsFailure, "load strategy recipients recovered")
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
	work := roundWork{instruments: instruments, changed: changed, full: full, baselines: baselines, recipients: recipients, catchUpLeft: new(monitor.catchUpLimit)}
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

func (monitor *Monitor) deliver(ctx context.Context, item message) {
	if err := monitor.sender.SendStrategyMessage(ctx, item.telegramID, item.text); err != nil {
		monitor.logger.WarnContext(ctx, "strategy Telegram delivery failed", "error", err)
	}
}
