package strategy

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"crypto-scanner/internal/closedindicator"
)

const (
	// settleDelay groups the recalculations of one candle close, which arrive
	// per interval and indicator, into one evaluation.
	settleDelay  = 10 * time.Second
	fullPassTime = 5 * time.Minute
	sendWorkers  = 4
)

// Instrument is one instrument strategies evaluate: an active favorite of
// the administrator.
type Instrument struct {
	ID     int64
	Symbol string
}

type MonitorStore interface {
	// ListStrategyInstruments returns the active favorites of the
	// administrator, which strategies evaluate.
	ListStrategyInstruments(ctx context.Context, administratorTelegramID int64) ([]Instrument, error)
	// ListStrategyMatches maps strategy ids to their matching instruments.
	ListStrategyMatches(context.Context) (map[int64][]int64, error)
	// ReplaceStrategyMatches stores the announced matches of revision and
	// completes its pending baseline. It reports false, changing nothing,
	// when that revision no longer awaits a baseline.
	ReplaceStrategyMatches(ctx context.Context, strategyID, revision int64, instrumentIDs []int64) (bool, error)
	// AddStrategyMatches adds matches while revision is current and
	// announced, and returns the instruments that were not matching before.
	AddStrategyMatches(ctx context.Context, strategyID, revision int64, instrumentIDs []int64) ([]int64, error)
	DeleteStrategyMatches(ctx context.Context, strategyID int64, instrumentIDs []int64) error
	// ListStrategyRecipients returns the Telegram IDs that receive strategy
	// messages: the administrator and users with strategy alerts.
	ListStrategyRecipients(ctx context.Context, administratorTelegramID int64) ([]int64, error)
}

// Values supplies the tracked closed indicator values.
type Values interface {
	Listen(func(closedindicator.Change))
	// Snapshot returns the value of each subscription and also lists the
	// targets some subscribed instrument has no value for yet, or none
	// calculated with the subscribed points.
	Snapshot([]closedindicator.Subscription) ([]closedindicator.Value, []closedindicator.Target)
}

// Strategies supplies the current strategies. A change requests its
// baseline before List can return the changed strategy.
type Strategies interface {
	List() []Entry
}

type Sender interface {
	SendStrategyMessage(ctx context.Context, telegramID int64, text string) error
}

// Monitor evaluates enabled strategies whenever indicator values of monitored
// instruments change and alerts when an instrument starts matching.
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
	// resync reloads the stored matches before the next round, after a match
	// write failed, conflicted, or was rejected. Only Run's goroutine uses it.
	resync bool

	mu sync.Mutex
	// pending are instruments with changed values; full requests evaluating
	// every instrument; baselines are strategies to announce.
	pending   map[int64]struct{}
	full      bool
	baselines map[int64]struct{}
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
		pending: map[int64]struct{}{}, baselines: map[int64]struct{}{},
	}
	values.Listen(func(change closedindicator.Change) {
		monitor.mu.Lock()
		monitor.pending[change.InstrumentID] = struct{}{}
		monitor.mu.Unlock()
		monitor.signal()
	})
	return monitor
}

// StrategiesChanged schedules a full evaluation and announces the current
// matches of baselines. It never blocks.
func (monitor *Monitor) StrategiesChanged(baselines []int64) {
	monitor.mu.Lock()
	monitor.full = true
	for _, id := range baselines {
		monitor.baselines[id] = struct{}{}
	}
	monitor.mu.Unlock()
	monitor.signal()
}

// Changed schedules a full evaluation after the monitored instruments change.
func (monitor *Monitor) Changed() { monitor.StrategiesChanged(nil) }

func (monitor *Monitor) signal() {
	select {
	case monitor.wake <- struct{}{}:
	default:
	}
}

// Run evaluates strategies until ctx is cancelled.
func (monitor *Monitor) Run(ctx context.Context) error {
	state := map[int64]map[int64]struct{}{}
	if err := monitor.loadMatches(ctx, state); err != nil {
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
	// Baselines requested before a restart are announced now.
	var pending []int64
	for _, entry := range monitor.strategies.List() {
		if entry.BaselinePending && entry.Enabled {
			pending = append(pending, entry.ID)
		}
	}
	monitor.StrategiesChanged(pending)
	ticker := time.NewTicker(fullPassTime)
	defer ticker.Stop()
	settle := time.NewTimer(settleDelay)
	settle.Stop()
	defer settle.Stop()
	settling := false
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			monitor.Changed()
		case <-monitor.wake:
			if !settling {
				settle.Reset(settleDelay)
				settling = true
			}
		case <-settle.C:
			settling = false
			if monitor.resync {
				if err := monitor.loadMatches(ctx, state); err != nil {
					monitor.logger.WarnContext(ctx, "reload strategy matches failed", "error", err)
					monitor.signal()
					continue
				}
				monitor.resync = false
			}
			monitor.evaluate(ctx, state)
		}
	}
}

