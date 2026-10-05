// Package scannerindicator holds the global indicator configuration the
// administrator manages: every configured indicator can be read by strategies
// and is optionally drawn on the charts of its interval and shown as a market
// table column.
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
	// ErrInUse means a strategy reads the indicator.
	ErrInUse = errors.New("scanner indicator is used by a strategy")
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
	ShowInChart bool
	Scale       Scale
}

// Target is the closed-candle calculation of the indicator.
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
	// CreateScannerIndicators appends the indicators to the display order in
	// one transaction and returns their ids in order. It fails with
	// ErrConflict for a duplicate selection.
	CreateScannerIndicators(context.Context, []Indicator) ([]int64, error)
	// UpdateScannerIndicator and DeleteScannerIndicator fail with
	// ErrNotFound for an unknown id. Deletions fail with ErrInUse while a
	// strategy reads an indicator.
	UpdateScannerIndicator(context.Context, Indicator) error
	DeleteScannerIndicator(context.Context, int64) error
	// DeleteUnusedScannerIndicators removes every indicator no strategy
	// references and returns the removed ids.
	DeleteUnusedScannerIndicators(context.Context) ([]int64, error)
	// ReorderScannerIndicators stores ids as the display order.
	ReorderScannerIndicators(context.Context, []int64) error
}

// Service keeps the configuration in memory and persists every change.
type Service struct {
	store    Store
	registry *indicator.Registry
	palette  []string
	logger   *slog.Logger
	changed  func()
	usage    func() map[int64][]string

	// writes serializes changes, so limit and duplicate checks see every
	// earlier change.
	writes  sync.Mutex
	mu      sync.RWMutex
	entries []Entry
}

// New creates an empty service; Load reads the stored configuration. Chart
// lines take palette colors (theme tokens) in turn. changed is called after
// each change and must not block. usage maps indicator ids to the names of
// the strategies that read them.
func New(store Store, registry *indicator.Registry, palette []string, logger *slog.Logger, changed func(), usage func() map[int64][]string) (*Service, error) {
	if store == nil || registry == nil || len(palette) == 0 || logger == nil || changed == nil || usage == nil {
		return nil, errors.New("scanner indicator store, registry, palette, logger, change listener, and usage are required")
	}
	return &Service{store: store, registry: registry, palette: slices.Clone(palette), logger: logger.With("module", "scanner_indicator"), changed: changed, usage: usage}, nil
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

// Usage maps indicator ids to the names of the strategies that read them.
func (service *Service) Usage() map[int64][]string { return service.usage() }

// Create validates, stores, and applies new indicators, such as one
// selection on several intervals. Either all of them are added or none.
func (service *Service) Create(ctx context.Context, items []Indicator) ([]Entry, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("%w: choose at least one interval", ErrInvalidArgument)
	}
	entries := make([]Entry, len(items))
	for i, item := range items {
		entry, err := service.entry(item)
		if err != nil {
			return nil, err
		}
		if slices.ContainsFunc(entries[:i], func(earlier Entry) bool { return earlier.Target().Equal(entry.Target()) }) {
			return nil, fmt.Errorf("%w: choose each interval once", ErrInvalidArgument)
		}
		entries[i] = entry
	}
	service.writes.Lock()
	defer service.writes.Unlock()
	current := service.List()
	for _, entry := range entries {
		count := 0
		for _, existing := range current {
			if existing.Interval != entry.Interval {
				continue
			}
			if existing.Target().Equal(entry.Target()) {
				return nil, fmt.Errorf("%w: %s already has %s", ErrConflict, entry.Interval, entry.lineTitle)
			}
			// Titles name table columns and strategy variables.
			if existing.Title == entry.Title {
				return nil, fmt.Errorf("%w: another indicator is titled %s", ErrConflict, entry.Title)
			}
			count++
		}
		if count >= chart.MaxIndicators {
			return nil, fmt.Errorf("%w: %s charts draw at most %d indicators", ErrLimit, entry.Interval, chart.MaxIndicators)
		}
	}
	indicators := make([]Indicator, len(entries))
	for i, entry := range entries {
		indicators[i] = entry.Indicator
	}
	ids, err := service.store.CreateScannerIndicators(ctx, indicators)
	if err != nil {
		return nil, err
	}
	for i := range entries {
		entries[i].ID = ids[i]
	}
	service.replace(append(current, entries...))
	return entries, nil
}

