package draw_test

import (
	"math"
	"reflect"
	"slices"
	"testing"
	"time"

	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/chart/draw"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
)

var start = time.Date(2026, 8, 26, 23, 0, 0, 0, time.UTC)

// hour is the open time of the candle hours after start.
func hour(hours int) time.Time { return start.Add(time.Duration(hours) * time.Hour) }

// candles closes the candle of each of hours at 10 plus its hour, its high
// one above and its low one below.
func candles(hours ...int) []market.Candle {
	result := make([]market.Candle, len(hours))
	for i, at := range hours {
		close := 10 + float64(at)
		result[i] = market.Candle{OpenTime: hour(at), Open: close - 0.5, High: close + 1, Low: close - 1, Close: close}
	}
	return result
}

// series holds values at the candles of the given indexes, NaN elsewhere.
func series(count int, values map[int]float64) []float64 {
	result := make([]float64, count)
	for i := range result {
		value, found := values[i]
		if !found {
			value = math.NaN()
		}
		result[i] = value
	}
	return result
}

func canvas(drawn []market.Candle, outputs []indicator.OutputDescriptor, values map[string][]float64, parameters indicator.Parameters) chart.Canvas {
	return chart.Canvas{
		Candles: drawn, Series: values, Outputs: outputs, Parameters: parameters, Place: chart.PriceScale, Title: "TEST",
		Colors: chart.NewColors(chart.Palette{Colors: []string{"yellow.5", "blue.5"}, Bullish: "green.6", Bearish: "red.6"}),
	}
}

var pivotOutputs = []indicator.OutputDescriptor{
	{Name: "high", Style: indicator.OutputMarkerAbove, Sparse: true},
	{Name: "low", Style: indicator.OutputMarkerBelow, Sparse: true},
}

// shape returns the shape id among shapes.
func shape(t *testing.T, shapes []chart.Shape, id string) chart.Shape {
	t.Helper()
	for _, shape := range shapes {
		if shape.ID == id {
			return shape
		}
	}
	t.Fatalf("no shape %s among %+v", id, shapes)
	return chart.Shape{}
}

// Pivots are drawn the confirming candles before the candle holding their
// value, counting candles, not hours; one moved before the first candle is
// not drawn.
func TestPivotMarkersAreShiftedBackByCandles(t *testing.T) {
	tests := []struct {
		name    string
		candles []market.Candle
		right   int
		want    []chart.Marker
	}{
		{name: "before the first candle", candles: candles(0, 1, 2, 3), right: 2, want: []chart.Marker{
			{Time: hour(1), Calculated: hour(3), Value: 12},
		}},
		// The candle of hour 2 is missing, so the value of hour 3 is drawn on
		// hour 1.
		{name: "missing candle", candles: candles(0, 1, 3), right: 1, want: []chart.Marker{
			{Time: hour(0), Calculated: hour(1), Value: 11},
			{Time: hour(1), Calculated: hour(3), Value: 12},
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			last := len(test.candles) - 1
			values := map[string][]float64{"high": series(len(test.candles), map[int]float64{1: 11, last: 12}), "low": series(len(test.candles), nil)}
			shapes := draw.Pivot{}.Draw(canvas(test.candles, pivotOutputs, values, indicator.Parameters{"left": 5, "right": test.right}))
			high := shape(t, shapes, "high")
			if high.Place != chart.PriceScale || high.Markers.Position != chart.MarkerAbove || high.Markers.Title != "TEST high" || high.Markers.Color != "yellow.5" {
				t.Fatalf("pivot highs = %+v, want yellow markers above the candles", high)
			}
			if !reflect.DeepEqual(high.Markers.Points, test.want) {
				t.Fatalf("markers = %+v, want %+v", high.Markers.Points, test.want)
			}
			if low := shape(t, shapes, "low"); low.Markers.Position != chart.MarkerBelow || low.Markers.Color != "blue.5" {
				t.Fatalf("pivot lows = %+v, want blue markers below the candles", low)
			}
		})
	}
}