// loadMatches replaces state with the stored matches.
func (monitor *Monitor) loadMatches(ctx context.Context, state map[int64]map[int64]struct{}) error {
	matches, err := monitor.store.ListStrategyMatches(ctx)
	if err != nil {
		return fmt.Errorf("load strategy matches: %w", err)
	}
	clear(state)
	for id, instruments := range matches {
		state[id] = set(instruments)
	}
	return nil
}

// desync reloads the stored matches and evaluates every instrument next
// round, so a match whose write outcome is unknown is neither lost nor kept
// forever.
func (monitor *Monitor) desync() {
	monitor.resync = true
	monitor.Changed()
}

// evaluate applies one round of pending work. Work that fails before any
// change is stored stays queued for the next round.
func (monitor *Monitor) evaluate(ctx context.Context, state map[int64]map[int64]struct{}) {
	monitor.mu.Lock()
	pending, full, baselines := monitor.pending, monitor.full, monitor.baselines
	monitor.pending, monitor.full, monitor.baselines = map[int64]struct{}{}, false, map[int64]struct{}{}
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
		monitor.signal()
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
	// Disabled and deleted strategies forget their matches in the store.
	for id := range state {
		if !slices.ContainsFunc(strategies, func(entry Entry) bool { return entry.ID == id }) {
			delete(state, id)
		}
	}
	if len(strategies) == 0 {
		return
	}
	instruments, err := monitor.store.ListStrategyInstruments(ctx, monitor.administratorID)
	if err != nil {
		monitor.logger.WarnContext(ctx, "load monitored instruments failed", "error", err)
		requeue()
		return
	}
	// Recipients are known before any match is stored, so a stored match is
	// never left without its alert.
	recipients, err := monitor.store.ListStrategyRecipients(ctx, monitor.administratorID)
	if err != nil {
		monitor.logger.WarnContext(ctx, "load strategy recipients failed", "error", err)
		requeue()
		return
	}
	monitored := make(map[int64]Instrument, len(instruments))
	for _, instrument := range instruments {
		monitored[instrument.ID] = instrument
	}
	var changed []Instrument
	for id := range pending {
		if instrument, ok := monitored[id]; ok {
			changed = append(changed, instrument)
		}
	}
	// A change of an instrument a strategy reads through of changes its
	// results for every instrument.
	candidatesOf := func(entry Entry) []Instrument {
		symbols := entry.Compiled.Symbols()
		if full || slices.ContainsFunc(changed, func(instrument Instrument) bool {
			return slices.Contains(symbols, instrument.Symbol)
		}) {
			return instruments
		}
		return changed
	}
	current := snapshot{reads: readsOf(strategies, instruments, instruments), now: monitor.now(), bySymbol: bySymbol(instruments)}
	var missing []closedindicator.Target
	current.values, missing = monitor.values.Snapshot(current.reads.subscriptions)

	for _, entry := range strategies {
		matched := state[entry.ID]
		if matched == nil {
			matched = map[int64]struct{}{}
			state[entry.ID] = matched
		}
		// Instruments that left the favorites forget their matches.
		var left []int64
		for id := range matched {
			if _, ok := monitored[id]; !ok {
				left = append(left, id)
			}
		}
		if len(left) > 0 {
			if err := monitor.store.DeleteStrategyMatches(ctx, entry.ID, left); err != nil {
				monitor.logger.WarnContext(ctx, "forget strategy matches failed", "strategy_id", entry.ID, "error", err)
				monitor.desync()
			} else {
				for _, id := range left {
					delete(matched, id)
				}
			}
		}

		if _, baseline := baselines[entry.ID]; baseline {
			// The baseline waits for the first calculation of what it reads,
			// so it never leaves out instruments that match; that calculation
			// reports changes, which evaluate again.
			if readsAny(entry, missing) {
				monitor.mu.Lock()
				monitor.baselines[entry.ID] = struct{}{}
				monitor.mu.Unlock()
				continue
			}
			monitor.baseline(ctx, state, entry, instruments, current, recipients)
			continue
		}

		var started, stopped []Instrument
		for _, instrument := range candidatesOf(entry) {
			result, known := current.match(entry, instrument.ID)
			if !known {
				continue
			}
			_, was := matched[instrument.ID]
			switch {
			case result && !was:
				started = append(started, instrument)
			case !result && was:
				stopped = append(stopped, instrument)
			}
		}
		if len(stopped) > 0 {
			if err := monitor.store.DeleteStrategyMatches(ctx, entry.ID, instrumentIDs(stopped)); err != nil {
				monitor.logger.WarnContext(ctx, "forget strategy matches failed", "strategy_id", entry.ID, "error", err)
				monitor.desync()
			} else {
				for _, instrument := range stopped {
					delete(matched, instrument.ID)
				}
			}
		}
		if len(started) == 0 {
			continue
		}
		// Only instruments stored as new matches alert, so a strategy
		// changed meanwhile, or a repeated evaluation, never alerts.
		added, err := monitor.store.AddStrategyMatches(ctx, entry.ID, entry.Revision, instrumentIDs(started))
		if err != nil {
			monitor.logger.WarnContext(ctx, "store strategy matches failed", "strategy_id", entry.ID, "error", err)
			monitor.desync()
			continue
		}
		// Instruments already stored, or a rejected stale revision, mean the
		// local state differs from the store.
		if len(added) < len(started) {
			monitor.desync()
		}
		for _, instrument := range started {
			if !slices.Contains(added, instrument.ID) {
				continue
			}
			matched[instrument.ID] = struct{}{}
			monitor.send(ctx, recipients, alertText(entry, instrument, current))
		}
	}
}

