package chart_test

import (
	"context"
	"log/slog"
	"reflect"
	"slices"
	"testing"
	"time"

	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/chart/draw"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/indicator/pivot"
	indicatortalib "crypto-scanner/internal/indicator/talib"
	"crypto-scanner/internal/market"
)

// drawingRegistry has the TA-Lib functions, the pivots, and the divergences.
func drawingRegistry(t *testing.T) *indicator.Registry {
	t.Helper()
	modules := indicatortalib.New()
	rsi := modules[slices.IndexFunc(modules, func(module indicator.Implementation) bool {
		return module.Describe().Type == indicatortalib.RSIType
	})]
	registry, err := indicator.NewRegistry(append(modules, pivot.New(), pivot.NewDivergence(rsi))...)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// applicationDrawers draws as the application does, with the palette
// yellow, blue, violet.
func applicationDrawers(t *testing.T, registry *indicator.Registry) *chart.Drawers {
	t.Helper()
	drawers, err := chart.NewDrawers(registry, chart.Palette{Colors: []string{"yellow.5", "blue.5", "violet.5"}, Bullish: "green.6", Bearish: "red.6"}, draw.Default{}, map[indicator.Type]chart.Drawer{
		indicatortalib.SupertrendType: draw.Supertrend{},
		pivot.Type:                    draw.Pivot{},
		pivot.DivergenceType:          draw.Divergence{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return drawers
}

// drawn calculates each selection over candles under its id and draws them.
func drawn(t *testing.T, registry *indicator.Registry, drawers *chart.Drawers, candles []market.Candle, ids []string, selections []indicator.Selection, scales ...chart.IndicatorScale) chart.Drawing {
	t.Helper()
	calculations, err := registry.CalculateCandles(market.IntervalHour, candles, selections)
	if err != nil {
		t.Fatal(err)
	}
	items := make([]chart.Calculated, len(calculations))
	for i, calculation := range calculations {
		items[i] = chart.Calculated{ID: ids[i], Calculation: calculation}
		if i < len(scales) {
			items[i].Scale = scales[i]
		}
	}
	drawing, err := drawers.Draw(candles, items)
	if err != nil {
		t.Fatal(err)
	}
	return drawing
}

func findShape(t *testing.T, drawing chart.Drawing, id string) chart.Shape {
	t.Helper()
	index := slices.IndexFunc(drawing.Shapes, func(shape chart.Shape) bool { return shape.ID == id })
	if index < 0 {
		t.Fatalf("no shape %s", id)
	}
	return drawing.Shapes[index]
}

func TestNewDrawersChecksTheOutputsADrawerReads(t *testing.T) {
	registry := drawingRegistry(t)
	palette := chart.Palette{Colors: []string{"yellow.5"}, Bullish: "green.6", Bearish: "red.6"}
	for name, drawers := range map[string]map[indicator.Type]chart.Drawer{
		"unknown type":    {"unknown": draw.Pivot{}},
		"missing outputs": {indicatortalib.RSIType: draw.Pivot{}},
	} {
		if _, err := chart.NewDrawers(registry, palette, draw.Default{}, drawers); err == nil {
			t.Errorf("%s: NewDrawers() accepted it", name)
		}
	}
}

// An overlay fits the price scale only over prices and the volume scale only
// over volumes.
func TestPlacementFollowsTheUnitsOfTheFields(t *testing.T) {
	registry := drawingRegistry(t)
	drawers := applicationDrawers(t, registry)
	source := func(field string) float64 { return float64(slices.Index(indicator.CandleFields, field)) }
	tests := []struct {
		name          string
		indicatorType indicator.Type
		field         string
		want          chart.Placement
	}{
		{name: "overlay of prices", indicatorType: indicatortalib.EMAType, field: "close", want: chart.PlacementOverlay},
		{name: "overlay of volumes", indicatorType: indicatortalib.EMAType, field: "volume", want: chart.PlacementVolume},
		{name: "overlay of quote volumes", indicatorType: indicatortalib.EMAType, field: "quote_volume", want: chart.PlacementPane},
		{name: "overlay of trade counts", indicatorType: indicatortalib.EMAType, field: "trades", want: chart.PlacementPane},
		{name: "oscillator of volumes", indicatorType: indicatortalib.RSIType, field: "volume", want: chart.PlacementPane},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			placement, err := drawers.Placement(indicator.Selection{Type: test.indicatorType, Parameters: indicator.Parameters{"source": source(test.field)}})
			if err != nil {
				t.Fatal(err)
			}
			if placement != test.want {
				t.Fatalf("placement = %q, want %q", placement, test.want)
			}
		})
	}
}

// An indicator of another indicator in the units of its input is drawn where
// that source is, and its markers sit on the line of the source; any other
// keeps a pane of its own type. Its titles name the source.
func TestIndicatorsOfIndicatorsAreDrawnWithTheirSource(t *testing.T) {
	registry := drawingRegistry(t)
	of := func(indicatorType indicator.Type, source indicator.Type, output string) indicator.Selection {
		nested := indicator.Source{Selection: indicator.Selection{Type: source}, Output: output}
		return indicator.Selection{Type: indicatorType, Parameters: indicator.Parameters{"source": nested.Value()}}
	}
	ids := []string{"rsi", "ema", "macd", "sma-rsi", "sma-ema", "rsi-sma", "pivot-rsi", "pivot-ema", "ema-hist"}
	selections := []indicator.Selection{
		{Type: indicatortalib.RSIType}, {Type: indicatortalib.EMAType}, {Type: "macd"},
		of("sma", indicatortalib.RSIType, "rsi"), of("sma", indicatortalib.EMAType, "ema"), of(indicatortalib.RSIType, "sma", "sma"),
		of(pivot.Type, indicatortalib.RSIType, "rsi"), of(pivot.Type, indicatortalib.EMAType, "ema"), of(indicatortalib.EMAType, "macd", "macdhist"),
	}
	drawing := drawn(t, registry, applicationDrawers(t, registry), testCandles(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), 300), ids, selections)
	rsi := findShape(t, drawing, "rsi/rsi").Place
	if rsi != (chart.Place{Placement: chart.PlacementPane, Pane: "rsi:price"}) {
		t.Fatalf("rsi lies in %+v, want the pane every rsi of prices shares", rsi)
	}
	if line := findShape(t, drawing, "sma-rsi/sma"); line.Place != rsi || line.Line.Title != "SMA 30 RSI 14" {
		t.Errorf("sma of rsi = %q in %+v, want SMA 30 RSI 14 in the pane of the rsi", line.Line.Title, line.Place)
	}
	if place := findShape(t, drawing, "sma-ema/sma").Place; place != chart.PriceScale {
		t.Errorf("sma of ema lies in %+v, want over the candles", place)
	}
	// An oscillator is not in the units of its source.
	if place := findShape(t, drawing, "rsi-sma/rsi").Place; place != rsi {
		t.Errorf("rsi of sma lies in %+v, want the pane of the rsi", place)
	}
	for _, output := range []string{"high", "low"} {
		markers := findShape(t, drawing, "pivot-rsi/"+output)
		if markers.Place != rsi || len(markers.Markers.Points) == 0 || slices.ContainsFunc(markers.Markers.Points, func(marker chart.Marker) bool { return marker.LineValue == nil }) {
			t.Errorf("pivot %s of rsi = %+v, want markers on the rsi line in its pane", output, markers)
		}
	}
	if markers := findShape(t, drawing, "pivot-ema/high"); markers.Place != chart.PriceScale || markers.Markers.Points[0].LineValue == nil {
		t.Errorf("pivot high of ema = %+v, want markers on the ema line over the candles", markers)
	}
	// A source with a histogram has a pane of its own, named by its id.
	if macd, hist := findShape(t, drawing, "macd/macdhist").Place, findShape(t, drawing, "ema-hist/ema").Place; macd.Pane != "macd" || hist != macd {
		t.Errorf("ema of macdhist lies in %+v, want the own pane of the macd, %+v", hist, macd)
	}
	var panes []string
	for _, pane := range drawing.Panes {
		panes = append(panes, pane.Key)
	}
	if want := []string{"rsi:price", "macd"}; !slices.Equal(panes, want) {
		t.Errorf("panes = %v, want %v", panes, want)
	}
}

// Shapes take the palette colors in turn across the chart, except those with
// a fixed meaning, which leave the palette to the others.
func TestShapesTakePaletteColorsInTurn(t *testing.T) {
	registry := drawingRegistry(t)
	drawing := drawn(t, registry, applicationDrawers(t, registry), testCandles(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), 100),
		[]string{"divergence", "rsi"}, []indicator.Selection{{Type: pivot.DivergenceType}, {Type: indicatortalib.RSIType, Parameters: indicator.Parameters{"period": 21}}})
	var colors []string
	for _, shape := range drawing.Shapes {
		switch {
		case shape.Line != nil:
			colors = append(colors, shape.ID+" "+shape.Line.Color)
		case shape.Markers != nil:
			colors = append(colors, shape.ID+" "+shape.Markers.Color)
		}
	}
	if want := []string{"divergence/bull green.6", "divergence/bear red.6", "divergence/rsi yellow.5", "rsi/rsi blue.5"}; !slices.Equal(colors, want) {
		t.Fatalf("colors = %v, want %v", colors, want)
	}
}

