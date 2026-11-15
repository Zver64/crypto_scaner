package chart_test

import (
	"reflect"
	"testing"
	"time"

	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/market"
)

func TestConnectorsJoinMarkersToThePreviousOne(t *testing.T) {
	start := time.Date(2026, 8, 26, 23, 0, 0, 0, time.UTC)
	hour := func(hours int) time.Time { return start.Add(time.Duration(hours) * time.Hour) }
	// Candle i closes at 10 + i, its low one below and its high one above.
	candles := make([]market.Candle, 7)
	for i := range candles {
		close := 10 + float64(i)
		candles[i] = market.Candle{OpenTime: hour(i), Open: close - 0.5, High: close + 1, Low: close - 1, Close: close}
	}
	pane := chart.Place{Placement: chart.PlacementPane, Pane: "rsi"}
	// Values calculated on candles 1, 2, 4, and 6, drawn one candle earlier.
	markers := func(position chart.MarkerPosition, onLine bool) chart.Shape {
		shape := chart.Shape{ID: "pivot/low", Place: chart.PriceScale, Markers: &chart.Markers{Color: "yellow.5", Position: position}}
		for _, point := range []struct {
			calculated int
			value      float64
		}{{1, 20}, {2, 30}, {4, 40}, {6, 35}} {
			marker := chart.Marker{Time: hour(point.calculated - 1), Calculated: hour(point.calculated), Value: point.value}
			if onLine {
				marker.LineValue = new(50.0)
			}
			shape.Markers.Points = append(shape.Markers.Points, marker)
		}
		if onLine {
			shape.Place = pane
		}
		return shape
	}
	segment := func(calculated, from int, fromValue float64, to int, toValue float64) chart.Segment {
		return chart.Segment{Calculated: hour(calculated), From: chart.SegmentPoint{Time: hour(from), Value: fromValue}, To: chart.SegmentPoint{Time: hour(to), Value: toValue}}
	}
	own := markers(chart.MarkerBelow, true)
	own.Markers.OwnSegments = true
	tests := []struct {
		name   string
		shapes []chart.Shape
		times  []time.Time
		want   []chart.Shape
	}{
		{
			name: "on the candles", shapes: []chart.Shape{markers(chart.MarkerBelow, false)}, times: []time.Time{hour(6)},
			want: []chart.Shape{{ID: "pivot/low-connected", Place: chart.PriceScale, Segments: &chart.Segments{Color: "yellow.5", Items: []chart.Segment{segment(6, 3, 40, 5, 35)}}}},
		},
		{
			name: "on a line and between the same candles", shapes: []chart.Shape{markers(chart.MarkerBelow, true)}, times: []time.Time{hour(4), hour(6)},
			want: []chart.Shape{
				{ID: "pivot/low-connected", Place: pane, Segments: &chart.Segments{Color: "yellow.5", Items: []chart.Segment{segment(4, 1, 30, 3, 40), segment(6, 3, 40, 5, 35)}}},
				{ID: "pivot/low-connected-candles", Place: chart.PriceScale, Segments: &chart.Segments{Color: "yellow.5", Items: []chart.Segment{segment(4, 1, 10, 3, 12), segment(6, 3, 12, 5, 14)}}},
			},
		},
		{
			name: "above a line from high to high", shapes: []chart.Shape{markers(chart.MarkerAbove, true)}, times: []time.Time{hour(6)},
			want: []chart.Shape{
				{ID: "pivot/low-connected", Place: pane, Segments: &chart.Segments{Color: "yellow.5", Items: []chart.Segment{segment(6, 3, 40, 5, 35)}}},
				{ID: "pivot/low-connected-candles", Place: chart.PriceScale, Segments: &chart.Segments{Color: "yellow.5", Items: []chart.Segment{segment(6, 3, 14, 5, 16)}}},
			},
		},
		{
			name: "separate segments", shapes: []chart.Shape{markers(chart.MarkerBelow, false)}, times: []time.Time{hour(2), hour(6)},
			want: []chart.Shape{{ID: "pivot/low-connected", Place: chart.PriceScale, Segments: &chart.Segments{Color: "yellow.5", Items: []chart.Segment{segment(2, 0, 20, 1, 30), segment(6, 3, 40, 5, 35)}}}},
		},
		// The first marker has none before it, and candle 5 calculated none.
		{name: "first marker or candle without one", shapes: []chart.Shape{markers(chart.MarkerBelow, false)}, times: []time.Time{hour(1), hour(5)}, want: []chart.Shape{}},
		{name: "no times", shapes: []chart.Shape{markers(chart.MarkerBelow, false)}, want: []chart.Shape{}},
		{name: "markers with segments of their own", shapes: []chart.Shape{own}, times: []time.Time{hour(6)}, want: []chart.Shape{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := chart.Connectors(test.shapes, candles, test.times); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("Connectors() = %+v, want %+v", got, test.want)
			}
		})
	}
}
