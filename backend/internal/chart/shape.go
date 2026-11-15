package chart

import (
	"time"

	"crypto-scanner/internal/market"
)

// Place is where a shape lies: over the candles on the price scale, over the
// volume bars on their scale, or in an indicator pane.
type Place struct {
	Placement Placement
	// Pane is the key of the pane of PlacementPane and empty otherwise.
	Pane string
}

// PriceScale is the place over the candles, on their price scale.
var PriceScale = Place{Placement: PlacementOverlay}

// Shape is one figure a chart draws. Exactly one of Line, Area, Markers, and
// Segments is set.
type Shape struct {
	// ID names the shape within its page; the frames of the same indicators
	// have the same shapes under the same ids, so a tail frame updates them.
	ID       string
	Place    Place
	Line     *Line
	Area     *Area
	Markers  *Markers
	Segments *Segments
}

// Line joins its points in order. Every point is calculated by its own
// candle.
type Line struct {
	Title string
	// Color is a client theme color token such as "yellow.5", so the client
	// keeps control of the actual palette.
	Color  string
	Points []LinePoint
}

// LinePoint is the value of a line at a candle.
type LinePoint struct {
	Time  time.Time
	Value float64
	// Color is the color of the point and of the join to the next one;
	// empty takes the line color.
	Color string
	// Break leaves out the join from the point before, such as where the
	// SUPERTREND line jumps to the other side of the candles.
	Break bool
}

// Area fills between two edges at each candle. Neighbouring points of the
// same color are joined; a missing point or a change of color breaks it.
// Every point is calculated by its own candle.
type Area struct {
	Points []AreaPoint
}

// AreaPoint is the extent of an area at a candle: from Value to Base.
type AreaPoint struct {
	Time  time.Time
	Value float64
	Base  float64
	// Color is a client theme color token; clients make the fill
	// translucent.
	Color string
}

// MarkerPosition is the side of a candle, or of a line, a marker is drawn on.
type MarkerPosition string

const (
	MarkerAbove MarkerPosition = "above"
	MarkerBelow MarkerPosition = "below"
)

// Markers draws a point above or below candles, or above or below the values
// of a line.
type Markers struct {
	Title    string
	Color    string
	Position MarkerPosition
	Points   []Marker
	// OwnSegments reports markers their drawer joins with segments of its
	// own, which Connectors leaves alone. Clients do not get it.
	OwnSegments bool
}

// Marker is one marker, drawn on the candle opened at Time.
type Marker struct {
	Time time.Time
	// Calculated is the open time of the candle that calculated the marker,
	// such as the candle confirming a pivot drawn on an earlier candle.
	Calculated time.Time
	// Value is the value of the marker, which legends show.
	Value float64
	// LineValue is the value of the line the marker sits on at Time; nil
	// draws the marker on the candle.
	LineValue *float64
}

// Segments draws straight lines between two points each.
type Segments struct {
	Color string
	Items []Segment
}

// Segment is one straight line.
type Segment struct {
	// Calculated is the open time of the candle that calculated the segment.
	Calculated time.Time
	From       SegmentPoint
	To         SegmentPoint
}

// SegmentPoint is an end of a segment: a value on a candle.
type SegmentPoint struct {
	Time  time.Time
	Value float64
}

// since returns the shape with only what candles opened at or after from
// calculated.
func (shape Shape) since(from time.Time) Shape {
	after := func(open time.Time) bool { return !open.Before(from) }
	switch {
	case shape.Line != nil:
		line := *shape.Line
		line.Points = keep(line.Points, func(point LinePoint) bool { return after(point.Time) })
		shape.Line = &line
	case shape.Area != nil:
		area := *shape.Area
		area.Points = keep(area.Points, func(point AreaPoint) bool { return after(point.Time) })
		shape.Area = &area
	case shape.Markers != nil:
		markers := *shape.Markers
		markers.Points = keep(markers.Points, func(marker Marker) bool { return after(marker.Calculated) })
		shape.Markers = &markers
	case shape.Segments != nil:
		segments := *shape.Segments
		segments.Items = keep(segments.Items, func(segment Segment) bool { return after(segment.Calculated) })
		shape.Segments = &segments
	}
	return shape
}

// keep returns the items of items for which wanted holds, in a new slice.
func keep[T any](items []T, wanted func(T) bool) []T {
	kept := make([]T, 0)
	for _, item := range items {
		if wanted(item) {
			kept = append(kept, item)
		}
	}
	return kept
}

// since returns the tail of the page from from: only the candles opened, and
// what of its shapes candles opened, at or after from.
func (page Page) since(from time.Time) Page {
	page.Candles = keep(page.Candles, func(candle market.Candle) bool { return !candle.OpenTime.Before(from) })
	shapes := make([]Shape, len(page.Shapes))
	for i, shape := range page.Shapes {
		shapes[i] = shape.since(from)
	}
	page.Shapes = shapes
	return page
}
