// Package markettable builds the market tables clients render: the backend
// decides the columns, their order, how each cell is shown, which columns are
// sortable, and the default sort.
package markettable

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"crypto-scanner/internal/analysis"
	"crypto-scanner/internal/closedindicator"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
)

// Kind tells the client how to render a cell.
type Kind string

const (
	KindText          Kind = "text"
	KindUSDCompact    Kind = "usd_compact"
	KindRangePercent  Kind = "range_percent"
	KindOscillator    Kind = "oscillator"
	KindPercentChange Kind = "percent_change"
	KindSparkline     Kind = "sparkline"
	KindLink          Kind = "link"
	KindFavorite      Kind = "favorite"
	KindCount         Kind = "count"
	// KindNumber is a plain number, such as an admin-configured indicator.
	KindNumber Kind = "number"
)

// data is the cell field a kind or source uses.
type data int

const (
	dataNone data = iota
	dataValue
	dataSeries
	dataURL
)

func (kind Kind) data() (data, bool) {
	switch kind {
	case KindText, KindFavorite:
		return dataNone, true
	case KindUSDCompact, KindRangePercent, KindOscillator, KindPercentChange, KindCount, KindNumber:
		return dataValue, true
	case KindSparkline:
		return dataSeries, true
	case KindLink:
		return dataURL, true
	default:
		return 0, false
	}
}

// Direction orders a sorted column.
type Direction string

const (
	Ascending  Direction = "asc"
	Descending Direction = "desc"
)

type Sort struct {
	Column    string
	Direction Direction
}

// Column is one table column. Source decides the cell data and must match the
// data Kind renders.
type Column struct {
	ID       string
	Title    string
	Kind     Kind
	Sortable bool
	Source   Source
}

// Row carries everything a column source can read for one instrument.
type Row struct {
	Symbol           string
	Evaluations      []analysis.Evaluation
	PriceHistory     []*float64
	ClosedIndicators []closedindicator.Value
	AlertCount       int
}

// RowFromSearchItem converts an analyzed instrument into a table row.
func RowFromSearchItem(item analysis.SearchItem) Row {
	return Row{Symbol: item.Symbol, Evaluations: item.Evaluations, PriceHistory: item.PriceHistory, ClosedIndicators: item.ClosedIndicators}
}

// Cell holds the field its column kind uses; unavailable values are nil.
type Cell struct {
	Value  *float64
	Series []*float64
	URL    *string
}

type TableRow struct {
	Symbol string
	Cells  map[string]Cell
}

type Table struct {
	Columns     []Column
	DefaultSort Sort
	Rows        []TableRow
}

// ColumnSource supplies the columns that fill a ConfiguredIndicators slot.
// They must already be valid: numeric closed indicator columns with
// normalized selections and ids that no static column uses.
type ColumnSource interface {
	TableColumns() []Column
}

// Catalog is a validated table definition.
type Catalog struct {
	columns     []Column
	defaultSort Sort
	configured  ColumnSource
}

