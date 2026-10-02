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

// Strategy is one stored strategy.
type Strategy struct {
	ID         int64
	Name       string
	Expression string
	// Message replaces the generated alert text when it is not empty.
	Message string
	Enabled bool
	// BaselinePending reports, as loaded, that the current matches still
	// have to be announced, such as after a restart right after a change.
	BaselinePending bool
	// Revision grows with every expression or enabled change; match writes
	// apply only to the revision that was evaluated.
	Revision int64
}

// Entry is a strategy with its compiled expression. Expression is nil when
// the stored source no longer compiles, and such a strategy is not evaluated.
type Entry struct {
	Strategy
	Compiled *Expression
}

// evaluated reports whether the monitor evaluates the strategy.
func (entry Entry) evaluated() bool { return entry.Enabled && entry.Compiled != nil }

type Store interface {
	ListStrategies(context.Context) ([]Strategy, error)
	// CreateStrategy stores the strategy, at revision 1, with the indicators
	// and the other instruments it reads and returns its id. It fails with
	// ErrConflict for a taken name and with ErrInvalidArgument when an
	// indicator no longer exists or an instrument it starts reading is not
	// in the favorites of administratorTelegramID.
	CreateStrategy(ctx context.Context, item Strategy, indicatorIDs []int64, symbols []string, administratorTelegramID int64) (int64, error)
	// UpdateStrategy replaces the name, expression, indicators, and
	// instruments and returns the revision; baseline marks the matches for
	// announcement. It fails like CreateStrategy, or with ErrNotFound.
	UpdateStrategy(ctx context.Context, item Strategy, indicatorIDs []int64, symbols []string, administratorTelegramID int64, baseline bool) (int64, error)
	// SetStrategyEnabled returns the revision and fails with ErrNotFound.
	// Enabling marks the matches for announcement; disabling forgets them.
	SetStrategyEnabled(context.Context, int64, bool) (int64, error)
	DeleteStrategy(context.Context, int64) error
	// ListStrategyInstruments returns the active favorites of the
	// administrator.
	ListStrategyInstruments(ctx context.Context, administratorTelegramID int64) ([]Instrument, error)
}

// Indicators supplies the configured indicators expressions may read.
type Indicators interface {
	List() []scannerindicator.Entry
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
// administratorID. changed receives the ids of strategies whose current
// matches must be announced after each change and must not block.
func NewService(store Store, indicators Indicators, registry *indicator.Registry, administratorID int64, logger *slog.Logger, changed func(baselines []int64)) (*Service, error) {
	if store == nil || indicators == nil || registry == nil || logger == nil || changed == nil {
		return nil, errors.New("strategy store, indicators, registry, logger, and change listener are required")
	}
	return &Service{store: store, indicators: indicators, registry: registry, administratorID: administratorID, logger: logger.With("module", "strategy"), changed: changed}, nil
}

// compile compiles source and checks that the kept history covers its
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
		if depth > market.HistoryDepth {
			problems = append(problems, fmt.Sprintf("%s %s needs %d closed candles, %d are kept", target.Interval, target.Selection.Type, depth, market.HistoryDepth))
		}
	}
	if len(problems) > 0 {
		return nil, invalidExpression(problems...)
	}
	return compiled, nil
}

// Validate lists every problem that would reject expression in a strategy:
// the problems of compiling it, and the coins it reads through of that are
// not the administrator's active favorites. An empty list means it is valid.
func (service *Service) Validate(ctx context.Context, expression string) ([]string, error) {
	compiled, err := service.compile(strings.TrimSpace(expression), service.Variables())
	var invalid *InvalidExpressionError
	if errors.As(err, &invalid) {
		return invalid.Problems, nil
	}
	if err != nil {
		return nil, err
	}
	favorites, err := service.Symbols(ctx)
	if err != nil {
		return nil, err
	}
	problems := []string{}
	for _, symbol := range compiled.Symbols() {
		if !slices.Contains(favorites, symbol) {
			problems = append(problems, symbol+" is not an active coin in the administrator's favorites")
		}
	}
	return problems, nil
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
		entries[i] = Entry{Strategy: item}
		compiled, err := service.compile(item.Expression, variables)
		if err != nil {
			service.logger.WarnContext(ctx, "skip invalid strategy", "strategy_id", item.ID, "error", err)
			continue
		}
		entries[i].Compiled = compiled
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
		for _, id := range entry.Compiled.IndicatorIDs() {
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
	return readsOf(evaluated, instruments).subscriptions, nil
}

// Create validates and stores a strategy.
func (service *Service) Create(ctx context.Context, item Strategy) (Entry, error) {
	entry, err := service.entry(item)
	if err != nil {
		return Entry{}, err
	}
	service.writes.Lock()
	defer service.writes.Unlock()
	entry.ID, err = service.store.CreateStrategy(ctx, entry.Strategy, entry.Compiled.IndicatorIDs(), entry.Compiled.Symbols(), service.administratorID)
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

// Update changes the name, expression, and message. A changed expression of
// an enabled strategy announces its current matches again.
func (service *Service) Update(ctx context.Context, id int64, name, expression, message string) (Entry, error) {
	service.writes.Lock()
	defer service.writes.Unlock()
	current := service.List()
	index := slices.IndexFunc(current, func(entry Entry) bool { return entry.ID == id })
	if index < 0 {
		return Entry{}, ErrNotFound
	}
	previous := current[index]
	entry, err := service.entry(Strategy{ID: id, Name: name, Expression: expression, Message: message, Enabled: previous.Enabled})
	if err != nil {
		return Entry{}, err
	}
	var baselines []int64
	if entry.Enabled && (previous.Expression != entry.Expression || previous.Compiled == nil) {
		baselines = []int64{id}
	}
	entry.Revision, err = service.store.UpdateStrategy(ctx, entry.Strategy, entry.Compiled.IndicatorIDs(), entry.Compiled.Symbols(), service.administratorID, len(baselines) > 0)
	if err != nil {
		return Entry{}, err
	}
	current[index] = entry
	service.replace(current, baselines)
	return entry, nil
}

// SetEnabled turns evaluation on or off. Enabling announces the current
// matches.
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

// Delete removes the strategy and its matches.
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
	item.Expression = strings.TrimSpace(item.Expression)
	compiled, err := service.compile(item.Expression, service.Variables())
	if err != nil {
		return Entry{}, err
	}
	return Entry{Strategy: item, Compiled: compiled}, nil
}