// baseline replaces the matches of entry with the instruments matching now
// and announces them. Instruments with unknown results are left out and alert
// once they match.
func (monitor *Monitor) baseline(ctx context.Context, state map[int64]map[int64]struct{}, entry Entry, instruments []Instrument, current snapshot, recipients []int64) {
	var matching []Instrument
	for _, instrument := range instruments {
		if result, known := current.match(entry, instrument.ID); known && result {
			matching = append(matching, instrument)
		}
	}
	replaced, err := monitor.store.ReplaceStrategyMatches(ctx, entry.ID, entry.Revision, instrumentIDs(matching))
	if err != nil {
		// The transaction may have committed; the stored flag decides
		// whether the retry is still needed.
		monitor.logger.WarnContext(ctx, "store strategy matches failed", "strategy_id", entry.ID, "error", err)
		monitor.StrategiesChanged([]int64{entry.ID})
		monitor.desync()
		return
	}
	if !replaced {
		// A newer change requested its own baseline, or an earlier attempt of
		// this one committed; either way the stored matches are current.
		monitor.desync()
		return
	}
	state[entry.ID] = set(instrumentIDs(matching))
	monitor.send(ctx, recipients, summaryText(entry, matching, current))
}

// snapshot is the tracked values of one evaluation round.
type snapshot struct {
	reads reads
	// values holds the value of each subscription of reads.
	values []closedindicator.Value
	// bySymbol finds the monitored instruments read through of.
	bySymbol map[string]int64
	now      time.Time
}

// match evaluates entry over the fresh values of one instrument and the
// instruments it reads through of.
func (current snapshot) match(entry Entry, instrumentID int64) (bool, bool) {
	return entry.Compiled.Evaluate(func(read Read) (float64, bool) {
		output, ok := current.output(entry, read, instrumentID)
		if !ok {
			return 0, false
		}
		return output.At(read.Shift)
	})
}