// NewCatalog validates the columns against their kinds and the indicator
// registry. Closed indicator selections are normalized, so they match the
// targets the background tracker calculates. A ConfiguredIndicators column is
// a slot that Build replaces with the current columns of configured.
func NewCatalog(registry *indicator.Registry, configured ColumnSource, defaultSort Sort, columns ...Column) (Catalog, error) {
	if registry == nil || configured == nil || len(columns) == 0 {
		return Catalog{}, errors.New("table catalog needs an indicator registry, a configured column source, and columns")
	}
	normalized := make([]Column, len(columns))
	ids := map[string]Column{}
	slots := 0
	for i, column := range columns {
		if _, ok := column.Source.(ConfiguredIndicators); ok {
			if slots++; slots > 1 {
				return Catalog{}, errors.New("table catalog has more than one configured indicator slot")
			}
			normalized[i] = column
			continue
		}
		if strings.TrimSpace(column.ID) == "" || strings.TrimSpace(column.Title) == "" || column.Source == nil {
			return Catalog{}, fmt.Errorf("table column %d needs an id, a title, and a source", i)
		}
		if _, exists := ids[column.ID]; exists {
			return Catalog{}, fmt.Errorf("table column %q is duplicated", column.ID)
		}
		kindData, ok := column.Kind.data()
		if !ok {
			return Catalog{}, fmt.Errorf("table column %q has unknown kind %q", column.ID, column.Kind)
		}
		if kindData != column.Source.data() {
			return Catalog{}, fmt.Errorf("table column %q: source does not provide the data of kind %q", column.ID, column.Kind)
		}
		if column.Sortable && kindData != dataValue {
			return Catalog{}, fmt.Errorf("table column %q: only numeric columns are sortable", column.ID)
		}
		if source, ok := column.Source.(ClosedIndicator); ok {
			normalizedSource, err := source.normalize(registry)
			if err != nil {
				return Catalog{}, fmt.Errorf("table column %q: %w", column.ID, err)
			}
			column.Source = normalizedSource
		}
		normalized[i] = column
		ids[column.ID] = column
	}
	if sorted, ok := ids[defaultSort.Column]; !ok || !sorted.Sortable ||
		(defaultSort.Direction != Ascending && defaultSort.Direction != Descending) {
		return Catalog{}, fmt.Errorf("default sort %q must name a sortable column with a valid direction", defaultSort.Column)
	}
	return Catalog{columns: normalized, defaultSort: defaultSort, configured: configured}, nil
}

// ClosedTargets returns the closed indicator targets the current columns
// read, including the configured indicator columns.
func (catalog Catalog) ClosedTargets() []closedindicator.Target {
	var targets []closedindicator.Target
	for _, column := range catalog.expand() {
		if source, ok := column.Source.(ClosedIndicator); ok {
			if !slices.ContainsFunc(targets, source.Target.Equal) {
				targets = append(targets, source.Target)
			}
		}
	}
	return targets
}

// Build fills every column for each row, in row order.
func (catalog Catalog) Build(rows []Row) Table {
	columns := catalog.expand()
	result := Table{Columns: columns, DefaultSort: catalog.defaultSort, Rows: make([]TableRow, len(rows))}
	for i, row := range rows {
		cells := make(map[string]Cell, len(columns))
		for _, column := range columns {
			if cell, ok := column.Source.cell(row); ok {
				cells[column.ID] = cell
			}
		}
		result.Rows[i] = TableRow{Symbol: row.Symbol, Cells: cells}
	}
	return result
}

// expand replaces the configured indicator slot with the current columns.
func (catalog Catalog) expand() []Column {
	columns := make([]Column, 0, len(catalog.columns))
	for _, column := range catalog.columns {
		if _, ok := column.Source.(ConfiguredIndicators); ok {
			columns = append(columns, catalog.configured.TableColumns()...)
			continue
		}
		columns = append(columns, column)
	}
	return columns
}

// Source reads one cell from a row. The set of sources is closed.
type Source interface {
	data() data
	// cell reports false when the column has no cell data (text, favorite).
	cell(Row) (Cell, bool)
}

// Symbol is the instrument symbol, which the row already carries.
type Symbol struct{}

func (Symbol) data() data            { return dataNone }
func (Symbol) cell(Row) (Cell, bool) { return Cell{}, false }

// ConfiguredIndicators marks where the admin-configured indicator columns go.
// The slot column's own id, title, and kind are ignored.
type ConfiguredIndicators struct{}

func (ConfiguredIndicators) data() data            { return dataNone }
func (ConfiguredIndicators) cell(Row) (Cell, bool) { return Cell{}, false }

// Favorite is the user's favorite toggle for the row's symbol.
type Favorite struct{}

func (Favorite) data() data            { return dataNone }
func (Favorite) cell(Row) (Cell, bool) { return Cell{}, false }

