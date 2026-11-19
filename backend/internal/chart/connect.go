package chart

import (
	"time"

	"crypto-scanner/internal/market"
)

// Connectors joins markers to the markers before them, such as the markers
// a signal fired on: for every markers shape without segments of its own, the
// marker each candle opened at one of times calculated is joined to the
// previous marker of the shape, from value to value where the markers lie.
// Markers on a line are also joined on the candles, from low to low below
// them or from high to high above them. The shapes are named after the
// markers shape, with "-connected" and "-connected-candles", and only those
// with segments are returned.
func Connectors(shapes []Shape, candles []market.Candle, times []time.Time) []Shape {
	byOpen := make(map[time.Time]market.Candle, len(candles))
	for _, candle := range candles {
		byOpen[candle.OpenTime.UTC()] = candle
	}
	connected := make(map[time.Time]bool, len(times))
	for _, at := range times {
		connected[at.UTC()] = true
	}
	result := make([]Shape, 0)
	for _, shape := range shapes {
		markers := shape.Markers
		if markers == nil || markers.OwnSegments {
			continue
		}
		onShape, onCandles := make([]Segment, 0), make([]Segment, 0)
		edge := func(candle market.Candle) float64 {
			if markers.Position == MarkerBelow {
				return candle.Low
			}
			return candle.High
		}
		for i := 1; i < len(markers.Points); i++ {
			from, to := markers.Points[i-1], markers.Points[i]
			if !connected[to.Calculated.UTC()] {
				continue
			}
			onShape = append(onShape, Segment{Calculated: to.Calculated, From: SegmentPoint{Time: from.Time, Value: from.Value}, To: SegmentPoint{Time: to.Time, Value: to.Value}})
			start, started := byOpen[from.Time.UTC()]
			end, ended := byOpen[to.Time.UTC()]
			if to.LineValue != nil && started && ended {
				onCandles = append(onCandles, Segment{Calculated: to.Calculated, From: SegmentPoint{Time: from.Time, Value: edge(start)}, To: SegmentPoint{Time: to.Time, Value: edge(end)}})
			}
		}
		if len(onShape) > 0 {
			result = append(result, Shape{ID: shape.ID + "-connected", Indicator: shape.Indicator, Place: shape.Place, Segments: &Segments{Color: markers.Color, Items: onShape}})
		}
		if len(onCandles) > 0 {
			result = append(result, Shape{ID: shape.ID + "-connected-candles", Indicator: shape.Indicator, Place: PriceScale, Segments: &Segments{Color: markers.Color, Items: onCandles}})
		}
	}
	return result
}
