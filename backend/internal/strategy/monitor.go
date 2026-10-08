package strategy

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
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
	// MarketCapUSD is the current market cap, nil when unknown.
	MarketCapUSD *float64
}

type MonitorStore interface {
	// ListStrategyInstruments returns the active favorites of the
	// administrator, which strategies evaluate.
	ListStrategyInstruments(ctx context.Context, administratorTelegramID int64) ([]Instrument, error)
	// ListStrategyStates maps strategy ids to the trading states of their
	// instruments.
	ListStrategyStates(context.Context) (map[int64]map[int64]TradeState, error)
	// ReplaceStrategyStates stores the baseline states of revision, which
	// hold no trades, and completes its pending baseline. It reports false,
	// changing nothing, when that revision no longer awaits a baseline.
	ReplaceStrategyStates(ctx context.Context, strategyID, revision int64, states map[int64]TradeState) (bool, error)
	// SaveStrategyStates stores states of later candles than the stored ones
	// while revision is current and announced, and returns the instruments
	// stored.
	SaveStrategyStates(ctx context.Context, strategyID, revision int64, states map[int64]TradeState) ([]int64, error)
	DeleteStrategyStates(ctx context.Context, strategyID int64, instrumentIDs []int64) error
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
	// write failed, conflicted, or was rejected. Only Run's goroutine uses it.
	resync bool

	mu sync.Mutex
	// pending are instruments with changed values; full requests evaluating
	// every instrument; baselines are strategies to start afresh.
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

// StrategiesChanged schedules a full evaluation and starts the trading states
// of baselines afresh. It never blocks.
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
	state := map[int64]map[int64]TradeState{}
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
				if err := monitor.loadStates(ctx, state); err != nil {
					monitor.logger.WarnContext(ctx, "reload strategy states failed", "error", err)
					monitor.signal()
					continue
				}
				monitor.resync = false
			}
			monitor.evaluate(ctx, state)
		}
	}
}

// loadStates replaces state with the stored trading states.
func (monitor *Monitor) loadStates(ctx context.Context, state map[int64]map[int64]TradeState) error {
	stored, err := monitor.store.ListStrategyStates(ctx)
	if err != nil {
		return fmt.Errorf("load strategy states: %w", err)
	}
	clear(state)
	maps.Copy(state, stored)
	return nil
}

// desync reloads the stored states and evaluates every instrument next
// round, so a state whose write outcome is unknown is neither lost nor
// repeated.
func (monitor *Monitor) desync() {
	monitor.resync = true
	monitor.Changed()
}

// evaluate applies one round of pending work. Work that fails before any
// change is stored stays queued for the next round.
func (monitor *Monitor) evaluate(ctx context.Context, state map[int64]map[int64]TradeState) {
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
	// Disabled and deleted strategies forget their states in the store.
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
	// Recipients are known before any state is stored, so a stored signal is
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
		symbols := entry.symbols
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
		states := state[entry.ID]
		if states == nil {
			states = map[int64]TradeState{}
			state[entry.ID] = states
		}
		// Instruments that left the favorites forget their states.
		var left []int64
		for id := range states {
			if _, ok := monitored[id]; !ok {
				left = append(left, id)
			}
		}
		if len(left) > 0 {
			if err := monitor.store.DeleteStrategyStates(ctx, entry.ID, left); err != nil {
				monitor.logger.WarnContext(ctx, "forget strategy states failed", "strategy_id", entry.ID, "error", err)
				monitor.desync()
			} else {
				for _, id := range left {
					delete(states, id)
				}
			}
		}

		if _, baseline := baselines[entry.ID]; baseline {
			// The baseline waits for the first calculation of what it reads,
			// so it never misses an entry that is true; that calculation
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

		changes := map[int64]TradeState{}
		signals := map[int64][]TradeEvent{}
		for _, instrument := range candidatesOf(entry) {
			next, events, processed := current.advance(entry, instrument.ID, states[instrument.ID], !entry.MarketCap.Contains(instrument.MarketCapUSD))
			if processed {
				changes[instrument.ID] = next
				signals[instrument.ID] = events
			}
		}
		if len(changes) == 0 {
			continue
		}
		// Only states stored for a later candle alert, so a strategy changed
		// meanwhile, or a repeated evaluation, never alerts.
		stored, err := monitor.store.SaveStrategyStates(ctx, entry.ID, entry.Revision, changes)
		if err != nil {
			monitor.logger.WarnContext(ctx, "store strategy states failed", "strategy_id", entry.ID, "error", err)
			monitor.desync()
			continue
		}
		// States not stored, or a rejected stale revision, mean the local
		// state differs from the store.
		if len(stored) < len(changes) {
			monitor.desync()
		}
		for _, instrument := range instruments {
			if !slices.Contains(stored, instrument.ID) {
				continue
			}
			states[instrument.ID] = changes[instrument.ID]
			for _, event := range signals[instrument.ID] {
				if event.Kind != TradeSkip {
					monitor.send(ctx, recipients, alertText(entry, instrument, event, current))
				}
			}
		}
	}
}

