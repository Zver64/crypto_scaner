package chart

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
)

// Placement tells the client where to draw an indicator.
type Placement string

const (
	// PlacementOverlay draws the indicator over the candles, on their price scale.
	PlacementOverlay Placement = "overlay"
	// PlacementPane draws the indicator in a pane below the candles.
	PlacementPane Placement = "pane"
	// PlacementVolume draws the indicator over the volume bars, on their scale.
	PlacementVolume Placement = "volume"
)

// LineStyle tells the client how to draw an indicator line.
type LineStyle string

const (
	// LineStyleLine joins the values with a line.
	LineStyleLine LineStyle = "line"
	// LineStyleMarkerAbove and LineStyleMarkerBelow draw each value as a point
	// above or below a candle, or the line they are anchored to. Only
	// overlays and anchored lines have them.
	LineStyleMarkerAbove LineStyle = "marker_above"
	LineStyleMarkerBelow LineStyle = "marker_below"
)

// LineAnchor names the drawn line markers sit on: an output of another
// indicator of the catalog. The zero anchor is the candles.
type LineAnchor struct {
	// IndicatorID is the catalog id of that indicator.
	IndicatorID string
	Output      string
}

// IndicatorLine maps one calculated output series to a drawn line.
type IndicatorLine struct {
	Output string
	Title  string
	// Color is a client theme color token such as "yellow.5", so the client
	// keeps control of the actual palette.
	Color string
	Style LineStyle
	// Shift is the number of candles a marker is drawn before the candle
	// holding its value. The values themselves are not shifted.
	Shift int
	// Anchor is the line the markers of an indicator of another indicator
	// sit on, in the pane of that line.
	Anchor LineAnchor
}

// IndicatorLevel is a labeled horizontal reference line in a pane.
type IndicatorLevel struct {
	Value float64
	Title string
}

// IndicatorScale describes the value axis of an indicator pane.
type IndicatorScale struct {
	Min    *float64
	Max    *float64
	Levels []IndicatorLevel
}

// CatalogIndicator is one indicator the chart calculates and how to draw it.
// It has no lines when nothing of it can be drawn where it is placed, such
// as markers neither over the candles nor anchored to a line.
type CatalogIndicator struct {
	ID        string
	Selection indicator.Selection
	Placement Placement
	Lines     []IndicatorLine
	// Scale is required for panes and must be nil otherwise: overlays share the
	// candle price scale and volume indicators the volume scale.
	Scale *IndicatorScale
	// Pane is required for panes and must be empty otherwise: pane indicators
	// with the same key share one pane.
	Pane string
}

// CatalogSource supplies the indicators every chart of an interval shows.
type CatalogSource interface {
	ChartCatalog(market.CandleInterval) []CatalogIndicator
}

// ValidateIndicator checks that a catalog indicator can be calculated and drawn.
func ValidateIndicator(registry *indicator.Registry, item CatalogIndicator) error {
	if strings.TrimSpace(item.ID) == "" {
		return errors.New("catalog indicator id is empty")
	}
	return validateCatalogIndicator(registry, item)
}

func validateCatalogIndicator(registry *indicator.Registry, item CatalogIndicator) error {
	lookback, err := registry.Lookback(item.Selection.Type, item.Selection.Parameters)
	if err != nil {
		return err
	}
	if lookback > maxIndicatorLookback {
		return fmt.Errorf("lookback exceeds %d", maxIndicatorLookback)
	}
	outputs, err := registry.Outputs(item.Selection.Type)
	if err != nil {
		return err
	}
	drawn := map[string]bool{}
	for _, line := range item.Lines {
		if drawn[line.Output] {
			return fmt.Errorf("line output %q is drawn twice", line.Output)
		}
		drawn[line.Output] = true
		if !slices.Contains(outputs, line.Output) {
			return fmt.Errorf("line output %q is not produced by %q", line.Output, item.Selection.Type)
		}
		if strings.TrimSpace(line.Title) == "" || strings.TrimSpace(line.Color) == "" {
			return fmt.Errorf("line %q needs a title and color", line.Output)
		}
		switch line.Style {
		case LineStyleLine:
			if line.Anchor != (LineAnchor{}) {
				return fmt.Errorf("line %q has an anchor, which only markers have", line.Output)
			}
		case LineStyleMarkerAbove, LineStyleMarkerBelow:
			switch {
			case line.Anchor == LineAnchor{}:
				if item.Placement != PlacementOverlay {
					return fmt.Errorf("line %q is drawn as markers, which only indicators over the candles or anchored to a line have", line.Output)
				}
			case line.Anchor.IndicatorID == "" || line.Anchor.Output == "":
				return fmt.Errorf("line %q is anchored to an unnamed line", line.Output)
			}
		default:
			return fmt.Errorf("line %q has unknown style %q", line.Output, line.Style)
		}
		if line.Shift < 0 {
			return fmt.Errorf("line %q has a negative shift", line.Output)
		}
	}
	switch item.Placement {
	case PlacementOverlay, PlacementVolume:
		if item.Scale != nil || item.Pane != "" {
			return errors.New("overlays share the candle pane and a scale and must not define them")
		}
	case PlacementPane:
		if item.Scale == nil || strings.TrimSpace(item.Pane) == "" {
			return errors.New("panes require a scale and a pane key")
		}
		if item.Scale.Min != nil && item.Scale.Max != nil && *item.Scale.Min >= *item.Scale.Max {
			return errors.New("scale min must be below max")
		}
		levels := map[float64]bool{}
		for _, level := range item.Scale.Levels {
			if levels[level.Value] {
				return fmt.Errorf("level %v is duplicated", level.Value)
			}
			levels[level.Value] = true
		}
	default:
		return fmt.Errorf("unknown placement %q", item.Placement)
	}
	return nil
}
