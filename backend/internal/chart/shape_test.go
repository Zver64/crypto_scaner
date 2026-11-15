package chart

import (
	"reflect"
	"testing"
	"time"

	"crypto-scanner/internal/market"
)

// A tail keeps what candles opened from its start calculated, wherever it
// is drawn: a pivot confirmed by the forming candle is drawn on an earlier
// one.
func TestTailKeepsWhatItsCandlesCalculated(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	hour := func(hours int) time.Time { return start.Add(time.Duration(hours) * time.Hour) }
	page := Page{
		Candles: []market.Candle{{OpenTime: hour(0)}, {OpenTime: hour(1)}, {OpenTime: hour(2)}},
		Shapes: []Shape{
			{ID: "line", Line: &Line{Points: []LinePoint{{Time: hour(1), Value: 1}, {Time: hour(2), Value: 2}}}},
			{ID: "area", Area: &Area{Points: []AreaPoint{{Time: hour(1)}, {Time: hour(2)}}}},
			{ID: "markers", Markers: &Markers{Points: []Marker{{Time: hour(0), Calculated: hour(1)}, {Time: hour(0), Calculated: hour(2)}}}},
			{ID: "segments", Segments: &Segments{Items: []Segment{{Calculated: hour(1)}, {Calculated: hour(2), From: SegmentPoint{Time: hour(0)}}}}},
		},
	}
	tail := page.since(hour(2))
	want := []Shape{
		{ID: "line", Line: &Line{Points: []LinePoint{{Time: hour(2), Value: 2}}}},
		{ID: "area", Area: &Area{Points: []AreaPoint{{Time: hour(2)}}}},
		{ID: "markers", Markers: &Markers{Points: []Marker{{Time: hour(0), Calculated: hour(2)}}}},
		{ID: "segments", Segments: &Segments{Items: []Segment{{Calculated: hour(2), From: SegmentPoint{Time: hour(0)}}}}},
	}
	if len(tail.Candles) != 1 || !tail.Candles[0].OpenTime.Equal(hour(2)) || !reflect.DeepEqual(tail.Shapes, want) {
		t.Fatalf("tail = %+v, want the last candle and %+v", tail, want)
	}
	if len(page.Shapes[0].Line.Points) != 2 {
		t.Fatal("the tail changed the page")
	}
}
