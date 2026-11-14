// Package scannerindicator holds the global indicator configuration the
// administrator manages: each period of a configured indicator optionally
// draws it on the charts of the interval and shows it as a market table
// column, and the strategy builder offers it. Strategies call any indicator,
// configured or not.
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
	// ErrConflict means the same selection, or one with the same title, is
	// configured.
	ErrConflict = errors.New("scanner indicator already exists")
	// ErrLimit means the charts of an interval already draw the maximum
	// number of indicators.
	ErrLimit = errors.New("scanner indicator limit reached")
)

// Scale is the optional value axis of an indicator pane. Overlays share the
// candle price scale and have none.
type Scale struct {
	Min    *float64
	Max    *float64
	Levels []float64
}

// Period is how the charts and tables of one interval show an indicator.
type Period struct {
	Interval    market.CandleInterval
	ShowInTable bool
	ShowInChart bool
}

// Indicator is one configured indicator. Its periods are display settings
// only.
type Indicator struct {
	ID        int64
	Selection indicator.Selection
	// Periods are in interval order, each interval at most once.
	Periods []Period
	Scale   Scale
}

// Target is the closed-candle calculation of the indicator on interval.
func (item Indicator) Target(interval market.CandleInterval) closedindicator.Target {
	return closedindicator.Target{Interval: interval, Selection: item.Selection}
}

// HasSelection reports whether the indicator calculates selection, which
// must be normalized.
func (item Indicator) HasSelection(selection indicator.Selection) bool {
	return item.Target("").Equal(closedindicator.Target{Selection: selection})
}

// Period returns the display settings of interval; found is false when the
// indicator has no such period.
func (item Indicator) Period(interval market.CandleInterval) (period Period, found bool) {
	index := slices.IndexFunc(item.Periods, func(period Period) bool { return period.Interval == interval })
	if index < 0 {
		return Period{}, false
	}
	return item.Periods[index], true
}

// Entry is a validated indicator with its derived presentation.
type Entry struct {
	Indicator
	// Title names the indicator without a period, such as "rsi-21";
	// ColumnTitle adds the interval.
	Title     string
	Placement chart.Placement
	Outputs   []string
	// Counts are the outputs that count candles.
	Counts []string
	// lines are the outputs charts draw, in output order: those not
	// indicator.OutputHidden, with how they are drawn. Titles and colors are
	// added when a chart draws them.
	lines []chart.IndicatorLine
	// lineTitle names the chart lines, such as "RSI 14".
	lineTitle string
	// pane is the key of the chart pane shared with other indicators, empty
	// for an indicator drawn in a pane of its own.
	pane    string
	palette []string
}

type Store interface {
	ListScannerIndicators(context.Context) ([]Indicator, error)
	// CreateScannerIndicator appends the indicator with its periods to the
	// display order and returns its id. It fails with ErrConflict for a
	// duplicate selection.
	CreateScannerIndicator(context.Context, Indicator) (int64, error)
	// UpdateScannerIndicator replaces the periods and the scale.
	// UpdateScannerIndicator and DeleteScannerIndicator fail with
	// ErrNotFound for an unknown id.
	UpdateScannerIndicator(context.Context, Indicator) error
	DeleteScannerIndicator(context.Context, int64) error
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

	// writes serializes changes, so limit and duplicate checks see every
	// earlier change.
	writes  sync.Mutex
	mu      sync.RWMutex
	entries []Entry
}

