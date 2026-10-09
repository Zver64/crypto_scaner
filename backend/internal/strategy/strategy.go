// Package strategy holds the administrator's strategies, CEL expressions over
// the configured indicators, and alerts when a monitored instrument starts
// matching one of them.
package strategy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"crypto-scanner/internal/closedindicator"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/platform/numeric"
	"crypto-scanner/internal/scannerindicator"
)

const (
	maxNameLength = 64
	// maxMessageLength keeps alerts well below Telegram's 4096-character
	// message limit.
	maxMessageLength = 1000
)

var (
	ErrInvalidArgument = errors.New("invalid strategy")
	ErrNotFound        = errors.New("strategy not found")
	// ErrConflict means another strategy already has the name.
	ErrConflict = errors.New("strategy name already exists")
	// ErrInstrumentsInUse means that a change would take instruments that
	// strategies read out of the administrator's favorites;
	// InstrumentsInUseError names them.
	ErrInstrumentsInUse = errors.New("strategies read the instruments")
)

// InstrumentUse names the strategies that read one instrument.
type InstrumentUse struct {
	Symbol     string
	Strategies []string
}

// InstrumentsInUseError lists the instruments strategies read that a removal
// from the administrator's favorites would take away from them.
type InstrumentsInUseError struct {
	Uses []InstrumentUse
}

func (err *InstrumentsInUseError) Error() string {
	parts := make([]string, len(err.Uses))
	for i, use := range err.Uses {
		parts[i] = use.Symbol + " (" + strings.Join(use.Strategies, ", ") + ")"
	}
	return ErrInstrumentsInUse.Error() + ": " + strings.Join(parts, "; ")
}

func (err *InstrumentsInUseError) Is(target error) bool { return target == ErrInstrumentsInUse }

// Direction is the price move a strategy trades or a signal expects after
// its entry signals: up, down, or, for a signal only, sideways.
type Direction string

const (
	DirectionLong     Direction = "long"
	DirectionShort    Direction = "short"
	DirectionSideways Direction = "sideways"
)

// gross is the fractional return of a position in direction opened at entry
// and closed at exit, before fees. A short one loses at most everything, as
// if liquidated, however far the price rose.
func (direction Direction) gross(entry, exit float64) float64 {
	if direction == DirectionShort {
		return max(1-exit/entry, -1)
	}
	return exit/entry - 1
}

// Strategy is one stored strategy. Signal makes it a signal that only
// announces its entry signals, expecting the move of Direction; otherwise it
// trades in Direction, long or short. Expression is the entry rule;
// ExitExpression, when not empty, the exit rule; TakeProfitExpression and
// StopLossExpression, when not empty, the prices a trade closes at,
// evaluated when its entry signals. A trading strategy with any of these
// exits holds one buy per trade; one without buys at every entry signal and
// never sells. A short strategy always has a take profit and a stop loss. A
// signal has none of them.
type Strategy struct {
	ID                   int64
	Name                 string
	Signal               bool
	Direction            Direction
	Expression           string
	ExitExpression       string
	TakeProfitExpression string
	StopLossExpression   string
	// MarketCap limits the coins the running strategy buys or a running
	// signal announces; backtests ignore it.
	MarketCap MarketCapRange
	// Message replaces the generated alert text when it is not empty.
	Message string
	Enabled bool
	// BaselinePending reports, as loaded, that the trading states still have
	// to start afresh, such as after a restart right after a change.
	BaselinePending bool
	// Revision grows with every change of how the strategy trades and with
	// every enabled change; state writes apply only to the revision that was
	// evaluated.
	Revision int64
}

// MarketCapRange is a range of market caps in USD; a nil bound is open.
type MarketCapRange struct {
	MinUSD, MaxUSD *float64
}