// A pane of one indicator has its scale; a pane indicators share spans their
// bounds, an unbounded side staying unbounded, and draws no levels.
func TestSharedPanesMergeTheirScales(t *testing.T) {
	registry := drawingRegistry(t)
	candles := testCandles(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), 100)
	rsi := func(period int) indicator.Selection {
		return indicator.Selection{Type: indicatortalib.RSIType, Parameters: indicator.Parameters{"period": period}}
	}
	levels := []chart.IndicatorLevel{{Value: 30, Title: "30"}, {Value: 70, Title: "70"}}
	first := chart.IndicatorScale{Min: new(0.0), Max: new(100.0), Levels: levels}
	second := chart.IndicatorScale{Min: new(10.0), Max: new(90.0), Levels: levels[:1]}
	alone := drawn(t, registry, applicationDrawers(t, registry), candles, []string{"rsi"}, []indicator.Selection{rsi(14)}, first)
	if want := []chart.Pane{{Key: "rsi:price", Scale: first}}; !reflect.DeepEqual(alone.Panes, want) {
		t.Fatalf("pane of one rsi = %+v, want %+v", alone.Panes, want)
	}
	shared := drawn(t, registry, applicationDrawers(t, registry), candles, []string{"fast", "slow"}, []indicator.Selection{rsi(14), rsi(21)}, first, second)
	if want := []chart.Pane{{Key: "rsi:price", Scale: chart.IndicatorScale{Min: new(0.0), Max: new(100.0), Levels: []chart.IndicatorLevel{}}}}; !reflect.DeepEqual(shared.Panes, want) {
		t.Fatalf("pane of two rsi = %+v, want %+v", shared.Panes, want)
	}
}