// New creates an empty service; Load reads the stored configuration. Chart
// lines take palette colors (theme tokens) in turn. changed is called after
// each change and must not block.
func New(store Store, registry *indicator.Registry, palette []string, logger *slog.Logger, changed func()) (*Service, error) {
	if store == nil || registry == nil || len(palette) == 0 || logger == nil || changed == nil {
		return nil, errors.New("scanner indicator store, registry, palette, logger, and change listener are required")
	}
	return &Service{store: store, registry: registry, palette: slices.Clone(palette), logger: logger.With("module", "scanner_indicator"), changed: changed}, nil
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

// Preview validates a selection that is not configured and derives its
// presentation, such as its title.
func (service *Service) Preview(selection indicator.Selection) (Entry, error) {
	return service.entry(Indicator{Selection: selection})
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
	for _, existing := range current {
		if existing.HasSelection(entry.Selection) {
			return Entry{}, fmt.Errorf("%w: %s is already configured", ErrConflict, entry.lineTitle)
		}
		// Titles name table columns.
		if existing.Title == entry.Title {
			return Entry{}, fmt.Errorf("%w: another indicator is titled %s", ErrConflict, entry.Title)
		}
	}
	if err := chartLimit(current, entry); err != nil {
		return Entry{}, err
	}
	id, err := service.store.CreateScannerIndicator(ctx, entry.Indicator)
	if err != nil {
		return Entry{}, err
	}
	entry.ID = id
	service.replace(append(current, entry))
	return entry, nil
}

// Update replaces the periods of the indicator and its pane scale.
func (service *Service) Update(ctx context.Context, id int64, periods []Period, scale Scale) (Entry, error) {
	service.writes.Lock()
	defer service.writes.Unlock()
	current := service.List()
	index := slices.IndexFunc(current, func(entry Entry) bool { return entry.ID == id })
	if index < 0 {
		return Entry{}, ErrNotFound
	}
	item := current[index].Indicator
	item.Periods, item.Scale = periods, scale
	entry, err := service.entry(item)
	if err != nil {
		return Entry{}, err
	}
	if err := chartLimit(current, entry); err != nil {
		return Entry{}, err
	}
	if err := service.store.UpdateScannerIndicator(ctx, entry.Indicator); err != nil {
		return Entry{}, err
	}
	current[index] = entry
	service.replace(current)
	return entry, nil
}

// chartLimit checks that the charts of every period that draws entry draw
// at most chart.MaxIndicators indicators with it, among the current ones
// other than entry itself.
func chartLimit(current []Entry, entry Entry) error {
	for _, period := range entry.Periods {
		if !period.ShowInChart {
			continue
		}
		count := 0
		for _, other := range current {
			if found, ok := other.Period(period.Interval); ok && found.ShowInChart && other.ID != entry.ID {
				count++
			}
		}
		if count >= chart.MaxIndicators {
			return fmt.Errorf("%w: %s charts draw at most %d indicators", ErrLimit, period.Interval, chart.MaxIndicators)
		}
	}
	return nil
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

// TableColumns are the market table columns of the periods shown in tables,
// in display order and, within an indicator, in interval order.
func (service *Service) TableColumns() []markettable.Column {
	service.mu.RLock()
	defer service.mu.RUnlock()
	var columns []markettable.Column
	for _, entry := range service.entries {
		for _, period := range entry.Periods {
			if !period.ShowInTable {
				continue
			}
			columns = append(columns, markettable.Column{
				ID:       fmt.Sprintf("indicator_%d_%s", entry.ID, period.Interval),
				Title:    entry.ColumnTitle(period.Interval),
				Kind:     markettable.KindNumber,
				Sortable: true,
				Source:   markettable.ClosedIndicator{Target: entry.Target(period.Interval), Output: entry.Outputs[0]},
			})
		}
	}
	return columns
}

// TableTargets returns the distinct closed indicator targets the table
// columns read.
func (service *Service) TableTargets() []closedindicator.Target {
	service.mu.RLock()
	defer service.mu.RUnlock()
	var targets []closedindicator.Target
	for _, entry := range service.entries {
		for _, period := range entry.Periods {
			if target := entry.Target(period.Interval); period.ShowInTable && !slices.ContainsFunc(targets, target.Equal) {
				targets = append(targets, target)
			}
		}
	}
	return targets
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
		if period, found := entry.Period(interval); !found || !period.ShowInChart {
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

// entry validates an indicator and derives its presentation. Its periods are
// put in interval order.
func (service *Service) entry(item Indicator) (Entry, error) {
	periods, err := sortedPeriods(item.Periods)
	if err != nil {
		return Entry{}, err
	}
	item.Periods = periods
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
	entry := Entry{Indicator: item, Placement: chart.PlacementPane, Outputs: make([]string, len(descriptor.Outputs)), palette: service.palette}
	for i, output := range descriptor.Outputs {
		entry.Outputs[i] = output.Name
		if output.Count {
			entry.Counts = append(entry.Counts, output.Name)
		}
		if output.Style != indicator.OutputHidden {
			line, err := drawnLine(output, selection.Parameters)
			if err != nil {
				return Entry{}, fmt.Errorf("%w: %w", ErrInvalidArgument, err)
			}
			entry.lines = append(entry.lines, line)
		}
	}
	// An overlay fits the price scale only over prices and the volume scale
	// only over volumes; quote volumes and trade counts get a pane.
	if descriptor.Overlay {
		switch {
		case !slices.ContainsFunc(fields, func(field string) bool { return !indicator.PriceField(field) }):
			entry.Placement = chart.PlacementOverlay
		case !slices.ContainsFunc(fields, func(field string) bool { return field != "volume" }):
			entry.Placement = chart.PlacementVolume
		}
	}
	if entry.Placement == chart.PlacementPane && !slices.ContainsFunc(descriptor.Outputs, histogram) {
		entry.pane = sharedPane(selection.Type, fields)
	}
	if slices.ContainsFunc(item.Periods, func(period Period) bool { return period.ShowInTable }) && len(entry.Outputs) != 1 {
		return Entry{}, fmt.Errorf("%w: only indicators with one output can be table columns", ErrInvalidArgument)
	}
	if err := validateScale(entry.Placement, item.Scale); err != nil {
		return Entry{}, err
	}
	entry.Title = title(descriptor, selection.Parameters, defaults.Parameters)
	entry.lineTitle = lineTitle(descriptor, selection.Parameters, defaults.Parameters)
	if err := chart.ValidateIndicator(service.registry, service.catalogIndicator(entry, 0)); err != nil {
		return Entry{}, fmt.Errorf("%w: %w", ErrInvalidArgument, err)
	}
	return entry, nil
}

// sortedPeriods checks the periods and puts them in interval order.
func sortedPeriods(periods []Period) ([]Period, error) {
	intervals := market.CandleIntervals()
	for i, period := range periods {
		if !period.Interval.Valid() {
			return nil, fmt.Errorf("%w: interval %q is unsupported", ErrInvalidArgument, period.Interval)
		}
		if slices.ContainsFunc(periods[:i], func(earlier Period) bool { return earlier.Interval == period.Interval }) {
			return nil, fmt.Errorf("%w: choose each period once", ErrInvalidArgument)
		}
	}
	sorted := slices.Clone(periods)
	slices.SortFunc(sorted, func(a, b Period) int {
		return slices.Index(intervals, a.Interval) - slices.Index(intervals, b.Interval)
	})
	return sorted, nil
}

// ColumnTitle is the title of the indicator's table column on interval, such
// as "d-rsi-21".
func (entry Entry) ColumnTitle(interval market.CandleInterval) string {
	return IntervalPrefix(interval) + "-" + entry.Title
}

// drawnLine is how charts draw output: as markers when the indicator says
// so, shifted by the parameter it names, and as a line otherwise.
func drawnLine(output indicator.OutputDescriptor, parameters indicator.Parameters) (chart.IndicatorLine, error) {
	line := chart.IndicatorLine{Output: output.Name, Style: chart.LineStyleLine}
	switch output.Style {
	case indicator.OutputMarkerAbove:
		line.Style = chart.LineStyleMarkerAbove
	case indicator.OutputMarkerBelow:
		line.Style = chart.LineStyleMarkerBelow
	}
	if output.Shift != "" {
		shift, err := indicator.ParseNumber(parameters[output.Shift])
		if err != nil {
			return chart.IndicatorLine{}, fmt.Errorf("%s %w", output.Shift, err)
		}
		line.Shift = int(shift)
	}
	return line, nil
}

func histogram(output indicator.OutputDescriptor) bool {
	return output.Style == indicator.OutputHistogram
}

// sharedPane keys the pane that indicators of one type share when they read
// fields of the same units, such as every RSI or every ATR, whatever their
// parameters. Indicators with a histogram, such as MACD, do not read well next
// to each other and get panes of their own.
func sharedPane(indicatorType indicator.Type, fields []string) string {
	units := make([]string, len(fields))
	for i, field := range fields {
		units[i] = field
		if indicator.PriceField(field) {
			units[i] = "price"
		}
	}
	slices.Sort(units)
	return string(indicatorType) + ":" + strings.Join(slices.Compact(units), ",")
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

// catalogIndicator draws the entry's lines, coloring them from palette
// position firstLine on.
func (service *Service) catalogIndicator(entry Entry, firstLine int) chart.CatalogIndicator {
	return entry.ChartIndicator(fmt.Sprintf("indicator-%d", entry.ID), firstLine)
}

// ChartIndicator derives chart presentation without consulting display flags.
// The caller supplies a unique id, including for indicators that are not stored.
func (entry Entry) ChartIndicator(id string, firstLine int) chart.CatalogIndicator {
	item := chart.CatalogIndicator{
		ID:        id,
		Selection: entry.Selection,
		Placement: entry.Placement,
		Lines:     make([]chart.IndicatorLine, len(entry.lines)),
	}
	for i, line := range entry.lines {
		line.Title = entry.lineTitle
		if len(entry.Outputs) > 1 {
			line.Title += " " + line.Output
		}
		line.Color = entry.palette[(firstLine+i)%len(entry.palette)]
		item.Lines[i] = line
	}
	if entry.Placement == chart.PlacementPane {
		scale := &chart.IndicatorScale{Min: entry.Scale.Min, Max: entry.Scale.Max}
		for _, level := range entry.Scale.Levels {
			scale.Levels = append(scale.Levels, chart.IndicatorLevel{Value: level, Title: formatValue(level)})
		}
		item.Scale = scale
		item.Pane = entry.pane
		if item.Pane == "" {
			item.Pane = item.ID
		}
	}
	return item
}

// title is the type followed by the parameter values, such as "rsi",
// "rsi-21", or "sma-20-volume". Values that are not named choices appear in
// order up to the last one that differs from its default, so each keeps its
// position and distinct selections get distinct titles: "bbands-20-2.5"
// raises the upper band, "bbands-20-2-2.5" the lower one. Once one of several
// named choices differs, all of them appear in order, so "beta-high-close"
// and "beta-close-high" differ.
func title(descriptor indicator.Descriptor, parameters, defaults indicator.Parameters) string {
	parts := []string{string(descriptor.Type)}
	named := namedChoicesChanged(descriptor, parameters, defaults)
	last := -1
	for i, parameter := range descriptor.Parameters {
		if !namedChoice(parameter) && parameterValue(parameter, parameters[parameter.Key]) != parameterValue(parameter, defaults[parameter.Key]) {
			last = i
		}
	}
	for i, parameter := range descriptor.Parameters {
		if namedChoice(parameter) && named || !namedChoice(parameter) && i <= last {
			parts = append(parts, parameterValue(parameter, parameters[parameter.Key]))
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
// Plain choices, such as moving average types, stay numbers, which keeps
// titles stable.
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
