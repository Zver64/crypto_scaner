package draw

import "crypto-scanner/internal/chart"

// Supertrend draws the SUPERTREND line in the bullish color where the trend
// is up and in the bearish color where it is down, broken where the trend
// flips and the line jumps to the other side of the candles; where the trend
// is zero or unknown the line keeps its palette color. Over the candles, it
// also fills the area between the line and the middle of the candle body in
// the color of the trend.
type Supertrend struct{}

func (Supertrend) Outputs() []string { return []string{"supertrend", "trend"} }

func (Supertrend) Draw(canvas chart.Canvas) []chart.Shape {
	// trend is the color of the trend at a candle, empty where it is zero or
	// unknown.
	trend := func(index int) string {
		value, ok := canvas.Value("trend", index)
		switch {
		case !ok || value == 0:
			return ""
		case value > 0:
			return canvas.Colors.Bullish()
		}
		return canvas.Colors.Bearish()
	}
	points := make([]chart.LinePoint, 0, len(canvas.Candles))
	fill := make([]chart.AreaPoint, 0, len(canvas.Candles))
	for i, candle := range canvas.Candles {
		value, ok := canvas.Value("supertrend", i)
		if !ok {
			continue
		}
		color, before := trend(i), trend(i-1)
		_, joined := canvas.Value("supertrend", i-1)
		points = append(points, chart.LinePoint{
			Time: candle.OpenTime.UTC(), Value: value, Color: color,
			Break: joined && color != "" && before != "" && before != color,
		})
		if color != "" {
			fill = append(fill, chart.AreaPoint{Time: candle.OpenTime.UTC(), Value: value, Base: (candle.Open + candle.Close) / 2, Color: color})
		}
	}
	shapes := []chart.Shape{{ID: "supertrend", Place: canvas.Place, Line: &chart.Line{Title: canvas.LineTitle("supertrend"), Color: canvas.Colors.Next(), Points: points}}}
	// The fill reaches the candle bodies, so only a line over the candles
	// has it.
	if canvas.Place == chart.PriceScale {
		shapes = append(shapes, chart.Shape{ID: "fill", Place: canvas.Place, Area: &chart.Area{Points: fill}})
	}
	return shapes
}
