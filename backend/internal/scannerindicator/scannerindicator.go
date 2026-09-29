// Package scannerindicator holds the global indicator configuration the
// administrator manages: every configured indicator is calculated in the
// background, drawn on the charts of its interval, and optionally shown as a
// market table column.
package scannerindicator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"

	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/closedindicator"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/markettable"
	"crypto-scanner/internal/platform/numeric"
)

var (
	ErrInvalidArgument = errors.New("invalid scanner indicator")
	ErrNotFound        = errors.New("scanner indicator not found")
	// ErrConflict means the interval already has the same selection.
	ErrConflict = errors.New("scanner indicator already exists")
	// ErrLimit means the interval charts already draw the maximum number of
	// indicators.
	ErrLimit = errors.New("scanner indicator limit reached")
)

// Scale is the optional value axis of an indicator pane. Overlays share the
// candle price scale and have none.
type Scale struct {
	Min    *float64
	Max    *float64
	Levels []float64
}

// Indicator is one configured indicator.
type Indicator struct {
	ID          int64
	Interval    market.CandleInterval
	Selection   indicator.Selection
	ShowInTable bool
	Scale       Scale
}

// Target is the background calculation of the indicator.
func (item Indicator) Target() closedindicator.Target {
	return closedindicator.Target{Interval: item.Interval, Selection: item.Selection}
}

// Entry is a validated indicator with its derived presentation.
type Entry struct {
	Indicator
	Title     string
	Placement chart.Placement
	Outputs   []string
	// lineTitle names the chart lines, such as "RSI 14".
	lineTitle string
}

type Store interface {
	ListScannerIndicators(context.Context) ([]Indicator, error)
	// CreateScannerIndicator fails with ErrConflict for a duplicate selection.
	CreateScannerIndicator(context.Context, Indicator) (int64, error)
	// UpdateScannerIndicator and DeleteScannerIndicator fail with
	// ErrNotFound for an unknown id.
	UpdateScannerIndicator(context.Context, Indicator) error
	DeleteScannerIndicator(context.Context, int64) error
	// ReorderScannerIndicators stores ids as the display order.
	ReorderScannerIndicators(context.Context, []int64) error
}

// Service keeps the configuration in memory and persists every change.
type Service struct {
	store          Store
	registry       *indicator.Registry
	palette        []string
	logger         *slog.Logger
	targetsChanged func([]closedindicator.Target)

	// writes serializes changes, so limit and duplicate checks see every
	// earlier change.
	writes  sync.Mutex
	mu      sync.RWMutex
	entries []Entry
}

// New creates an empty service; Load reads the stored configuration. Chart
// lines take palette colors (theme tokens) in turn. targetsChanged receives
// every indicator target after each change and must not block.
func New(store Store, registry *indicator.Registry, palette []string, logger *slog.Logger, targetsChanged func([]closedindicator.Target)) (*Service, error) {
	if store == nil || registry == nil || len(palette) == 0 || logger == nil || targetsChanged == nil {
		return nil, errors.New("scanner indicator store, registry, palette, logger, and change listener are required")
	}
	return &Service{store: store, registry: registry, palette: slices.Clone(palette), logger: logger.With("module", "scanner_indicator"), targetsChanged: targetsChanged}, nil
}

// Load replaces the configuration with the stored one. Indicators the
// registry no longer accepts are skipped, so a TA-Lib upgrade cannot stop the
// process.
func (service *Service) Load(ctx context.Context) error {
	stored, err := service.store.ListScannerIndicators(ctx)
	if err != nil {
		return fmt.Errorf("load scanner indicators: %w", err)
	}
	entries := make([]Entry, 0, len(stored))
	for _, item := range stored {
		entry, err := service.entry(item)
		if err != nil {
			service.logger.WarnContext(ctx, "skip invalid scanner indicator", "indicator_id", item.ID, "error", err)
			continue
		}
		entries = append(entries, entry)
	}
	service.writes.Lock()
	defer service.writes.Unlock()
	service.replace(entries)
	return nil
}

// List returns the configuration in display order.
func (service *Service) List() []Entry {
	service.mu.RLock()
	defer service.mu.RUnlock()
	return slices.Clone(service.entries)
}

// Targets returns the background calculation of every indicator.
func (service *Service) Targets() []closedindicator.Target {
	service.mu.RLock()
	defer service.mu.RUnlock()
	return targets(service.entries)
}

