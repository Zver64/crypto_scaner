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
	// PlacementPane draws the indicator in its own pane below the candles.
	PlacementPane Placement = "pane"
)

// IndicatorLine maps one calculated output series to a drawn line.
type IndicatorLine struct {
	Output string
	Title  string
	// Color is a client theme color token such as "yellow.5", so the client
	// keeps control of the actual palette.
	Color string
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
type CatalogIndicator struct {
	ID        string
	Selection indicator.Selection
	Placement Placement
	Lines     []IndicatorLine
	// Scale is required for panes and must be nil for overlays, which share the
	// candle price scale.
	Scale *IndicatorScale
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
	if len(item.Lines) == 0 {
		return errors.New("at least one line is required")
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
	}
	switch item.Placement {
	case PlacementOverlay:
		if item.Scale != nil {
			return errors.New("overlays share the candle scale and must not define one")
		}
	case PlacementPane:
		if item.Scale == nil {
			return errors.New("panes require a scale")
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