// Pivots of another indicator sit on the line of that source, in its place,
// where it has a value; outside the price scale without one they are not
// drawn and take no color.
func TestPivotMarkersSitOnTheLineOfTheirSource(t *testing.T) {
	drawn := candles(0, 1, 2, 3)
	values := map[string][]float64{"high": series(4, map[int]float64{1: 11, 3: 12}), "low": series(4, nil)}
	pane := chart.Place{Placement: chart.PlacementPane, Pane: "rsi"}
	on := canvas(drawn, pivotOutputs, values, indicator.Parameters{"right": 0})
	on.Place = pane
	// The first pivot lies before the first value of the line.
	on.Source = &chart.SourceLine{Place: pane, Values: series(4, map[int]float64{2: 50, 3: 60})}
	high := shape(t, draw.Pivot{}.Draw(on), "high")
	want := []chart.Marker{{Time: hour(3), Calculated: hour(3), Value: 12, LineValue: new(60.0)}}
	if high.Place != pane || !reflect.DeepEqual(high.Markers.Points, want) {
		t.Fatalf("pivot highs = %+v in %+v, want %+v in the pane of the line", high.Markers.Points, high.Place, want)
	}

	off := canvas(drawn, pivotOutputs, values, indicator.Parameters{"right": 0})
	off.Place = chart.Place{Placement: chart.PlacementVolume}
	if shapes := (draw.Pivot{}).Draw(off); len(shapes) != 0 || off.Colors.Next() != "yellow.5" {
		t.Fatalf("pivots off the candles without a line = %+v, want none and no color taken", shapes)
	}
}

// Each divergence joins its two pivots, the earlier one the candles its bars
// hold before the later one, on the RSI line from the RSI to the value and
// on the candles from low to low; both pivots are drawn the confirming
// candles before the value.
func TestDivergenceSegmentsJoinThePivots(t *testing.T) {
	outputs := []indicator.OutputDescriptor{
		{Name: "bull", Style: indicator.OutputMarkerBelow, Sparse: true},
		{Name: "bear", Style: indicator.OutputMarkerAbove, Sparse: true},
		{Name: "rsi", Style: indicator.OutputLine},
		{Name: "bull_bars", Style: indicator.OutputValue, Sparse: true},
		{Name: "bear_bars", Style: indicator.OutputValue, Sparse: true},
	}
	segment := func(calculated, from int, fromValue float64, to int, toValue float64) chart.Segment {
		return chart.Segment{Calculated: hour(calculated), From: chart.SegmentPoint{Time: hour(from), Value: fromValue}, To: chart.SegmentPoint{Time: hour(to), Value: toValue}}
	}
	pane := chart.Place{Placement: chart.PlacementPane, Pane: "divergence"}
	tests := []struct {
		name              string
		candles           []market.Candle
		bull, bars, rsi   map[int]float64
		onLine, onCandles []chart.Segment
	}{
		// The value on candle 2 starts before the first candle; the value on
		// candle 1 has no bars.
		{
			name: "contiguous", candles: candles(0, 1, 2, 3, 4, 5, 6),
			bull: map[int]float64{1: 20, 2: 30, 4: 40, 6: 35}, bars: map[int]float64{2: 5, 4: 3, 6: 1},
			rsi:       map[int]float64{0: 20, 1: 30, 2: 25, 3: 40, 4: 45, 5: 35, 6: 50},
			onLine:    []chart.Segment{segment(4, 0, 20, 3, 40), segment(6, 4, 45, 5, 35)},
			onCandles: []chart.Segment{segment(4, 0, 9, 3, 12), segment(6, 4, 13, 5, 14)},
		},
		// The candle of hour 2 is missing: the value of hour 5 is drawn on
		// hour 4, and its segment starts 3 candles before that, on hour 0.
		{
			name: "missing candle", candles: candles(0, 1, 3, 4, 5),
			bull: map[int]float64{4: 35}, bars: map[int]float64{4: 3},
			rsi:       map[int]float64{0: 20, 1: 21, 2: 23, 3: 24, 4: 25},
			onLine:    []chart.Segment{segment(5, 0, 20, 4, 35)},
			onCandles: []chart.Segment{segment(5, 0, 9, 4, 13)},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			count := len(test.candles)
			values := map[string][]float64{
				"bull": series(count, test.bull), "bull_bars": series(count, test.bars), "rsi": series(count, test.rsi),
				"bear": series(count, nil), "bear_bars": series(count, nil),
			}
			on := canvas(test.candles, outputs, values, indicator.Parameters{"left": 5, "right": 1})
			on.Place = pane
			shapes := draw.Divergence{}.Draw(on)
			if line := shape(t, shapes, "bull-rsi"); line.Place != pane || line.Segments.Color != "green.6" || !reflect.DeepEqual(line.Segments.Items, test.onLine) {
				t.Fatalf("segments on the RSI = %+v in %+v, want %+v in its pane", line.Segments, line.Place, test.onLine)
			}
			if candles := shape(t, shapes, "bull-candles"); candles.Place != chart.PriceScale || !reflect.DeepEqual(candles.Segments.Items, test.onCandles) {
				t.Fatalf("segments on the candles = %+v in %+v, want %+v over the candles", candles.Segments, candles.Place, test.onCandles)
			}
			for _, marker := range shape(t, shapes, "bull").Markers.Points {
				index := slices.IndexFunc(test.candles, func(candle market.Candle) bool { return candle.OpenTime.Equal(marker.Time) })
				if marker.LineValue == nil || *marker.LineValue != test.rsi[index] {
					t.Fatalf("bullish marker %+v, want it on the RSI of its candle", marker)
				}
			}
		})
	}
}

