package scannerindicator_test

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"testing"

	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/indicator/pivot"
	"crypto-scanner/internal/indicator/talib"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/scannerindicator"
)

// unusedStore satisfies the Store interface for calls that never persist.
type unusedStore struct{ scannerindicator.Store }

func TestPreviewPlacesIndicatorsByTheirSourceUnits(t *testing.T) {
	registry, err := indicator.NewRegistry(talib.New()...)
	if err != nil {
		t.Fatal(err)
	}
	service, err := scannerindicator.New(unusedStore{}, registry, []string{"yellow.5"}, slog.New(slog.DiscardHandler), func() {})
	if err != nil {
		t.Fatal(err)
	}
	source := func(field string) float64 { return float64(slices.Index(indicator.CandleFields, field)) }
	tests := []struct {
		name          string
		indicatorType indicator.Type
		field         string
		want          chart.Placement
	}{
		{name: "overlay of prices", indicatorType: talib.EMAType, field: "close", want: chart.PlacementOverlay},
		{name: "overlay of volumes", indicatorType: talib.EMAType, field: "volume", want: chart.PlacementVolume},
		{name: "overlay of quote volumes", indicatorType: talib.EMAType, field: "quote_volume", want: chart.PlacementPane},
		{name: "overlay of trade counts", indicatorType: talib.EMAType, field: "trades", want: chart.PlacementPane},
		{name: "oscillator of volumes", indicatorType: talib.RSIType, field: "volume", want: chart.PlacementPane},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entry, err := service.Preview(indicator.Selection{Type: test.indicatorType, Parameters: indicator.Parameters{"source": source(test.field)}})
			if err != nil {
				t.Fatal(err)
			}
			if entry.Placement != test.want {
				t.Fatalf("placement = %q, want %q", entry.Placement, test.want)
			}
		})
	}
}

// An indicator of another indicator is drawn where that source is; markers,
// which are drawn on candles, are left out elsewhere.
func TestPreviewPlacesIndicatorsOfIndicatorsWithTheirSource(t *testing.T) {
	registry, err := indicator.NewRegistry(append(talib.New(), pivot.New())...)
	if err != nil {
		t.Fatal(err)
	}
	service, err := scannerindicator.New(unusedStore{}, registry, []string{"yellow.5"}, slog.New(slog.DiscardHandler), func() {})
	if err != nil {
		t.Fatal(err)
	}
	preview := func(selection indicator.Selection) chart.CatalogIndicator {
		t.Helper()
		entry, err := service.Preview(selection)
		if err != nil {
			t.Fatal(err)
		}
		return entry.ChartIndicator("id", "source", 0)
	}
	of := func(indicatorType indicator.Type, source indicator.Type) indicator.Selection {
		nested := indicator.Source{Selection: indicator.Selection{Type: source}, Output: string(source)}
		return indicator.Selection{Type: indicatorType, Parameters: indicator.Parameters{"source": nested.Value()}}
	}
	rsi := preview(indicator.Selection{Type: talib.RSIType})
	if item := preview(of("sma", talib.RSIType)); item.Placement != chart.PlacementPane || item.Pane != rsi.Pane || item.Lines[0].Title != "SMA 30 RSI 14" {
		t.Errorf("sma of rsi = %+v, want a line titled SMA 30 RSI 14 in the pane %q", item, rsi.Pane)
	}
	if item := preview(of("sma", talib.EMAType)); item.Placement != chart.PlacementOverlay {
		t.Errorf("sma of ema is placed %q, want over the candles", item.Placement)
	}
	// An oscillator is not in the units of its source.
	if item := preview(of(talib.RSIType, "sma")); item.Placement != chart.PlacementPane || item.Pane != rsi.Pane {
		t.Errorf("rsi of sma = %+v, want the pane %q", item, rsi.Pane)
	}
	// Only previews take an indicator of another indicator.
	if _, err := service.Create(context.Background(), scannerindicator.Indicator{Selection: of("sma", talib.RSIType)}); !errors.Is(err, scannerindicator.ErrInvalidArgument) {
		t.Errorf("Create() of sma of rsi error = %v, want ErrInvalidArgument", err)
	}
	// Markers sit on the line of the source, wherever it is drawn.
	anchor := chart.LineAnchor{IndicatorID: "source", Output: "rsi"}
	if item := preview(of(pivot.Type, talib.RSIType)); item.Placement != chart.PlacementPane || item.Pane != rsi.Pane || len(item.Lines) != 2 ||
		item.Lines[0].Style != chart.LineStyleMarkerAbove || item.Lines[0].Anchor != anchor || item.Lines[1].Anchor != anchor {
		t.Errorf("pivot of rsi = %+v, want two marker lines on the rsi line in the pane %q", item, rsi.Pane)
	}
	if item := preview(of(pivot.Type, talib.EMAType)); item.Placement != chart.PlacementOverlay || len(item.Lines) != 2 || item.Lines[0].Anchor.Output != "ema" {
		t.Errorf("pivot of ema = %+v, want two marker lines on the ema line over the candles", item)
	}
	// A source with a histogram has a pane of its own, named by its id.
	histogram := indicator.Source{Selection: indicator.Selection{Type: "macd"}, Output: "macdhist"}
	if item := preview(indicator.Selection{Type: talib.EMAType, Parameters: indicator.Parameters{"source": histogram.Value()}}); item.Placement != chart.PlacementPane || item.Pane != "source" {
		t.Errorf("ema of macdhist = %+v, want the pane of its source", item)
	}
}