// Live charts give a pane the scale configured for the catalog indicator
// with the requested selection, however the request spells it.
func TestLiveChartsTakeTheScaleOfTheCatalog(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	registry := drawingRegistry(t)
	selection, err := registry.Normalize(indicator.Selection{Type: indicatortalib.RSIType})
	if err != nil {
		t.Fatal(err)
	}
	scale := chart.IndicatorScale{Levels: []chart.IndicatorLevel{{Value: 30, Title: "30"}}}
	service, err := chart.NewService(&storeStub{page: market.CandlePage{Candles: testCandles(start, 50)}}, registry, applicationDrawers(t, registry),
		catalog{{ID: "indicator-1", Selection: selection, Scale: scale}}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	page, err := service.Build(context.Background(), chart.Request{Symbol: "BTCUSDT", Interval: market.IntervalHour, Limit: 50,
		Indicators: []indicator.Selection{{Type: indicatortalib.RSIType, Parameters: indicator.Parameters{"period": float64(14)}}}})
	if err != nil {
		t.Fatal(err)
	}
	if want := []chart.Pane{{Key: "rsi:price", Scale: scale}}; !reflect.DeepEqual(page.Panes, want) {
		t.Fatalf("panes = %+v, want %+v", page.Panes, want)
	}
}

type catalog []chart.CatalogIndicator

func (items catalog) ChartCatalog(market.CandleInterval) []chart.CatalogIndicator { return items }

// The catalog lays out the panes of a chart that requests exactly it, before
// any candle is drawn.
func TestCatalogPanesAreThoseOfItsCharts(t *testing.T) {
	registry := drawingRegistry(t)
	normalized := func(selection indicator.Selection) indicator.Selection {
		t.Helper()
		selection, err := registry.Normalize(selection)
		if err != nil {
			t.Fatal(err)
		}
		return selection
	}
	levels := chart.IndicatorScale{Min: new(0.0), Max: new(100.0), Levels: []chart.IndicatorLevel{{Value: 30, Title: "30"}}}
	items := catalog{
		{ID: "indicator-1", Selection: normalized(indicator.Selection{Type: indicatortalib.RSIType}), Scale: levels},
		{ID: "indicator-2", Selection: normalized(indicator.Selection{Type: "macd"})},
		{ID: "indicator-3", Selection: normalized(indicator.Selection{Type: pivot.DivergenceType})},
		{ID: "indicator-4", Selection: normalized(indicator.Selection{Type: indicatortalib.RSIType, Parameters: indicator.Parameters{"period": 21}}), Scale: levels},
		{ID: "indicator-5", Selection: normalized(indicator.Selection{Type: indicatortalib.EMAType})},
	}
	store := &storeStub{page: market.CandlePage{Candles: testCandles(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), 100)}}
	service, err := chart.NewService(store, registry, applicationDrawers(t, registry), items, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	laidOut, err := service.Catalog(market.IntervalHour)
	if err != nil {
		t.Fatal(err)
	}
	selections := make([]indicator.Selection, len(items))
	for i, item := range items {
		selections[i] = item.Selection
	}
	page, err := service.Build(context.Background(), chart.Request{Symbol: "BTCUSDT", Interval: market.IntervalHour, Limit: 100, Indicators: selections})
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, pane := range laidOut.Panes {
		keys = append(keys, pane.Key)
	}
	if want := []string{"rsi:price", "1", "divergence:price"}; !slices.Equal(keys, want) || !reflect.DeepEqual(laidOut.Panes, page.Panes) {
		t.Fatalf("catalog panes = %+v, chart panes %+v, want the panes %v of both", laidOut.Panes, page.Panes, want)
	}
}
