// Package strategy holds the administrator's strategies, CEL expressions over
// the configured indicators, and alerts when a monitored instrument starts
// matching one of them.
package strategy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"

	"crypto-scanner/internal/scannerindicator"
)

const maxNameLength = 64

var (
	ErrInvalidArgument = errors.New("invalid strategy")
	ErrNotFound        = errors.New("strategy not found")
	// ErrConflict means another strategy already has the name.
	ErrConflict = errors.New("strategy name already exists")
)

// Strategy is one stored strategy.
type Strategy struct {
	ID         int64
	Name       string
	Expression string
	Enabled    bool
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

type Store interface {
	ListStrategies(context.Context) ([]Strategy, error)
	// CreateStrategy stores the strategy, at revision 1, with the indicators
	// it reads and returns its id. It fails with ErrConflict for a taken name and with
	// ErrInvalidArgument when an indicator no longer exists.
	CreateStrategy(context.Context, Strategy, []int64) (int64, error)
	// UpdateStrategy replaces the name, expression, and indicators and
	// returns the revision; baseline marks the matches for announcement. It
	// fails like CreateStrategy, or with ErrNotFound.
	UpdateStrategy(ctx context.Context, item Strategy, indicatorIDs []int64, baseline bool) (int64, error)
	// SetStrategyEnabled returns the revision and fails with ErrNotFound.
	// Enabling marks the matches for announcement; disabling forgets them.
	SetStrategyEnabled(context.Context, int64, bool) (int64, error)
	DeleteStrategy(context.Context, int64) error
}

// Indicators supplies the configured indicators expressions may read.
type Indicators interface {
	List() []scannerindicator.Entry
}

// Service keeps the strategies in memory and persists every change.
type Service struct {
	store      Store
	indicators Indicators
	logger     *slog.Logger
	changed    func(baselines []int64)

	// writes serializes changes, so name checks see every earlier change.
	writes  sync.Mutex
	mu      sync.RWMutex
	entries []Entry
}

// NewService creates an empty service; Load reads the stored strategies.
// changed receives the ids of strategies whose current matches must be
// announced after each change and must not block.
func NewService(store Store, indicators Indicators, logger *slog.Logger, changed func(baselines []int64)) (*Service, error) {
	if store == nil || indicators == nil || logger == nil || changed == nil {
		return nil, errors.New("strategy store, indicators, logger, and change listener are required")
	}
	return &Service{store: store, indicators: indicators, logger: logger.With("module", "strategy"), changed: changed}, nil
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
		compiled, err := Compile(item.Expression, variables)
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

// Create validates and stores a strategy.
func (service *Service) Create(ctx context.Context, item Strategy) (Entry, error) {
	entry, err := service.entry(item)
	if err != nil {
		return Entry{}, err
	}
	service.writes.Lock()
	defer service.writes.Unlock()
	entry.ID, err = service.store.CreateStrategy(ctx, entry.Strategy, entry.Compiled.IndicatorIDs())
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

// Update changes the name and expression. A changed expression of an enabled
// strategy announces its current matches again.
func (service *Service) Update(ctx context.Context, id int64, name, expression string) (Entry, error) {
	service.writes.Lock()
	defer service.writes.Unlock()
	current := service.List()
	index := slices.IndexFunc(current, func(entry Entry) bool { return entry.ID == id })
	if index < 0 {
		return Entry{}, ErrNotFound
	}
	previous := current[index]
	entry, err := service.entry(Strategy{ID: id, Name: name, Expression: expression, Enabled: previous.Enabled})
	if err != nil {
		return Entry{}, err
	}
	var baselines []int64
	if entry.Enabled && (previous.Expression != entry.Expression || previous.Compiled == nil) {
		baselines = []int64{id}
	}
	entry.Revision, err = service.store.UpdateStrategy(ctx, entry.Strategy, entry.Compiled.IndicatorIDs(), len(baselines) > 0)
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
	item.Expression = strings.TrimSpace(item.Expression)
	compiled, err := Compile(item.Expression, service.Variables())
	if err != nil {
		return Entry{}, err
	}
	return Entry{Strategy: item, Compiled: compiled}, nil
}
