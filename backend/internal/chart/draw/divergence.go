package draw

import "crypto-scanner/internal/chart"

// Divergence draws the RSI line of the divergences, bullish divergences below
// and bearish ones above it on the later of their two pivots, which lies the
// confirming candles before the candle holding the value, and the lines
// between the two pivots: on the RSI line and on the candles, from low to low
// below them or from high to high above them.
type Divergence struct{}

func (Divergence) Outputs() []string {
	return []string{"bull", "bear", "rsi", "bull_bars", "bear_bars"}
}

func (Divergence) Draw(canvas chart.Canvas) []chart.Shape {
	right := shift(canvas, "right")
	rsi := &chart.SourceLine{Place: canvas.Place, Values: canvas.Series["rsi"]}
	sides := []struct {
		output, bars string
		position     chart.MarkerPosition
		color        string
	}{
		{"bull", "bull_bars", chart.MarkerBelow, canvas.Colors.Bullish()},
		{"bear", "bear_bars", chart.MarkerAbove, canvas.Colors.Bearish()},
	}
	shapes := make([]chart.Shape, 0, 7)
	for _, side := range sides {
		shape := markers(canvas, side.output, side.position, right, side.color, rsi)
		shape.Markers.OwnSegments = true
		shapes = append(shapes, shape)
	}
	shapes = append(shapes, line(canvas, "rsi", canvas.Colors.Next()))
	for _, side := range sides {
		onLine, onCandles := divergenceSegments(canvas, side.output, side.bars, side.position, right)
		shapes = append(shapes,
			chart.Shape{ID: side.output + "-rsi", Place: canvas.Place, Segments: &chart.Segments{Color: side.color, Items: onLine}},
			chart.Shape{ID: side.output + "-candles", Place: chart.PriceScale, Segments: &chart.Segments{Color: side.color, Items: onCandles}},
		)
	}
	return shapes
}

// divergenceSegments joins the two pivots of each divergence of output, each
// drawn shift candles before the candle holding the value: the earlier pivot
// lies the candles that bars holds there before the later one. On the RSI
// line a segment runs from the RSI at the earlier pivot to the value at the
// later one; on the candles, from low to low or high to high. A segment with
// a pivot before the first candle or the RSI missing there is not drawn.
func divergenceSegments(canvas chart.Canvas, output, bars string, position chart.MarkerPosition, shift int) (onLine, onCandles []chart.Segment) {
	onLine, onCandles = make([]chart.Segment, 0), make([]chart.Segment, 0)
	edge := func(index int) float64 {
		if position == chart.MarkerBelow {
			return canvas.Candles[index].Low
		}
		return canvas.Candles[index].High
	}
	for i, candle := range canvas.Candles {
		value, ok := canvas.Value(output, i)
		count, counted := canvas.Value(bars, i)
		if !ok || !counted || count <= 0 {
			continue
		}
		from, to := i-int(count)-shift, i-shift
		if from < 0 {
			continue
		}
		start, end := canvas.Candles[from].OpenTime.UTC(), canvas.Candles[to].OpenTime.UTC()
		calculated := candle.OpenTime.UTC()
		if rsi, ok := canvas.Value("rsi", from); ok {
			onLine = append(onLine, chart.Segment{Calculated: calculated, From: chart.SegmentPoint{Time: start, Value: rsi}, To: chart.SegmentPoint{Time: end, Value: value}})
		}
		onCandles = append(onCandles, chart.Segment{Calculated: calculated, From: chart.SegmentPoint{Time: start, Value: edge(from)}, To: chart.SegmentPoint{Time: end, Value: edge(to)}})
	}
	return onLine, onCandles
}
