package chart

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
)

// Drawer draws the indicators of one type, or of every type without a drawer
// of its own, as shapes.
type Drawer interface {
	// Outputs names the outputs Draw reads; registration checks that the
	// type has them. A drawer of every type reads what each one declares and
	// returns nil.
	Outputs() []string
	// Draw returns the shapes of the indicator on the canvas. The shapes and
	// their ids depend only on the indicator and where it is drawn, never on
	// its values, so every frame of a chart has the same shapes.
	Draw(canvas Canvas) []Shape
}

// Palette is the colors of chart shapes, as client theme color tokens.
type Palette struct {
	// Colors are taken in turn by the shapes across a chart.
	Colors []string
	// Bullish and Bearish color what has a fixed meaning, such as the up and
	// down trend of the SUPERTREND and bullish and bearish divergences.
	Bullish string
	Bearish string
}

// Colors hands out the palette colors of one chart in turn.
type Colors struct {
	palette Palette
	next    int
}

// NewColors starts handing out the colors of palette from its first one.
func NewColors(palette Palette) *Colors { return &Colors{palette: palette} }

// Next returns the next color of the palette.
func (colors *Colors) Next() string {
	color := colors.palette.Colors[colors.next%len(colors.palette.Colors)]
	colors.next++
	return color
}

func (colors *Colors) Bullish() string { return colors.palette.Bullish }
func (colors *Colors) Bearish() string { return colors.palette.Bearish }

// Canvas is what a drawer draws one indicator with.
type Canvas struct {
	// Candles are the candles of the chart, oldest first.
	Candles []market.Candle
	// Series holds the values of every output at each of the candles, NaN
	// where it has none.
	Series     map[string][]float64
	Outputs    []indicator.OutputDescriptor
	Parameters indicator.Parameters
	// Place is where the indicator is drawn.
	Place Place
	// Source is the line of the output an indicator of another indicator
	// reads, when the chart draws that output as a line; nil otherwise.
	Source *SourceLine
	// Title names the indicator, such as "RSI 14".
	Title  string
	Colors *Colors
}

// SourceLine is a line another indicator of the chart draws.
type SourceLine struct {
	Place Place
	// Values are its values at each of the candles, NaN where it has none.
	Values []float64
}

// LineTitle titles the shape of output: the title of the indicator,
// followed by the output when it has several.
func (canvas Canvas) LineTitle(output string) string {
	if len(canvas.Outputs) > 1 {
		return canvas.Title + " " + output
	}
	return canvas.Title
}

// Value returns the value of output at the candle index; ok is false where
// it has none.
func (canvas Canvas) Value(output string, index int) (value float64, ok bool) {
	values := canvas.Series[output]
	if index < 0 || index >= len(values) || indicator.Empty(values[index]) {
		return 0, false
	}
	return values[index], true
}

// Drawers draws indicators with the drawer of their type, or with the
// fallback.
type Drawers struct {
	registry *indicator.Registry
	palette  Palette
	fallback Drawer
	byType   map[indicator.Type]Drawer
}

// NewDrawers registers drawers by indicator type. Each type must be
// registered with registry, which describes the outputs its drawer reads.
func NewDrawers(registry *indicator.Registry, palette Palette, fallback Drawer, drawers map[indicator.Type]Drawer) (*Drawers, error) {
	if registry == nil || fallback == nil || len(palette.Colors) == 0 || strings.TrimSpace(palette.Bullish) == "" || strings.TrimSpace(palette.Bearish) == "" {
		return nil, errors.New("chart drawers need a registry, a fallback, palette colors, and bullish and bearish colors")
	}
	for indicatorType, drawer := range drawers {
		descriptor, err := registry.Describe(indicatorType)
		if err != nil {
			return nil, fmt.Errorf("drawer of %q: %w", indicatorType, err)
		}
		if drawer == nil {
			return nil, fmt.Errorf("drawer of %q is nil", indicatorType)
		}
		for _, output := range drawer.Outputs() {
			if !slices.ContainsFunc(descriptor.Outputs, func(described indicator.OutputDescriptor) bool { return described.Name == output }) {
				return nil, fmt.Errorf("drawer of %q reads output %q, which it does not have", indicatorType, output)
			}
		}
	}
	return &Drawers{registry: registry, palette: palette, fallback: fallback, byType: drawers}, nil
}

// Placement is where charts draw the selection.
func (drawers *Drawers) Placement(selection indicator.Selection) (Placement, error) {
	layout, err := layoutOf(drawers.registry, selection)
	return layout.placement, err
}

// Title names the selection on charts, such as "RSI 14" or "SMA 30 RSI 14".
func (drawers *Drawers) Title(selection indicator.Selection) (string, error) {
	layout, err := layoutOf(drawers.registry, selection)
	return layout.title, err
}

// Calculated is an indicator calculated over the candles of a chart.
type Calculated struct {
	// ID names the indicator within the chart: its shapes and the pane it
	// has of its own are named after it.
	ID string
	indicator.Calculation
	// Scale is the configured value axis of its pane.
	Scale IndicatorScale
}