// memoryStore keeps created indicators in memory.
type memoryStore struct {
	scannerindicator.Store
	next int64
}

func (store *memoryStore) CreateScannerIndicator(context.Context, scannerindicator.Indicator) (int64, error) {
	store.next++
	return store.next, nil
}

func (store *memoryStore) UpdateScannerIndicator(context.Context, scannerindicator.Indicator) error {
	return nil
}

// One indicator is shown per period: a table column and a chart line of
// each interval that shows it, and charts count it against their own limit.
func TestIndicatorPeriods(t *testing.T) {
	registry, err := indicator.NewRegistry(talib.New()...)
	if err != nil {
		t.Fatal(err)
	}
	service, err := scannerindicator.New(&memoryStore{}, registry, []string{"yellow.5"}, slog.New(slog.DiscardHandler), func() {})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	rsi := func(period int) indicator.Selection {
		return indicator.Selection{Type: talib.RSIType, Parameters: indicator.Parameters{"period": period}}
	}
	entry, err := service.Create(ctx, scannerindicator.Indicator{Selection: rsi(21), Periods: []scannerindicator.Period{
		{Interval: market.IntervalDay, ShowInTable: true, ShowInChart: true}, {Interval: market.IntervalHour, ShowInTable: true},
	}})
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, column := range service.TableColumns() {
		titles = append(titles, column.ID+" "+column.Title)
	}
	if want := []string{"indicator_1_1h h-rsi-21", "indicator_1_1d d-rsi-21"}; !slices.Equal(titles, want) {
		t.Fatalf("columns = %v, want %v", titles, want)
	}
	if len(service.ChartCatalog(market.IntervalDay)) != 1 || len(service.ChartCatalog(market.IntervalHour)) != 0 {
		t.Fatal("charts draw the indicator outside its chart periods")
	}
	if _, err := service.Create(ctx, scannerindicator.Indicator{Selection: rsi(21)}); !errors.Is(err, scannerindicator.ErrConflict) {
		t.Fatalf("duplicate selection error = %v, want ErrConflict", err)
	}

	weekly := []scannerindicator.Period{{Interval: market.IntervalWeek, ShowInChart: true}}
	for period := range chart.MaxIndicators - 1 {
		if _, err := service.Create(ctx, scannerindicator.Indicator{Selection: rsi(2 + period), Periods: weekly}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := service.Update(ctx, entry.ID, weekly, scannerindicator.Scale{}); err != nil {
		t.Fatalf("the last chart slot: %v", err)
	}
	if _, err := service.Create(ctx, scannerindicator.Indicator{Selection: rsi(30), Periods: weekly}); !errors.Is(err, scannerindicator.ErrLimit) {
		t.Fatalf("chart limit error = %v, want ErrLimit", err)
	}
}

// Charts draw pivots as markers moved back to the pivot candle, which lies
// the confirming candles before the value; markers exist only over candles.
func TestPivotsAreDrawnAsShiftedMarkers(t *testing.T) {
	registry, err := indicator.NewRegistry(pivot.New())
	if err != nil {
		t.Fatal(err)
	}
	service, err := scannerindicator.New(unusedStore{}, registry, []string{"yellow.5"}, slog.New(slog.DiscardHandler), func() {})
	if err != nil {
		t.Fatal(err)
	}
	entry, err := service.Preview(indicator.Selection{Type: pivot.Type, Parameters: indicator.Parameters{"right": 3}})
	if err != nil {
		t.Fatal(err)
	}
	item := entry.ChartIndicator("pivot", "", 0)
	want := []chart.IndicatorLine{
		{Output: "high", Title: "PIVOT 5 3 high", Color: "yellow.5", Style: chart.LineStyleMarkerAbove, Shift: 3},
		{Output: "low", Title: "PIVOT 5 3 low", Color: "yellow.5", Style: chart.LineStyleMarkerBelow, Shift: 3},
	}
	if !slices.Equal(item.Lines, want) {
		t.Fatalf("lines = %+v, want %+v", item.Lines, want)
	}
	item.Placement = chart.PlacementVolume
	if err := chart.ValidateIndicator(registry, item); err == nil {
		t.Fatal("markers outside the candles are accepted")
	}
}