// Contains reports whether a coin of the market cap, nil when unknown, lies
// within the range. Without bounds every coin does; with any, only coins of a
// known market cap.
func (bounds MarketCapRange) Contains(usd *float64) bool {
	if bounds.MinUSD == nil && bounds.MaxUSD == nil {
		return true
	}
	return usd != nil && (bounds.MinUSD == nil || *usd >= *bounds.MinUSD) && (bounds.MaxUSD == nil || *usd <= *bounds.MaxUSD)
}

// String writes the range as "$10M – $500M", "≥ $1B", or "≤ $500M", and
// as nothing without bounds.
func (bounds MarketCapRange) String() string {
	switch {
	case bounds.MinUSD != nil && bounds.MaxUSD != nil:
		return formatUSD(*bounds.MinUSD) + " – " + formatUSD(*bounds.MaxUSD)
	case bounds.MinUSD != nil:
		return "≥ " + formatUSD(*bounds.MinUSD)
	case bounds.MaxUSD != nil:
		return "≤ " + formatUSD(*bounds.MaxUSD)
	}
	return ""
}

// formatUSD writes an amount compactly, such as $150M or $1.5B.
func formatUSD(usd float64) string {
	for _, scale := range []struct {
		suffix string
		size   float64
	}{{"T", 1e12}, {"B", 1e9}, {"M", 1e6}, {"K", 1e3}} {
		// Rounding first lets 999,995,000 read $1B rather than $1000M.
		if rounded := math.Round(usd/scale.size*100) / 100; rounded >= 1 {
			return "$" + strconv.FormatFloat(rounded, 'f', -1, 64) + scale.suffix
		}
	}
	return "$" + strconv.FormatFloat(usd, 'f', -1, 64)
}

// validate rejects bounds that are not positive and finite, and a minimum
// above the maximum.
func (bounds MarketCapRange) validate() error {
	for _, bound := range []*float64{bounds.MinUSD, bounds.MaxUSD} {
		if bound != nil && !(*bound > 0 && numeric.Finite(*bound)) {
			return fmt.Errorf("%w: market cap bounds must be positive", ErrInvalidArgument)
		}
	}
	if bounds.MinUSD != nil && bounds.MaxUSD != nil && *bounds.MinUSD > *bounds.MaxUSD {
		return fmt.Errorf("%w: the minimum market cap must not exceed the maximum", ErrInvalidArgument)
	}
	return nil
}

// trades reports whether other, of the same kind and direction, differs from
// strategy in how it trades or signals.
func (strategy Strategy) trades(other Strategy) bool {
	return strategy.Expression != other.Expression || strategy.ExitExpression != other.ExitExpression ||
		strategy.TakeProfitExpression != other.TakeProfitExpression || strategy.StopLossExpression != other.StopLossExpression
}

// Entry is a strategy with its compiled expressions. Compiled, the entry
// rule, is nil when a stored source no longer compiles, and such a strategy
// is not evaluated; Exit, TakeProfit, and StopLoss are nil without their
// source.
type Entry struct {
	Strategy
	Compiled             *Expression
	Exit                 *Expression
	TakeProfit, StopLoss *Expression
	// Interval is the finest interval the expressions read, whose candles
	// the strategy trades on.
	Interval market.CandleInterval
	// Problem explains why a stored source no longer compiles.
	Problem string
	// reads lists what the expressions read, each read once, and symbols
	// the other instruments they read through of, ascending.
	reads   []Read
	symbols []string
}

// evaluated reports whether the monitor evaluates the strategy.
func (entry Entry) evaluated() bool { return entry.Enabled && entry.Compiled != nil }

// Exits reports whether the strategy sells: through its exit rule, take
// profit, or stop loss.
func (entry Entry) Exits() bool {
	return entry.Exit != nil || entry.TakeProfit != nil || entry.StopLoss != nil
}