// Create validates, stores, and applies a new indicator.
func (service *Service) Create(ctx context.Context, item Indicator) (Entry, error) {
	entry, err := service.entry(item)
	if err != nil {
		return Entry{}, err
	}
	service.writes.Lock()
	defer service.writes.Unlock()
	current := service.List()
	count := 0
	for _, existing := range current {
		if existing.Interval != entry.Interval {
			continue
		}
		if existing.Target().Equal(entry.Target()) {
			return Entry{}, ErrConflict
		}
		count++
	}
	if count >= chart.MaxIndicators {
		return Entry{}, fmt.Errorf("%w: %s charts draw at most %d indicators", ErrLimit, entry.Interval, chart.MaxIndicators)
	}
	if entry.ID, err = service.store.CreateScannerIndicator(ctx, entry.Indicator); err != nil {
		return Entry{}, err
	}
	service.replace(append(current, entry))
	return entry, nil
}

// Update changes whether the indicator is a table column and its pane scale.
func (service *Service) Update(ctx context.Context, id int64, showInTable bool, scale Scale) (Entry, error) {
	service.writes.Lock()
	defer service.writes.Unlock()
	current := service.List()
	index := slices.IndexFunc(current, func(entry Entry) bool { return entry.ID == id })
	if index < 0 {
		return Entry{}, ErrNotFound
	}
	item := current[index].Indicator
	item.ShowInTable, item.Scale = showInTable, scale
	entry, err := service.entry(item)
	if err != nil {
		return Entry{}, err
	}
	if err := service.store.UpdateScannerIndicator(ctx, entry.Indicator); err != nil {
		return Entry{}, err
	}
	current[index] = entry
	service.replace(current)
	return entry, nil
}

// Delete removes the indicator and stops its background calculation.
func (service *Service) Delete(ctx context.Context, id int64) error {
	service.writes.Lock()
	defer service.writes.Unlock()
	current := service.List()
	index := slices.IndexFunc(current, func(entry Entry) bool { return entry.ID == id })
	if index < 0 {
		return ErrNotFound
	}
	if err := service.store.DeleteScannerIndicator(ctx, id); err != nil {
		return err
	}
	service.replace(slices.Delete(current, index, index+1))
	return nil
}

// Reorder sets the display order, which orders table columns and chart
// indicators. ids must name every configured indicator once.
func (service *Service) Reorder(ctx context.Context, ids []int64) ([]Entry, error) {
	service.writes.Lock()
	defer service.writes.Unlock()
	current := service.List()
	positions := make(map[int64]int, len(current))
	for i, entry := range current {
		positions[entry.ID] = i
	}
	reordered := make([]Entry, 0, len(ids))
	for _, id := range ids {
		position, ok := positions[id]
		if !ok {
			return nil, fmt.Errorf("%w: the order must name every indicator once", ErrInvalidArgument)
		}
		delete(positions, id)
		reordered = append(reordered, current[position])
	}
	if len(positions) > 0 {
		return nil, fmt.Errorf("%w: the order must name every indicator once", ErrInvalidArgument)
	}
	if err := service.store.ReorderScannerIndicators(ctx, ids); err != nil {
		return nil, err
	}
	service.replace(reordered)
	return slices.Clone(reordered), nil
}

// TableColumns are the market table columns of indicators shown in tables.
func (service *Service) TableColumns() []markettable.Column {
	service.mu.RLock()
	defer service.mu.RUnlock()
	var columns []markettable.Column
	for _, entry := range service.entries {
		if !entry.ShowInTable {
			continue
		}
		columns = append(columns, markettable.Column{
			ID:       fmt.Sprintf("indicator_%d", entry.ID),
			Title:    entry.Title,
			Kind:     markettable.KindNumber,
			Sortable: true,
			Source:   markettable.ClosedIndicator{Target: entry.Target(), Output: entry.Outputs[0]},
		})
	}
	return columns
}

// ChartCatalog lists the indicators charts of interval draw, in display
// order. Line colors follow the palette across the whole chart.
func (service *Service) ChartCatalog(interval market.CandleInterval) []chart.CatalogIndicator {
	service.mu.RLock()
	defer service.mu.RUnlock()
	var catalog []chart.CatalogIndicator
	line := 0
	for _, entry := range service.entries {
		if entry.Interval != interval {
			continue
		}
		item := service.catalogIndicator(entry, line)
		line += len(item.Lines)
		catalog = append(catalog, item)
	}
	return catalog
}

// replace must be called with writes held.
func (service *Service) replace(entries []Entry) {
	service.mu.Lock()
	service.entries = entries
	service.mu.Unlock()
	service.targetsChanged(targets(entries))
}

func targets(entries []Entry) []closedindicator.Target {
	result := make([]closedindicator.Target, len(entries))
	for i, entry := range entries {
		result[i] = entry.Target()
	}
	return result
}