// Drawing is what a chart draws: the shapes of its indicators and the panes
// they lie in, in the order of the indicators.
type Drawing struct {
	Shapes []Shape
	Panes  []Pane
}

// Pane is an indicator pane below the candles.
type Pane struct {
	Key   string
	Scale IndicatorScale
}

// Draw draws indicators, in order, over candles, the candles of the chart
// oldest first, on which the points of their series lie. The shapes take the
// palette colors in turn across the indicators.
func (drawers *Drawers) Draw(candles []market.Candle, indicators []Calculated) (Drawing, error) {
	layouts := make([]layout, len(indicators))
	for i, item := range indicators {
		var err error
		if layouts[i], err = layoutOf(drawers.registry, indicator.Selection{Type: item.Type, Parameters: item.Parameters}); err != nil {
			return Drawing{}, fmt.Errorf("draw %q: %w", item.Type, err)
		}
	}
	positions := make(map[time.Time]int, len(candles))
	for i, candle := range candles {
		positions[candle.OpenTime.UTC()] = i
	}
	series := make([]map[string][]float64, len(indicators))
	for i, item := range indicators {
		series[i] = make(map[string][]float64, len(item.Series))
		for _, named := range item.Series {
			series[i][named.Name] = aligned(positions, len(candles), named.Points)
		}
	}
	places := make([]Place, len(indicators))
	for i, layout := range layouts {
		places[i] = Place{Placement: layout.placement}
		if layout.placement != PlacementPane {
			continue
		}
		switch source := sourceOf(layouts, layout); {
		case layout.pane != "":
			places[i].Pane = layout.pane
		case layout.sourcePane && source >= 0:
			places[i].Pane = indicators[source].ID
		default:
			places[i].Pane = indicators[i].ID
		}
	}
	colors := NewColors(drawers.palette)
	drawing := Drawing{Shapes: make([]Shape, 0), Panes: make([]Pane, 0)}
	for i, item := range indicators {
		canvas := Canvas{
			Candles: candles, Series: series[i], Outputs: layouts[i].descriptor.Outputs, Parameters: layouts[i].selection.Parameters,
			Place: places[i], Title: layouts[i].title, Colors: colors,
		}
		if source := sourceOf(layouts, layouts[i]); source >= 0 && drawnAsLine(layouts[source].descriptor, layouts[i].source.Output) {
			canvas.Source = &SourceLine{Place: places[source], Values: series[source][layouts[i].source.Output]}
		}
		drawer, found := drawers.byType[item.Type]
		if !found {
			drawer = drawers.fallback
		}
		shapes := drawer.Draw(canvas)
		for _, shape := range shapes {
			shape.ID = item.ID + "/" + shape.ID
			shape.Indicator = item.ID
			drawing.Shapes = append(drawing.Shapes, shape)
		}
		if len(shapes) == 0 || places[i].Placement != PlacementPane {
			continue
		}
		key := places[i].Pane
		index := slices.IndexFunc(drawing.Panes, func(pane Pane) bool { return pane.Key == key })
		if index < 0 {
			drawing.Panes = append(drawing.Panes, Pane{Key: key, Scale: item.Scale})
			continue
		}
		drawing.Panes[index].Scale = mergeScales(drawing.Panes[index].Scale, item.Scale)
	}
	return drawing, nil
}

// sourceOf returns the index of the layout of the source of layout among
// layouts, or -1 when it has none or it is not drawn.
func sourceOf(layouts []layout, of layout) int {
	if of.source == nil {
		return -1
	}
	return slices.IndexFunc(layouts, func(candidate layout) bool { return sameSelection(candidate.selection, of.source.Selection) })
}

// drawnAsLine reports whether charts draw output of an indicator as a line.
func drawnAsLine(descriptor indicator.Descriptor, output string) bool {
	return slices.ContainsFunc(descriptor.Outputs, func(described indicator.OutputDescriptor) bool {
		return described.Name == output && LineOutputStyle(described.Style)
	})
}

// LineOutputStyle reports the styles of outputs charts draw as lines.
func LineOutputStyle(style indicator.OutputStyle) bool {
	switch style {
	case indicator.OutputLine, indicator.OutputDashedLine, indicator.OutputHistogram, indicator.OutputUpperLimit, indicator.OutputLowerLimit:
		return true
	}
	return false
}

// aligned returns the values of points at each of count candles, NaN where
// there is none; positions holds the index of each candle by its open time.
func aligned(positions map[time.Time]int, count int, points []indicator.Point) []float64 {
	values := make([]float64, count)
	for i := range values {
		values[i] = math.NaN()
	}
	for _, point := range points {
		if index, found := positions[point.Time.UTC()]; found {
			values[index] = point.Value
		}
	}
	return values
}

// mergeScales is the scale of a pane indicators share: it spans the bounds of
// both, an unbounded side staying unbounded, and draws no levels.
func mergeScales(first, second IndicatorScale) IndicatorScale {
	merged := IndicatorScale{Levels: []IndicatorLevel{}}
	if first.Min != nil && second.Min != nil {
		merged.Min = new(min(*first.Min, *second.Min))
	}
	if first.Max != nil && second.Max != nil {
		merged.Max = new(max(*first.Max, *second.Max))
	}
	return merged
}
