package main

import (
	"fmt"

	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/indicator"
	indicatortalib "crypto-scanner/internal/indicator/talib"
)

// indicatorModules are the calculation algorithms available to every consumer:
// every TA-Lib function generated in the talib package. A new chart line only
// needs a catalog entry below.
func indicatorModules() []indicator.Implementation {
	return indicatortalib.New()
}

// chartIndicatorCatalog lists what every chart shows and how clients draw it.
// Clients read it from the API, so adding an entry needs no frontend change.
func chartIndicatorCatalog() []chart.CatalogIndicator {
	return []chart.CatalogIndicator{
		emaOverlay(20, "yellow.5"),
		emaOverlay(50, "orange.6"),
		emaOverlay(100, "violet.5"),
		rsiPane(indicatortalib.DefaultRSIPeriod, "blue.5"),
	}
}

func emaOverlay(period int, color string) chart.CatalogIndicator {
	title := fmt.Sprintf("EMA %d", period)
	return chart.CatalogIndicator{
		ID:        fmt.Sprintf("ema-%d", period),
		Selection: indicator.Selection{Type: indicatortalib.EMAType, Parameters: indicator.Parameters{"period": period}},
		Placement: chart.PlacementOverlay,
		Lines:     []chart.IndicatorLine{{Output: "ema", Title: title, Color: color}},
	}
}

func rsiPane(period int, color string) chart.CatalogIndicator {
	minimum, maximum := 0.0, 100.0
	return chart.CatalogIndicator{
		ID:        fmt.Sprintf("rsi-%d", period),
		Selection: indicator.Selection{Type: indicatortalib.RSIType, Parameters: indicator.Parameters{"period": period}},
		Placement: chart.PlacementPane,
		Lines:     []chart.IndicatorLine{{Output: "rsi", Title: fmt.Sprintf("RSI %d", period), Color: color}},
		Scale: &chart.IndicatorScale{
			Min:       &minimum,
			Max:       &maximum,
			Levels:    []chart.IndicatorLevel{{Value: 30, Title: "RSI 30"}, {Value: 70, Title: "RSI 70"}},
			Precision: 1,
		},
	}
}
