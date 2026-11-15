package draw

import "crypto-scanner/internal/chart"

// Pivot draws pivot highs above and pivot lows below the pivot candle, which
// lies the confirming candles before the candle holding the value: over the
// candles, or on the line of the source the pivots are found in.
type Pivot struct{}

func (Pivot) Outputs() []string { return []string{"high", "low"} }

func (Pivot) Draw(canvas chart.Canvas) []chart.Shape {
	right := shift(canvas, "right")
	shapes := make([]chart.Shape, 0, 2)
	for _, output := range []struct {
		name     string
		position chart.MarkerPosition
	}{{"high", chart.MarkerAbove}, {"low", chart.MarkerBelow}} {
		if shape, ok := sourceMarkers(canvas, output.name, output.position, right); ok {
			shapes = append(shapes, shape)
		}
	}
	return shapes
}