// entry validates an indicator and derives its presentation.
func (service *Service) entry(item Indicator) (Entry, error) {
	if !item.Interval.Valid() {
		return Entry{}, fmt.Errorf("%w: interval %q is unsupported", ErrInvalidArgument, item.Interval)
	}
	selection, err := service.registry.Normalize(item.Selection)
	if err != nil {
		return Entry{}, fmt.Errorf("%w: %w", ErrInvalidArgument, err)
	}
	item.Selection = selection
	descriptor, err := service.registry.Describe(selection.Type)
	if err != nil {
		return Entry{}, fmt.Errorf("%w: %w", ErrInvalidArgument, err)
	}
	defaults, err := service.registry.Normalize(indicator.Selection{Type: selection.Type})
	if err != nil {
		return Entry{}, fmt.Errorf("%w: %w", ErrInvalidArgument, err)
	}
	entry := Entry{Indicator: item, Placement: chart.PlacementPane, Outputs: make([]string, len(descriptor.Outputs))}
	for i, output := range descriptor.Outputs {
		entry.Outputs[i] = output.Name
	}
	if descriptor.Overlay {
		entry.Placement = chart.PlacementOverlay
	}
	if item.ShowInTable && len(entry.Outputs) != 1 {
		return Entry{}, fmt.Errorf("%w: only indicators with one output can be table columns", ErrInvalidArgument)
	}
	if err := validateScale(entry.Placement, item.Scale); err != nil {
		return Entry{}, err
	}
	entry.Title = tableTitle(item.Interval, descriptor, selection.Parameters, defaults.Parameters)
	entry.lineTitle = lineTitle(descriptor, selection.Parameters)
	if err := chart.ValidateIndicator(service.registry, service.catalogIndicator(entry, 0)); err != nil {
		return Entry{}, fmt.Errorf("%w: %w", ErrInvalidArgument, err)
	}
	return entry, nil
}

func validateScale(placement chart.Placement, scale Scale) error {
	if placement == chart.PlacementOverlay {
		if scale.Min != nil || scale.Max != nil || len(scale.Levels) > 0 {
			return fmt.Errorf("%w: indicators on the price scale have no scale settings", ErrInvalidArgument)
		}
		return nil
	}
	for _, value := range []*float64{scale.Min, scale.Max} {
		if value != nil && !numeric.Finite(*value) {
			return fmt.Errorf("%w: scale bounds must be finite", ErrInvalidArgument)
		}
	}
	for i, level := range scale.Levels {
		if !numeric.Finite(level) || slices.Contains(scale.Levels[:i], level) {
			return fmt.Errorf("%w: scale levels must be finite and distinct", ErrInvalidArgument)
		}
	}
	return nil
}

// catalogIndicator draws the entry, coloring its lines from palette position
// firstLine on.
func (service *Service) catalogIndicator(entry Entry, firstLine int) chart.CatalogIndicator {
	item := chart.CatalogIndicator{
		ID:        fmt.Sprintf("indicator-%d", entry.ID),
		Selection: entry.Selection,
		Placement: entry.Placement,
		Lines:     make([]chart.IndicatorLine, len(entry.Outputs)),
	}
	for i, output := range entry.Outputs {
		title := entry.lineTitle
		if len(entry.Outputs) > 1 {
			title += " " + output
		}
		item.Lines[i] = chart.IndicatorLine{Output: output, Title: title, Color: service.palette[(firstLine+i)%len(service.palette)]}
	}
	if entry.Placement == chart.PlacementPane {
		scale := &chart.IndicatorScale{Min: entry.Scale.Min, Max: entry.Scale.Max}
		for _, level := range entry.Scale.Levels {
			scale.Levels = append(scale.Levels, chart.IndicatorLevel{Value: level, Title: formatValue(level)})
		}
		item.Scale = scale
	}
	return item
}

// tableTitle is "<interval>-<type>", followed by the parameter values that
// differ from the defaults, such as "d-rsi" or "d-rsi-21".
func tableTitle(interval market.CandleInterval, descriptor indicator.Descriptor, parameters, defaults indicator.Parameters) string {
	parts := []string{intervalPrefix(interval), string(descriptor.Type)}
	for _, parameter := range descriptor.Parameters {
		value := formatValue(parameters[parameter.Key])
		if value != formatValue(defaults[parameter.Key]) {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, "-")
}

// lineTitle is the upper-case type followed by every parameter value, such as
// "RSI 14" or "MACD 12 26 9".
func lineTitle(descriptor indicator.Descriptor, parameters indicator.Parameters) string {
	parts := []string{strings.ToUpper(string(descriptor.Type))}
	for _, parameter := range descriptor.Parameters {
		parts = append(parts, formatValue(parameters[parameter.Key]))
	}
	return strings.Join(parts, " ")
}

func formatValue(value any) string {
	return fmt.Sprintf("%v", value)
}

func intervalPrefix(interval market.CandleInterval) string {
	switch interval {
	case market.IntervalHour:
		return "h"
	case market.IntervalDay:
		return "d"
	case market.IntervalWeek:
		return "w"
	case market.IntervalMonth:
		return "m"
	default:
		return string(interval)
	}
}