// indicatorIDs lists the indicators the rules read, ascending.
func (entry Entry) indicatorIDs() []int64 {
	var ids []int64
	for _, read := range entry.reads {
		if id := read.Variable.IndicatorID; id != 0 && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids
}

type Store interface {
	ListStrategies(context.Context) ([]Strategy, error)
	// CreateStrategy stores the strategy, at revision 1, with the indicators
	// and the other instruments it reads and returns its id. It fails with
	// ErrConflict for a taken name and with ErrInvalidArgument when an
	// indicator no longer exists or an instrument it starts reading is not
	// in the favorites of administratorTelegramID.
	CreateStrategy(ctx context.Context, item Strategy, indicatorIDs []int64, symbols []string, administratorTelegramID int64) (int64, error)
	// UpdateStrategy replaces the name, rules, trading settings, message,
	// indicators, and instruments and returns the revision; baseline marks
	// the trading states for a fresh start. It fails like CreateStrategy, or
	// with ErrNotFound.
	UpdateStrategy(ctx context.Context, item Strategy, indicatorIDs []int64, symbols []string, administratorTelegramID int64, baseline bool) (int64, error)
	// SetStrategyEnabled returns the revision and fails with ErrNotFound.
	// Enabling marks the trading states for a fresh start; disabling
	// forgets them.
	SetStrategyEnabled(context.Context, int64, bool) (int64, error)
	DeleteStrategy(context.Context, int64) error
	// ListStrategyInstruments returns the active favorites of the
	// administrator.
	ListStrategyInstruments(ctx context.Context, administratorTelegramID int64) ([]Instrument, error)
	// GetActiveInstrumentBySymbol fails with market.ErrInstrumentNotFound.
	GetActiveInstrumentBySymbol(context.Context, string) (market.Instrument, error)
	// ListLatestCandles returns up to limit latest closed candles per
	// instrument in chronological order.
	ListLatestCandles(context.Context, []int64, market.CandleInterval, int) (map[int64][]market.Candle, error)
}

// Indicators supplies the configured indicators expressions may read.
type Indicators interface {
	List() []scannerindicator.Entry
	// Preview derives the entry of an indicator that is not configured.
	Preview(scannerindicator.Indicator) (scannerindicator.Entry, error)
}

// Service keeps the strategies in memory and persists every change.
type Service struct {
	store      Store
	indicators Indicators
	// registry checks that the history kept covers what expressions read.
	registry *indicator.Registry
	logger   *slog.Logger
	changed  func(baselines []int64)
	// administratorID keeps in favorites the coins expressions read
	// through of.
	administratorID int64

	// writes serializes changes, so name checks see every earlier change.
	writes  sync.Mutex
	mu      sync.RWMutex
	entries []Entry
}

// NewService creates an empty service; Load reads the stored strategies.
// Expressions read through of the coins in the favorites of
// administratorID. changed receives the ids of strategies whose trading
// states must start afresh after each change and must not block.
func NewService(store Store, indicators Indicators, registry *indicator.Registry, administratorID int64, logger *slog.Logger, changed func(baselines []int64)) (*Service, error) {
	if store == nil || indicators == nil || registry == nil || logger == nil || changed == nil {
		return nil, errors.New("strategy store, indicators, registry, logger, and change listener are required")
	}
	return &Service{store: store, indicators: indicators, registry: registry, administratorID: administratorID, logger: logger.With("module", "strategy"), changed: changed}, nil
}

// RuleKind tells what an expression is to a strategy.
type RuleKind int

const (
	// EntryRule is the entry rule, a condition.
	EntryRule RuleKind = iota
	// ExitRule is the exit rule, a condition that may also read the position
	// variables.
	ExitRule
	// PriceRule is a take profit or stop loss price.
	PriceRule
)

// compile compiles source as a rule of kind and checks that the synchronized
// history covers its deepest reads, so a strategy never waits for values that
// cannot exist.
func (service *Service) compile(source string, variables []Variable, kind RuleKind) (*Expression, error) {
	var compiled *Expression
	var err error
	switch kind {
	case PriceRule:
		compiled, err = CompilePrice(source, variables)
	case ExitRule:
		compiled, err = Compile(source, append(slices.Clone(variables), PositionVariables()...))
	default:
		compiled, err = Compile(source, variables)
	}
	if err != nil {
		return nil, err
	}
	deepest := map[string]Read{}
	for _, read := range compiled.Reads() {
		key := read.Variable.Target.Key()
		if current, ok := deepest[key]; !ok || read.Shift > current.Shift {
			deepest[key] = read
		}
	}
	var problems []string
	for _, key := range slices.Sorted(maps.Keys(deepest)) {
		target := deepest[key].Variable.Target
		depth, err := closedindicator.Depth(service.registry, target, deepest[key].Shift+1)
		if err != nil {
			return nil, err
		}
		if depth > market.SyncDepth {
			problems = append(problems, fmt.Sprintf("%s %s needs %d closed candles, %d are synchronized", target.Interval, target.Selection.Type, depth, market.SyncDepth))
		}
	}
	if len(problems) > 0 {
		return nil, invalidExpression(problems...)
	}
	return compiled, nil
}

// Validation is the result of checking an expression.
type Validation struct {
	// Problems would reject the expression.
	Problems []string
	// Missing are the indicators the expression reads by names no
	// configured indicator has but exactly one new indicator would, in
	// reading order. They need not be added: expressions read them all the
	// same.
	Missing []scannerindicator.Entry
}

// Validate lists every problem that would reject expression as a rule of
// kind: the problems of compiling it, and the coins it reads through of that
// are not the administrator's active favorites. Names of indicators that are
// not configured are resolved into Missing.
func (service *Service) Validate(ctx context.Context, expression string, kind RuleKind) (Validation, error) {
	expression = strings.TrimSpace(expression)
	variables, missing := service.resolve(rule{expression, kind})
	compiled, err := service.compile(expression, variables, kind)
	var invalid *InvalidExpressionError
	if errors.As(err, &invalid) {
		return Validation{Problems: invalid.Problems, Missing: missing}, nil
	}
	if err != nil {
		return Validation{}, err
	}
	favorites, err := service.Symbols(ctx)
	if err != nil {
		return Validation{}, err
	}
	problems := []string{}
	for _, symbol := range compiled.Symbols() {
		if !slices.Contains(favorites, symbol) {
			problems = append(problems, symbol+" is not an active coin in the administrator's favorites")
		}
	}
	return Validation{Problems: problems, Missing: missing}, nil
}

// rule is an expression of a strategy and what it is to it.
type rule struct {
	source string
	kind   RuleKind
}

// resolve lists the variables rules may read: those of the configured
// indicators and, for names no configured indicator has, those of the
// indicators missingIndicators resolves them into, which it also returns.
// The variables of resolved indicators belong to no indicator, so they are
// read but never linked and keep no indicator configured.
func (service *Service) resolve(rules ...rule) ([]Variable, []scannerindicator.Entry) {
	configured := service.indicators.List()
	configuredVariables := Variables(configured)
	var unknown []string
	for _, rule := range rules {
		_, err := service.compile(rule.source, configuredVariables, rule.kind)
		var invalid *InvalidExpressionError
		if errors.As(err, &invalid) {
			unknown = append(unknown, invalid.Unknown...)
		}
	}
	missing := service.missingIndicators(unknown)
	if len(missing) == 0 {
		return configuredVariables, nil
	}
	return variables(configured, missing), missing
}

// Unconfigured lists the indicators entry reads that are not configured
// now, in reading order: those it compiled without and those removed since.
func (service *Service) Unconfigured(entry Entry) []scannerindicator.Entry {
	configured := service.indicators.List()
	var missing []scannerindicator.Entry
	for _, read := range entry.reads {
		target := read.Variable.Target
		same := func(other scannerindicator.Entry) bool { return other.Target().Equal(target) }
		if read.Variable.Position || target.Equal(CandleTarget(target.Interval)) || slices.ContainsFunc(configured, same) || slices.ContainsFunc(missing, same) {
			continue
		}
		if found, err := service.indicators.Preview(scannerindicator.Indicator{Interval: target.Interval, Selection: target.Selection}); err == nil {
			missing = append(missing, found)
		}
	}
	return missing
}

// compileEntry compiles the expressions of item over the configured
// indicators and those the names of the others resolve into, and finds the
// interval it trades on. Problems of the other expressions than the entry
// rule name them.
func (service *Service) compileEntry(item Strategy) (Entry, error) {
	entry := Entry{Strategy: item}
	rules := []rule{{item.Expression, EntryRule}}
	for _, other := range []rule{{item.ExitExpression, ExitRule}, {item.TakeProfitExpression, PriceRule}, {item.StopLossExpression, PriceRule}} {
		if other.source != "" {
			rules = append(rules, other)
		}
	}
	variables, _ := service.resolve(rules...)
	compiled, err := service.compile(item.Expression, variables, EntryRule)
	if err != nil {
		return entry, err
	}
	optional := func(source, name string, kind RuleKind) (*Expression, error) {
		if source == "" {
			return nil, nil
		}
		expression, err := service.compile(source, variables, kind)
		var invalid *InvalidExpressionError
		if errors.As(err, &invalid) {
			for index, problem := range invalid.Problems {
				invalid.Problems[index] = name + ": " + problem
			}
		}
		return expression, err
	}
	if entry.Exit, err = optional(item.ExitExpression, "exit rule", ExitRule); err != nil {
		return entry, err
	}
	if entry.TakeProfit, err = optional(item.TakeProfitExpression, "take profit", PriceRule); err != nil {
		return entry, err
	}
	if entry.StopLoss, err = optional(item.StopLossExpression, "stop loss", PriceRule); err != nil {
		return entry, err
	}
	entry.Compiled = compiled
	entry.reads = mergeReads(compiled, entry.Exit, entry.TakeProfit, entry.StopLoss)
	for _, read := range entry.reads {
		if read.Symbol != "" && !slices.Contains(entry.symbols, read.Symbol) {
			entry.symbols = append(entry.symbols, read.Symbol)
		}
	}
	slices.Sort(entry.symbols)
	// The entry rule reads at least one interval.
	for _, interval := range market.CandleIntervals() {
		if slices.ContainsFunc(entry.reads, func(read Read) bool { return read.Variable.Target.Interval == interval }) {
			entry.Interval = interval
			break
		}
	}
	return entry, nil
}

// mergeReads lists what the entry rule and the other expressions, which may
// be nil, read, each read once.
func mergeReads(compiled *Expression, others ...*Expression) []Read {
	reads := compiled.Reads()
	type key struct {
		name, symbol string
		shift        int
	}
	seen := make(map[key]struct{}, len(reads))
	for _, read := range reads {
		seen[key{read.Variable.Name, read.Symbol, read.Shift}] = struct{}{}
	}
	for _, other := range others {
		if other == nil {
			continue
		}
		for _, read := range other.Reads() {
			if _, ok := seen[key{read.Variable.Name, read.Symbol, read.Shift}]; !ok {
				seen[key{read.Variable.Name, read.Symbol, read.Shift}] = struct{}{}
				reads = append(reads, read)
			}
		}
	}
	return reads
}

// Load replaces the strategies with the stored ones. A strategy that no
// longer compiles is kept but not evaluated.
func (service *Service) Load(ctx context.Context) error {
	stored, err := service.store.ListStrategies(ctx)
	if err != nil {
		return fmt.Errorf("load strategies: %w", err)
	}
	entries := make([]Entry, len(stored))
	for i, item := range stored {
		entry, err := service.compileEntry(item)
		if err != nil {
			service.logger.WarnContext(ctx, "skip invalid strategy", "strategy_id", item.ID, "error", err)
			entry.Problem = err.Error()
		}
		entries[i] = entry
	}
	service.writes.Lock()
	defer service.writes.Unlock()
	service.replace(entries, nil)
	return nil
}

// List returns the strategies ordered by creation.
func (service *Service) List() []Entry {
	service.mu.RLock()
	defer service.mu.RUnlock()
	return slices.Clone(service.entries)
}

// Variables lists what expressions can read now.
func (service *Service) Variables() []Variable {
	return Variables(service.indicators.List())
}

// Symbols lists the instruments of can read: the administrator's active
// favorites, ascending.
func (service *Service) Symbols(ctx context.Context) ([]string, error) {
	instruments, err := service.store.ListStrategyInstruments(ctx, service.administratorID)
	if err != nil {
		return nil, err
	}
	symbols := make([]string, len(instruments))
	for i, instrument := range instruments {
		symbols[i] = instrument.Symbol
	}
	slices.Sort(symbols)
	return symbols, nil
}

// IndicatorUsage maps each indicator id to the names of the strategies that
// read it.
func (service *Service) IndicatorUsage() map[int64][]string {
	service.mu.RLock()
	defer service.mu.RUnlock()
	usage := map[int64][]string{}
	for _, entry := range service.entries {
		if entry.Compiled == nil {
			continue
		}
		for _, id := range entry.indicatorIDs() {
			usage[id] = append(usage[id], entry.Name)
		}
	}
	return usage
}

// Subscriptions returns the values enabled strategies read, with the points
// they need, which must stay current in the background: each value of the
// evaluated instrument on every instrument they evaluate, and each value read
// through of only on the instrument it names.
func (service *Service) Subscriptions(ctx context.Context) ([]closedindicator.Subscription, error) {
	instruments, err := service.store.ListStrategyInstruments(ctx, service.administratorID)
	if err != nil {
		return nil, err
	}
	var evaluated []Entry
	for _, entry := range service.List() {
		if entry.evaluated() {
			evaluated = append(evaluated, entry)
		}
	}
	return readsOf(evaluated, instruments, instruments).subscriptions, nil
}

// Create validates and stores a strategy.
func (service *Service) Create(ctx context.Context, item Strategy) (Entry, error) {
	entry, err := service.entry(item)
	if err != nil {
		return Entry{}, err
	}
	service.writes.Lock()
	defer service.writes.Unlock()
	entry.ID, err = service.store.CreateStrategy(ctx, entry.Strategy, entry.indicatorIDs(), entry.symbols, service.administratorID)
	if err != nil {
		return Entry{}, err
	}
	entry.Revision = 1
	var baselines []int64
	if entry.Enabled {
		baselines = []int64{entry.ID}
	}
	service.replace(append(service.List(), entry), baselines)
	return entry, nil
}

// Update changes the name, rules, trading settings, and message of the
// strategy item.ID; its enabled state stays. A change of how an enabled
// strategy trades starts it afresh, forgetting its open trades.
func (service *Service) Update(ctx context.Context, item Strategy) (Entry, error) {
	service.writes.Lock()
	defer service.writes.Unlock()
	current := service.List()
	index := slices.IndexFunc(current, func(entry Entry) bool { return entry.ID == item.ID })
	if index < 0 {
		return Entry{}, ErrNotFound
	}
	previous := current[index]
	if item.Signal != previous.Signal || item.Direction != previous.Direction {
		return Entry{}, fmt.Errorf("%w: a saved strategy or signal keeps its kind and direction; create a new one", ErrInvalidArgument)
	}
	item.Enabled = previous.Enabled
	entry, err := service.entry(item)
	if err != nil {
		return Entry{}, err
	}
	var baselines []int64
	if entry.Enabled && (previous.trades(entry.Strategy) || previous.Compiled == nil) {
		baselines = []int64{item.ID}
	}
	entry.Revision, err = service.store.UpdateStrategy(ctx, entry.Strategy, entry.indicatorIDs(), entry.symbols, service.administratorID, len(baselines) > 0)
	if err != nil {
		return Entry{}, err
	}
	current[index] = entry
	service.replace(current, baselines)
	return entry, nil
}

// SetEnabled turns evaluation on or off. Enabling starts the trading states
// afresh.
func (service *Service) SetEnabled(ctx context.Context, id int64, enabled bool) (Entry, error) {
	service.writes.Lock()
	defer service.writes.Unlock()
	current := service.List()
	index := slices.IndexFunc(current, func(entry Entry) bool { return entry.ID == id })
	if index < 0 {
		return Entry{}, ErrNotFound
	}
	if current[index].Enabled == enabled {
		return current[index], nil
	}
	revision, err := service.store.SetStrategyEnabled(ctx, id, enabled)
	if err != nil {
		return Entry{}, err
	}
	current[index].Revision = revision
	var baselines []int64
	if enabled {
		baselines = []int64{id}
	}
	current[index].Enabled = enabled
	service.replace(current, baselines)
	return current[index], nil
}

// Delete removes the strategy and its trading states.
func (service *Service) Delete(ctx context.Context, id int64) error {
	service.writes.Lock()
	defer service.writes.Unlock()
	current := service.List()
	index := slices.IndexFunc(current, func(entry Entry) bool { return entry.ID == id })
	if index < 0 {
		return ErrNotFound
	}
	if err := service.store.DeleteStrategy(ctx, id); err != nil {
		return err
	}
	service.replace(slices.Delete(current, index, index+1), nil)
	return nil
}

// replace must be called with writes held. The listener runs under the
// lock, so whoever lists a changed strategy finds its baseline requested.
func (service *Service) replace(entries []Entry, baselines []int64) {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.entries = entries
	service.changed(baselines)
}

func (service *Service) entry(item Strategy) (Entry, error) {
	item.Name = strings.TrimSpace(item.Name)
	if item.Name == "" || utf8.RuneCountInString(item.Name) > maxNameLength {
		return Entry{}, fmt.Errorf("%w: the name must have 1 to %d characters", ErrInvalidArgument, maxNameLength)
	}
	item.Message = strings.TrimSpace(item.Message)
	if utf8.RuneCountInString(item.Message) > maxMessageLength {
		return Entry{}, fmt.Errorf("%w: the message must have at most %d characters", ErrInvalidArgument, maxMessageLength)
	}
	if err := item.MarketCap.validate(); err != nil {
		return Entry{}, err
	}
	item.Expression = strings.TrimSpace(item.Expression)
	item.ExitExpression = strings.TrimSpace(item.ExitExpression)
	item.TakeProfitExpression = strings.TrimSpace(item.TakeProfitExpression)
	item.StopLossExpression = strings.TrimSpace(item.StopLossExpression)
	switch {
	case item.Direction != DirectionLong && item.Direction != DirectionShort && item.Direction != DirectionSideways:
		return Entry{}, fmt.Errorf("%w: the direction must be long, short, or sideways", ErrInvalidArgument)
	case item.Signal:
		if item.ExitExpression != "" || item.TakeProfitExpression != "" || item.StopLossExpression != "" {
			return Entry{}, fmt.Errorf("%w: a signal has no exit rule, take profit, or stop loss", ErrInvalidArgument)
		}
	case item.Direction == DirectionSideways:
		return Entry{}, fmt.Errorf("%w: a strategy trades long or short; only a signal expects a sideways move", ErrInvalidArgument)
	case item.Direction == DirectionShort && (item.TakeProfitExpression == "" || item.StopLossExpression == ""):
		return Entry{}, fmt.Errorf("%w: a short strategy needs a take profit and a stop loss", ErrInvalidArgument)
	}
	entry, err := service.compileEntry(item)
	if err != nil {
		return Entry{}, err
	}
	return entry, nil
}