// CriterionMetric is a metric of the first evaluation of the named criterion
// on Unit (empty for criteria without candles). It does not depend on the
// instance keys clients choose for their criteria.
type CriterionMetric struct {
	Criterion string
	Unit      analysis.Unit
	Metric    string
}

func (CriterionMetric) data() data { return dataValue }
func (source CriterionMetric) cell(row Row) (Cell, bool) {
	for _, evaluation := range row.Evaluations {
		if evaluation.Name == source.Criterion && evaluation.Unit == source.Unit {
			if value, ok := evaluation.Metrics[source.Metric]; ok {
				return Cell{Value: &value}, true
			}
		}
	}
	return Cell{}, true
}

// ClosedIndicator is an output of a background indicator at the latest closed
// candle.
type ClosedIndicator struct {
	Target closedindicator.Target
	Output string
}

func (source ClosedIndicator) normalize(registry *indicator.Registry) (ClosedIndicator, error) {
	if !source.Target.Interval.Valid() {
		return ClosedIndicator{}, fmt.Errorf("invalid interval %q", source.Target.Interval)
	}
	selection, err := registry.Normalize(source.Target.Selection)
	if err != nil {
		return ClosedIndicator{}, err
	}
	outputs, err := registry.Outputs(selection.Type)
	if err != nil {
		return ClosedIndicator{}, err
	}
	if !slices.Contains(outputs, source.Output) {
		return ClosedIndicator{}, fmt.Errorf("output %q is not produced by %q", source.Output, selection.Type)
	}
	source.Target.Selection = selection
	return source, nil
}

func (ClosedIndicator) data() data { return dataValue }
func (source ClosedIndicator) cell(row Row) (Cell, bool) {
	for _, value := range row.ClosedIndicators {
		if !value.Target.Equal(source.Target) {
			continue
		}
		for _, output := range value.Outputs {
			if output.Name == source.Output {
				result := output.Value
				return Cell{Value: &result}, true
			}
		}
	}
	return Cell{}, true
}

// PriceHistory is the seven-day hourly close series.
type PriceHistory struct{}

func (PriceHistory) data() data { return dataSeries }
func (PriceHistory) cell(row Row) (Cell, bool) {
	if len(row.PriceHistory) == market.SevenDayPriceSlots {
		return Cell{Series: row.PriceHistory}, true
	}
	return Cell{Series: make([]*float64, market.SevenDayPriceSlots)}, true
}

// PriceChangePercent is the change between the first and the last available
// seven-day hourly close.
type PriceChangePercent struct{}

func (PriceChangePercent) data() data { return dataValue }
func (PriceChangePercent) cell(row Row) (Cell, bool) {
	var first, last *float64
	for _, price := range row.PriceHistory {
		if price == nil {
			continue
		}
		if first == nil {
			first = price
		}
		last = price
	}
	if first == nil || *first == 0 {
		return Cell{}, true
	}
	change := (*last - *first) / *first * 100
	return Cell{Value: &change}, true
}

// ExchangeLink opens the instrument on Binance Spot.
type ExchangeLink struct{}

const binanceSpotQuoteAsset = "USDT"

func (ExchangeLink) data() data { return dataURL }
func (ExchangeLink) cell(row Row) (Cell, bool) {
	symbol := market.NormalizeSymbol(row.Symbol)
	base, ok := strings.CutSuffix(symbol, binanceSpotQuoteAsset)
	if !ok || base == "" {
		return Cell{}, true
	}
	url := "https://www.binance.com/en/trade/" + base + "_" + binanceSpotQuoteAsset + "?type=spot"
	return Cell{URL: &url}, true
}

// AlertCount is the number of the user's price alerts for the instrument.
type AlertCount struct{}

func (AlertCount) data() data { return dataValue }
func (AlertCount) cell(row Row) (Cell, bool) {
	count := float64(row.AlertCount)
	return Cell{Value: &count}, true
}