// The SUPERTREND line takes the color of the trend, keeps its own where the
// trend is zero or unknown, and breaks where the trend flips; its fill runs
// to the middle of the candle body in the color of the trend wherever there
// is one.
func TestSupertrendColorsAndFillsByTheTrend(t *testing.T) {
	drawn := candles(0, 1, 2, 3, 4, 5)
	values := map[string][]float64{
		"supertrend": series(6, map[int]float64{0: 10, 1: 11, 2: 12, 3: 13, 4: 14}),
		"trend":      series(6, map[int]float64{0: 1, 1: 1, 2: -1, 3: 0, 5: 1}),
	}
	outputs := []indicator.OutputDescriptor{{Name: "supertrend", Style: indicator.OutputLine}, {Name: "trend", Style: indicator.OutputValue}}
	shapes := draw.Supertrend{}.Draw(canvas(drawn, outputs, values, indicator.Parameters{"period": 10, "multiplier": 3}))
	line := shape(t, shapes, "supertrend")
	wantPoints := []chart.LinePoint{
		{Time: hour(0), Value: 10, Color: "green.6"},
		{Time: hour(1), Value: 11, Color: "green.6"},
		{Time: hour(2), Value: 12, Color: "red.6", Break: true},
		{Time: hour(3), Value: 13},
		{Time: hour(4), Value: 14},
	}
	if line.Line.Color != "yellow.5" || line.Line.Title != "TEST supertrend" || !reflect.DeepEqual(line.Line.Points, wantPoints) {
		t.Fatalf("line = %+v, want %+v", line.Line, wantPoints)
	}
	wantFill := []chart.AreaPoint{
		{Time: hour(0), Value: 10, Base: 9.75, Color: "green.6"},
		{Time: hour(1), Value: 11, Base: 10.75, Color: "green.6"},
		{Time: hour(2), Value: 12, Base: 11.75, Color: "red.6"},
	}
	if fill := shape(t, shapes, "fill"); fill.Place != chart.PriceScale || !reflect.DeepEqual(fill.Area.Points, wantFill) {
		t.Fatalf("fill = %+v, want %+v", fill.Area, wantFill)
	}
}
