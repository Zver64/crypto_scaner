// Package draw holds the chart drawers: the default one, which draws every
// output as its style says, and those of the indicators drawn otherwise.
// Only these drawers know particular indicators.
package draw

import (
	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/indicator"
)

// Default draws every output as its style says: the line styles as a solid
// line and the markers on the candle holding each value; value outputs are
// not drawn.
type Default struct{}

func (Default) Outputs() []string { return nil }

func (Default) Draw(canvas chart.Canvas) []chart.Shape {
	shapes := make([]chart.Shape, 0, len(canvas.Outputs))
	for _, output := range canvas.Outputs {
		switch {
		case chart.LineOutputStyle(output.Style):
			shapes = append(shapes, line(canvas, output.Name, canvas.Colors.Next()))
		case output.Style == indicator.OutputMarkerAbove || output.Style == indicator.OutputMarkerBelow:
			if shape, ok := sourceMarkers(canvas, output.Name, position(output.Style), 0); ok {
				shapes = append(shapes, shape)
			}
		}
	}
	return shapes
}

// line draws the values of output as a line where the indicator is drawn,
// its shape named after the output.
func line(canvas chart.Canvas, output, color string) chart.Shape {
	points := make([]chart.LinePoint, 0, len(canvas.Candles))
	for i, candle := range canvas.Candles {
		if value, ok := canvas.Value(output, i); ok {
			points = append(points, chart.LinePoint{Time: candle.OpenTime.UTC(), Value: value})
		}
	}
	return chart.Shape{ID: output, Place: canvas.Place, Line: &chart.Line{Title: canvas.LineTitle(output), Color: color, Points: points}}
}

// sourceMarkers draws the values of output as markers shift candles before
// the candle holding each, in the next palette color: on the line of the
// source of an indicator of another indicator, or over the candles of an
// indicator drawn over them. Anywhere else nothing is drawn, no color is
// taken, and ok is false.
func sourceMarkers(canvas chart.Canvas, output string, position chart.MarkerPosition, shift int) (shape chart.Shape, ok bool) {
	switch {
	case canvas.Source != nil:
		return markers(canvas, output, position, shift, canvas.Colors.Next(), canvas.Source), true
	case canvas.Place == chart.PriceScale:
		return markers(canvas, output, position, shift, canvas.Colors.Next(), nil), true
	}
	return chart.Shape{}, false
}

// markers draws the values of output as markers shift candles before the
// candle holding each, its shape named after the output. On a line, a
// marker is drawn only where the line has a value, in the place of the
// line; without one, on the candles over the price scale. A marker moved
// before the first candle is not drawn.
func markers(canvas chart.Canvas, output string, position chart.MarkerPosition, shift int, color string, on *chart.SourceLine) chart.Shape {
	place := chart.PriceScale
	if on != nil {
		place = on.Place
	}
	points := make([]chart.Marker, 0)
	for i, candle := range canvas.Candles {
		value, ok := canvas.Value(output, i)
		drawn := i - shift
		if !ok || drawn < 0 {
			continue
		}
		marker := chart.Marker{Time: canvas.Candles[drawn].OpenTime.UTC(), Calculated: candle.OpenTime.UTC(), Value: value}
		if on != nil {
			at := on.Values[drawn]
			if indicator.Empty(at) {
				continue
			}
			marker.LineValue = &at
		}
		points = append(points, marker)
	}
	return chart.Shape{ID: output, Place: place, Markers: &chart.Markers{Title: canvas.LineTitle(output), Color: color, Position: position, Points: points}}
}

func position(style indicator.OutputStyle) chart.MarkerPosition {
	if style == indicator.OutputMarkerBelow {
		return chart.MarkerBelow
	}
	return chart.MarkerAbove
}

// shift reads the integer parameter key: the candles a marker is drawn
// before the candle holding its value, such as the candles that confirm a
// pivot.
func shift(canvas chart.Canvas, key string) int {
	value, err := indicator.ParseNumber(canvas.Parameters[key])
	if err != nil {
		return 0
	}
	return int(value)
}
