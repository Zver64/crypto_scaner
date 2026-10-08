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
	"slices"
	"strings"
	"sync"
	"unicode/utf8"

	"crypto-scanner/internal/closedindicator"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/scannerindicator"
)

const (
	maxNameLength = 64
	// MaxBuys bounds the max buys of a trade a strategy may set.
	MaxBuys = 1000
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

// Strategy is one stored strategy. Expression is the entry rule;
// ExitExpression, when not empty, the exit rule. Accumulate lets entry
// signals add buys to an open trade, which only a strategy with an exit has;
// one without an exit buys at every entry signal and never sells. MaxBuys
// caps the buys of a trade, 0 meaning no cap.
type Strategy struct {
	ID             int64
	Name           string
	Expression     string
	ExitExpression string
	Accumulate     bool
	MaxBuys        int
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

// trades reports whether other differs from strategy in how it trades.
func (strategy Strategy) trades(other Strategy) bool {
	return strategy.Expression != other.Expression || strategy.ExitExpression != other.ExitExpression ||
		strategy.Accumulate != other.Accumulate || strategy.MaxBuys != other.MaxBuys
}

// Entry is a strategy with its compiled expressions. Compiled, the entry
// rule, is nil when a stored source no longer compiles, and such a strategy
// is not evaluated; Exit is nil without an exit rule.
type Entry struct {
	Strategy
	Compiled *Expression
	Exit     *Expression
	// Interval is the finest interval the expressions read, whose candles
	// the strategy trades on.
	Interval market.CandleInterval
	// Problem explains why a stored source no longer compiles.
	Problem string
	// reads lists what the entry and the exit rules read, each read once,
	// and symbols the other instruments they read through of, ascending.
	reads   []Read
	symbols []string
}

// evaluated reports whether the monitor evaluates the strategy.
func (entry Entry) evaluated() bool { return entry.Enabled && entry.Compiled != nil }

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
	// CapacityProblems names the intervals that cannot take the added
	// indicators.
	CapacityProblems(added []scannerindicator.Entry) []string
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

// compile compiles source and checks that the synchronized history covers its
// deepest reads, so a strategy never waits for values that cannot exist.
func (service *Service) compile(source string, variables []Variable) (*Expression, error) {
	compiled, err := Compile(source, variables)
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
	// Problems would reject the expression; none means it is valid once
	// the missing indicators are added.
	Problems []string
	// Missing are the indicators the expression reads by names no
	// configured indicator has but exactly one new indicator would, in
	// reading order.
	Missing []scannerindicator.Entry
}

// Validate lists every problem that would reject expression as the entry
// rule of a strategy, or as its exit rule when exit is set: the problems of
// compiling it, and the coins it reads through of that are not the
// administrator's active favorites. Names of indicators that are not
// configured yet are resolved into Missing, and the problems assume they
// were added.
func (service *Service) Validate(ctx context.Context, expression string, exit bool) (Validation, error) {
	compiled, missing, err := service.compileResolving(expression, exit)
	var invalid *InvalidExpressionError
	if errors.As(err, &invalid) {
		return Validation{Problems: append(invalid.Problems, service.indicators.CapacityProblems(missing)...), Missing: missing}, nil
	}
	if err != nil {
		return Validation{}, err
	}
	favorites, err := service.Symbols(ctx)
	if err != nil {
		return Validation{}, err
	}
	problems := service.indicators.CapacityProblems(missing)
	for _, symbol := range compiled.Symbols() {
		if !slices.Contains(favorites, symbol) {
			problems = append(problems, symbol+" is not an active coin in the administrator's favorites")
		}
	}
	return Validation{Problems: problems, Missing: missing}, nil
}

// compileResolving compiles expression, an entry rule or, when exit is set,
// an exit rule, over the configured indicators and, for the names of
// indicators that are not configured, over the indicators missingIndicators
// resolves them into, which it returns.
func (service *Service) compileResolving(expression string, exit bool) (*Expression, []scannerindicator.Entry, error) {
	expression = strings.TrimSpace(expression)
	configured := service.indicators.List()
	compiled, err := service.compile(expression, ruleVariables(Variables(configured), exit))
	var invalid *InvalidExpressionError
	if !errors.As(err, &invalid) || len(invalid.Unknown) == 0 {
		return compiled, nil, err
	}
	missing := service.missingIndicators(configured, invalid.Unknown)
	if len(missing) > 0 {
		compiled, err = service.compile(expression, ruleVariables(Variables(append(slices.Clone(configured), missing...)), exit))
	}
	return compiled, missing, err
}

// ruleVariables adds the position variables to variables for an exit rule.
func ruleVariables(variables []Variable, exit bool) []Variable {
	if !exit {
		return variables
	}
	return append(slices.Clone(variables), PositionVariables()...)
}

// compileEntry compiles the rules of item over variables and finds the
// interval it trades on. Problems of the exit rule name it.
func (service *Service) compileEntry(item Strategy, variables []Variable) (Entry, error) {
	entry := Entry{Strategy: item}
	compiled, err := service.compile(item.Expression, variables)
	if err != nil {
		return entry, err
	}
	var exit *Expression
	if item.ExitExpression != "" {
		if exit, err = service.compile(item.ExitExpression, ruleVariables(variables, true)); err != nil {
			var invalid *InvalidExpressionError
			if errors.As(err, &invalid) {
				for index, problem := range invalid.Problems {
					invalid.Problems[index] = "exit rule: " + problem
				}
			}
			return entry, err
		}
	}
	entry.Compiled, entry.Exit = compiled, exit
	entry.reads = mergeReads(compiled, exit)
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

// mergeReads lists what the entry rule and the exit rule, which may be nil,
// read, each read once.
func mergeReads(compiled, exit *Expression) []Read {
	reads := compiled.Reads()
	if exit == nil {
		return reads
	}
	type key struct {
		name, symbol string
		shift        int
	}
	seen := make(map[key]struct{}, len(reads))
	for _, read := range reads {
		seen[key{read.Variable.Name, read.Symbol, read.Shift}] = struct{}{}
	}
	for _, read := range exit.Reads() {
		if _, ok := seen[key{read.Variable.Name, read.Symbol, read.Shift}]; !ok {
			reads = append(reads, read)
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
	variables := service.Variables()
	entries := make([]Entry, len(stored))
	for i, item := range stored {
		entry, err := service.compileEntry(item, variables)
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
	if item.MaxBuys < 0 || item.MaxBuys > MaxBuys {
		return Entry{}, fmt.Errorf("%w: max buys must be from 0 to %d", ErrInvalidArgument, MaxBuys)
	}
	item.Expression = strings.TrimSpace(item.Expression)
	item.ExitExpression = strings.TrimSpace(item.ExitExpression)
	// Without an exit every entry signal buys anyway, and a trade of one buy
	// has no max buys to keep.
	if item.ExitExpression == "" {
		item.Accumulate = false
	} else if !item.Accumulate {
		item.MaxBuys = 0
	}
	entry, err := service.compileEntry(item, service.Variables())
	if err != nil {
		return Entry{}, err
	}
	return entry, nil
}
