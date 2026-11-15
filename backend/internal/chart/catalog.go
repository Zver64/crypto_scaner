package chart

import (
	"errors"
	"fmt"
	"strings"

	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
)

// Placement is where an indicator is drawn.
type Placement string

const (
	// PlacementOverlay draws the indicator over the candles, on their price scale.
	PlacementOverlay Placement = "overlay"
	// PlacementPane draws the indicator in a pane below the candles.
	PlacementPane Placement = "pane"
	// PlacementVolume draws the indicator over the volume bars, on their scale.
	PlacementVolume Placement = "volume"
)

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

// CatalogIndicator is one indicator the charts of an interval draw.
type CatalogIndicator struct {
	ID        string
	Selection indicator.Selection
	// Scale is the configured value axis of the pane the indicator is drawn
	// in; indicators drawn elsewhere have none.
	Scale IndicatorScale
}

// CatalogSource supplies the indicators every chart of an interval shows.
type CatalogSource interface {
	ChartCatalog(market.CandleInterval) []CatalogIndicator
}

// ValidateIndicator checks that a catalog indicator can be calculated and
// that its scale is consistent.
func ValidateIndicator(registry *indicator.Registry, item CatalogIndicator) error {
	if strings.TrimSpace(item.ID) == "" {
		return errors.New("catalog indicator id is empty")
	}
	lookback, err := registry.Lookback(item.Selection.Type, item.Selection.Parameters)
	if err != nil {
		return err
	}
	if lookback > maxIndicatorLookback {
		return fmt.Errorf("lookback exceeds %d", maxIndicatorLookback)
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
	return nil
}