// baseline starts the trading states of entry afresh from the entry values
// now, without trades, and announces the instruments within its market cap
// range whose entry is true. Instruments with unknown entries are left out and signal once their entry
// turns true.
func (monitor *Monitor) baseline(ctx context.Context, state map[int64]map[int64]TradeState, entry Entry, instruments []Instrument, current snapshot, recipients []int64) {
	at := entry.Interval.LastClosedOpenTime(current.now)
	states := map[int64]TradeState{}
	var matching []Instrument
	for _, instrument := range instruments {
		result, known := current.match(entry.Compiled, entry, instrument.ID, nil)
		if !known {
			continue
		}
		states[instrument.ID] = TradeState{OpenTime: at, Entry: result}
		if result && entry.MarketCap.Contains(instrument.MarketCapUSD) {
			matching = append(matching, instrument)
		}
	}
	replaced, err := monitor.store.ReplaceStrategyStates(ctx, entry.ID, entry.Revision, states)
	if err != nil {
		// The transaction may have committed; the stored flag decides
		// whether the retry is still needed.
		monitor.logger.WarnContext(ctx, "store strategy states failed", "strategy_id", entry.ID, "error", err)
		monitor.StrategiesChanged([]int64{entry.ID})
		monitor.desync()
		return
	}
	if !replaced {
		// A newer change requested its own baseline, or an earlier attempt of
		// this one committed; either way the stored states are current.
		monitor.desync()
		return
	}
	state[entry.ID] = states
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

// match evaluates expression, a rule of entry, over the fresh values of one
// instrument, the instruments it reads through of, and positions.
func (current snapshot) match(expression *Expression, entry Entry, instrumentID int64, positions map[string]float64) (bool, bool) {
	return expression.Evaluate(current.resolver(entry, instrumentID, positions))
}

// resolver reads the values of entry on the evaluated instrument, and the
// position variables from positions.
func (current snapshot) resolver(entry Entry, instrumentID int64, positions map[string]float64) func(Read) (float64, bool) {
	return func(read Read) (float64, bool) {
		if read.Variable.Position {
			value, ok := positions[read.Variable.Name]
			return value, ok
		}
		output, ok := current.output(entry, read, instrumentID)
		if !ok {
			return 0, false
		}
		return output.At(read.Shift)
	}
}

// advance processes the latest closed candle of the interval of entry on
// one instrument after state; see Entry.step. outOfRange skips its entry
// signal. Without the fresh candle nothing is processed.
func (current snapshot) advance(entry Entry, instrumentID int64, state TradeState, outOfRange bool) (TradeState, []TradeEvent, bool) {
	at := entry.Interval.LastClosedOpenTime(current.now)
	position, ok := current.reads.positions[pair{instrumentID, CandleTarget(entry.Interval).Key()}]
	if !ok || !current.values[position].OpenTime.Equal(at) {
		return state, nil, false
	}
	candle := TradeCandle{
		OpenTime:   at,
		OutOfRange: outOfRange,
		Exit: func(positions map[string]float64) (bool, bool) {
			return current.match(entry.Exit, entry, instrumentID, positions)
		},
		Levels: func() (float64, float64, bool) {
			takeProfit, knownTakeProfit := current.price(entry.TakeProfit, entry, instrumentID)
			stopLoss, knownStopLoss := current.price(entry.StopLoss, entry, instrumentID)
			return takeProfit, stopLoss, knownTakeProfit && knownStopLoss
		},
	}
	for _, output := range current.values[position].Outputs {
		switch output.Name {
		case "open":
			candle.Open = output.Value
		case "high":
			candle.High = output.Value
		case "low":
			candle.Low = output.Value
		case "close":
			candle.Close = output.Value
		}
	}
	candle.Entry, candle.EntryKnown = current.match(entry.Compiled, entry, instrumentID, nil)
	return entry.step(state, candle)
}

// price evaluates the price expression, which may be nil for a price the
// strategy lacks, 0 and known then, on the evaluated instrument.
func (current snapshot) price(expression *Expression, entry Entry, instrumentID int64) (float64, bool) {
	if expression == nil {
		return 0, true
	}
	return expression.Price(current.resolver(entry, instrumentID, nil))
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
		// The candles of the interval of entry price its trades.
		candles := Read{Variable: Variable{Target: CandleTarget(entry.Interval)}}
		for _, instrument := range evaluated {
			subscribe(instrument.ID, candles, candles.Variable.Target.Key())
		}
		for _, read := range entry.reads {
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
	return slices.ContainsFunc(entry.reads, func(read Read) bool {
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

// maxSummaryLength keeps summaries below Telegram's 4096-character message
// limit; the symbols that do not fit are counted instead.
const maxSummaryLength = 4000

// summaryText announces an enabled or changed strategy, its market cap
// range, and the instruments within it whose entry is true now, which buy only once it turns true again, and warns
// about instruments it reads that are not monitored, such as delisted ones,
// which leave its results unknown.
func summaryText(entry Entry, matching []Instrument, current snapshot) string {
	text := "🎯 " + entry.Name + " is active\nEntry: " + entry.Expression + "\n"
	if entry.Exit != nil {
		text += "Exit: " + entry.ExitExpression + "\n"
	}
	if entry.TakeProfit != nil {
		text += "Take profit: " + entry.TakeProfitExpression + "\n"
	}
	if entry.StopLoss != nil {
		text += "Stop loss: " + entry.StopLossExpression + "\n"
	}
	if !entry.Exits() {
		text += "No exit: every entry signal buys.\n"
	}
	if bounds := entry.MarketCap.String(); bounds != "" {
		text += "Market cap: " + bounds + "\n"
	}
	var absent []string
	for _, symbol := range entry.symbols {
		if _, ok := current.bySymbol[symbol]; !ok {
			absent = append(absent, symbol)
		}
	}
	if len(absent) > 0 {
		text += "⚠️ Not in favorites: " + strings.Join(absent, ", ") + "\n"
	}
	if len(matching) == 0 {
		return text + "No coin's entry is true now."
	}
	symbols := make([]string, len(matching))
	for i, instrument := range matching {
		symbols[i] = instrument.Symbol
	}
	slices.Sort(symbols)
	text += "Entry true now, buying once it turns true again: "
	for i, symbol := range symbols {
		more := fmt.Sprintf(" and %d more", len(symbols)-i)
		if len(text)+len(symbol)+2+len(more) > maxSummaryLength {
			return strings.TrimSuffix(text, ", ") + more
		}
		text += symbol + ", "
	}
	return strings.TrimSuffix(text, ", ")
}

// alertText names the strategy, the instrument, and the buy, with the take
// profit and stop loss of the trade it opens, or the sell of event and what
// sold it, then the rule that signaled it and the values that rule read,
// named as it writes them. A sell at a take
// profit or stop loss has no rule. A strategy message replaces the rule and
// the values.
func alertText(entry Entry, instrument Instrument, event TradeEvent, current snapshot) string {
	title := "🟢 " + entry.Name + ": " + instrument.Symbol + " buy #" + strconv.Itoa(event.Buy) + " at " + formatNumber(event.Close)
	if event.Trade.TakeProfit > 0 {
		title += ", take profit " + formatNumber(event.Trade.TakeProfit)
	}
	if event.Trade.StopLoss > 0 {
		title += ", stop loss " + formatNumber(event.Trade.StopLoss)
	}
	rule, source := entry.Compiled, entry.Expression
	if event.Kind == TradeSell {
		price, reason := event.Close, "exit rule"
		switch event.Reason {
		case ExitTakeProfit:
			price, reason = event.Price, "take profit"
		case ExitStopLoss:
			price, reason = event.Price, "stop loss"
		}
		title = fmt.Sprintf("🔴 %s: %s sell %d buys at %s by %s, entry %s, pnl %+.2f%%",
			entry.Name, instrument.Symbol, event.Trade.Buys, formatNumber(price), reason, formatNumber(event.Trade.EntryPrice()), 100*event.Return)
		rule, source = entry.Exit, entry.ExitExpression
	}
	if entry.Message != "" {
		return title + "\n" + entry.Message
	}
	if event.Kind == TradeSell && event.Reason != ExitRuleSignal {
		return title
	}
	lines := []string{title, source}
	var positions map[string]float64
	if event.Kind == TradeSell {
		positions = entry.positions(event.Trade, entry.Interval.LastClosedOpenTime(current.now), event.Close)
	}
	values := rule.Values(current.resolver(entry, instrument.ID, positions))
	readings := make([]string, 0, len(values))
	for _, label := range slices.Sorted(maps.Keys(values)) {
		readings = append(readings, label+" "+formatNumber(values[label]))
	}
	if len(readings) > 0 {
		lines = append(lines, strings.Join(readings, " · "))
	}
	return strings.Join(lines, "\n")
}

func formatNumber(value float64) string {
	return strconv.FormatFloat(value, 'g', 6, 64)
}