// output returns the fresh output read reads of the evaluated instrument or
// of the instrument read names. Values whose latest candle is older are stale
// and left out.
func (current snapshot) output(entry Entry, read Read, instrumentID int64) (closedindicator.Output, bool) {
	if read.Symbol != "" {
		var ok bool
		if instrumentID, ok = current.bySymbol[read.Symbol]; !ok {
			return closedindicator.Output{}, false
		}
	}
	position, ok := current.reads.positions[pair{instrumentID, current.reads.keys[entry.ID][read.Variable.Name]}]
	if !ok {
		return closedindicator.Output{}, false
	}
	value := current.values[position]
	if !value.OpenTime.Equal(read.Variable.Target.Interval.LastClosedOpenTime(current.now)) {
		return closedindicator.Output{}, false
	}
	for _, output := range value.Outputs {
		if output.Name == read.Variable.Output {
			return output, true
		}
	}
	return closedindicator.Output{}, false
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

// pair identifies the value of one target of one instrument.
type pair struct {
	instrumentID int64
	target       string
}

// reads are the values strategies read in one round.
type reads struct {
	// subscriptions are the distinct pairs, each with the points its deepest
	// read needs.
	subscriptions []closedindicator.Subscription
	positions     map[pair]int
	// keys maps the variables of each strategy to the keys of their targets.
	keys map[int64]map[string]string
}

// readsOf subscribes every read of the evaluated instrument on each of
// evaluated and every read through of only on the instrument it names among
// named.
func readsOf(strategies []Entry, evaluated, named []Instrument) reads {
	result := reads{positions: map[pair]int{}, keys: make(map[int64]map[string]string, len(strategies))}
	symbols := bySymbol(named)
	subscribe := func(instrumentID int64, read Read, key string) {
		at := pair{instrumentID, key}
		position, ok := result.positions[at]
		if !ok {
			position = len(result.subscriptions)
			result.positions[at] = position
			result.subscriptions = append(result.subscriptions, closedindicator.Subscription{InstrumentID: instrumentID, Target: read.Variable.Target})
		}
		result.subscriptions[position].Points = max(result.subscriptions[position].Points, read.Shift+1)
	}
	for _, entry := range strategies {
		keys := map[string]string{}
		result.keys[entry.ID] = keys
		for _, read := range entry.Compiled.Reads() {
			key, ok := keys[read.Variable.Name]
			if !ok {
				key = read.Variable.Target.Key()
				keys[read.Variable.Name] = key
			}
			if read.Symbol != "" {
				if id, ok := symbols[read.Symbol]; ok {
					subscribe(id, read, key)
				}
				continue
			}
			for _, instrument := range evaluated {
				subscribe(instrument.ID, read, key)
			}
		}
	}
	return result
}

// readsAny reports whether entry reads any of targets.
func readsAny(entry Entry, targets []closedindicator.Target) bool {
	return slices.ContainsFunc(entry.Compiled.Reads(), func(read Read) bool {
		return slices.ContainsFunc(targets, read.Variable.Target.Equal)
	})
}

func bySymbol(instruments []Instrument) map[string]int64 {
	result := make(map[string]int64, len(instruments))
	for _, instrument := range instruments {
		result[instrument.Symbol] = instrument.ID
	}
	return result
}

func set(ids []int64) map[int64]struct{} {
	result := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		result[id] = struct{}{}
	}
	return result
}

func instrumentIDs(instruments []Instrument) []int64 {
	ids := make([]int64, len(instruments))
	for i, instrument := range instruments {
		ids[i] = instrument.ID
	}
	return ids
}

// maxSummaryLength keeps summaries below Telegram's 4096-character message
// limit; the symbols that do not fit are counted instead.
const maxSummaryLength = 4000

// summaryText announces the instruments matching an enabled or changed
// strategy and warns about instruments it reads that are not monitored, such
// as delisted ones, which leave its results unknown.
func summaryText(entry Entry, matching []Instrument, current snapshot) string {
	text := "🎯 " + entry.Name + " is active\n" + entry.Expression + "\n"
	var absent []string
	for _, symbol := range entry.Compiled.Symbols() {
		if _, ok := current.bySymbol[symbol]; !ok {
			absent = append(absent, symbol)
		}
	}
	if len(absent) > 0 {
		text += "⚠️ Not in favorites: " + strings.Join(absent, ", ") + "\n"
	}
	if len(matching) == 0 {
		return text + "No coins match now."
	}
	symbols := make([]string, len(matching))
	for i, instrument := range matching {
		symbols[i] = instrument.Symbol
	}
	slices.Sort(symbols)
	text += "Matching now: "
	for i, symbol := range symbols {
		more := fmt.Sprintf(" and %d more", len(symbols)-i)
		if len(text)+len(symbol)+2+len(more) > maxSummaryLength {
			return strings.TrimSuffix(text, ", ") + more
		}
		text += symbol + ", "
	}
	return strings.TrimSuffix(text, ", ")
}

// alertText names the strategy, the instrument, and the latest values it
// read, those of other instruments after their symbol. A strategy message
// replaces the expression and the values.
func alertText(entry Entry, instrument Instrument, current snapshot) string {
	title := "🎯 " + entry.Name + ": " + instrument.Symbol
	if entry.Message != "" {
		return title + "\n" + entry.Message
	}
	lines := []string{title, entry.Expression}
	var readings []string
	shown := map[string]struct{}{}
	for _, read := range entry.Compiled.Reads() {
		name := read.Variable.Name
		if read.Symbol != "" {
			name = read.Symbol + " " + name
		}
		if _, ok := shown[name]; ok {
			continue
		}
		shown[name] = struct{}{}
		if output, ok := current.output(entry, read, instrument.ID); ok {
			readings = append(readings, name+" "+strconv.FormatFloat(output.Value, 'g', 6, 64))
		}
	}
	if len(readings) > 0 {
		lines = append(lines, strings.Join(readings, " · "))
	}
	return strings.Join(lines, "\n")
}