// Update changes whether the indicator is a table column, whether charts draw
// it, and its pane scale.
func (service *Service) Update(ctx context.Context, id int64, showInTable, showInChart bool, scale Scale) (Entry, error) {
	service.writes.Lock()
	defer service.writes.Unlock()
	current := service.List()
	index := slices.IndexFunc(current, func(entry Entry) bool { return entry.ID == id })
	if index < 0 {
		return Entry{}, ErrNotFound
	}
	item := current[index].Indicator
	item.ShowInTable, item.ShowInChart, item.Scale = showInTable, showInChart, scale
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

// Delete removes the indicator.
func (service *Service) Delete(ctx context.Context, id int64) error {
	service.writes.Lock()
	defer service.writes.Unlock()
	current := service.List()
	index := slices.IndexFunc(current, func(entry Entry) bool { return entry.ID == id })
	if index < 0 {
		return ErrNotFound
	}
	if names := service.usage()[id]; len(names) > 0 {
		return fmt.Errorf("%w: %s", ErrInUse, strings.Join(names, ", "))
	}
	if err := service.store.DeleteScannerIndicator(ctx, id); err != nil {
		return err
	}
	service.replace(slices.Delete(current, index, index+1))
	return nil
}

// DeleteUnused removes every indicator no strategy references.
func (service *Service) DeleteUnused(ctx context.Context) error {
	service.writes.Lock()
	defer service.writes.Unlock()
	deleted, err := service.store.DeleteUnusedScannerIndicators(ctx)
	if err != nil {
		return err
	}
	service.replace(slices.DeleteFunc(service.List(), func(entry Entry) bool { return slices.Contains(deleted, entry.ID) }))
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

// ChartCatalog lists the indicators charts of interval draw, those shown in
// charts, in display order. Line colors follow the palette across the whole
// chart.
func (service *Service) ChartCatalog(interval market.CandleInterval) []chart.CatalogIndicator {
	service.mu.RLock()
	defer service.mu.RUnlock()
	var catalog []chart.CatalogIndicator
	line := 0
	for _, entry := range service.entries {
		if entry.Interval != interval || !entry.ShowInChart {
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
	service.changed()
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
	if descriptor.Internal {
		return Entry{}, fmt.Errorf("%w: %s cannot be configured", ErrInvalidArgument, selection.Type)
	}
	fields, err := service.registry.Fields(selection)
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
	// An overlay of volumes or trade counts would not fit the price scale.
	if descriptor.Overlay && !slices.ContainsFunc(fields, func(field string) bool { return !indicator.PriceField(field) }) {
		entry.Placement = chart.PlacementOverlay
	}
	if item.ShowInTable && len(entry.Outputs) != 1 {
		return Entry{}, fmt.Errorf("%w: only indicators with one output can be table columns", ErrInvalidArgument)
	}
	if err := validateScale(entry.Placement, item.Scale); err != nil {
		return Entry{}, err
	}
	entry.Title = tableTitle(item.Interval, descriptor, selection.Parameters, defaults.Parameters)
	entry.lineTitle = lineTitle(descriptor, selection.Parameters, defaults.Parameters)
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
// differ from the defaults, such as "d-rsi", "d-rsi-21", or
// "h-sma-20-volume". Once one of several named choices differs, all of them
// appear in order, so "h-beta-high-close" and "h-beta-close-high" differ.
func tableTitle(interval market.CandleInterval, descriptor indicator.Descriptor, parameters, defaults indicator.Parameters) string {
	parts := []string{IntervalPrefix(interval), string(descriptor.Type)}
	named := namedChoicesChanged(descriptor, parameters, defaults)
	for _, parameter := range descriptor.Parameters {
		value := parameterValue(parameter, parameters[parameter.Key])
		if (namedChoice(parameter) && named) || value != parameterValue(parameter, defaults[parameter.Key]) {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, "-")
}

// lineTitle is the upper-case type followed by every parameter value, such as
// "RSI 14" or "MACD 12 26 9". Named choices, such as the candle field, appear
// only once one of them differs from the default, as in "SMA 20 volume".
func lineTitle(descriptor indicator.Descriptor, parameters, defaults indicator.Parameters) string {
	parts := []string{strings.ToUpper(string(descriptor.Type))}
	named := namedChoicesChanged(descriptor, parameters, defaults)
	for _, parameter := range descriptor.Parameters {
		if !namedChoice(parameter) || named {
			parts = append(parts, parameterValue(parameter, parameters[parameter.Key]))
		}
	}
	return strings.Join(parts, " ")
}

// namedChoicesChanged reports whether a named choice differs from its
// default.
func namedChoicesChanged(descriptor indicator.Descriptor, parameters, defaults indicator.Parameters) bool {
	return slices.ContainsFunc(descriptor.Parameters, func(parameter indicator.ParameterDescriptor) bool {
		return namedChoice(parameter) && parameterValue(parameter, parameters[parameter.Key]) != parameterValue(parameter, defaults[parameter.Key])
	})
}

// parameterValue formats a value, naming named choices such as "volume".
// Plain choices, such as moving average types, stay numbers, which keeps the
// titles strategies read stable.
func parameterValue(parameter indicator.ParameterDescriptor, value any) string {
	if namedChoice(parameter) {
		for _, choice := range parameter.Choices {
			if formatValue(choice.Value) == formatValue(value) {
				return choice.Name
			}
		}
	}
	return formatValue(value)
}

func namedChoice(parameter indicator.ParameterDescriptor) bool {
	return parameter.Kind == indicator.ParameterChoice && len(parameter.Choices) > 0 && parameter.Choices[0].Name != ""
}

func formatValue(value any) string {
	return fmt.Sprintf("%v", value)
}

// IntervalPrefix is the short interval name that starts titles, such as "h".
func IntervalPrefix(interval market.CandleInterval) string {
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
