package main

import (
	"slices"

	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/chart/draw"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/indicator/candle"
	"crypto-scanner/internal/indicator/pivot"
	indicatortalib "crypto-scanner/internal/indicator/talib"
)

// indicatorModules are the calculation algorithms available to every consumer:
// every TA-Lib function generated in the talib package, which the
// administrator chooses for the scanner, charts, and tables, the pivots and
// RSI divergences TA-Lib lacks, and the internal candle fields strategies
// read.
func indicatorModules() []indicator.Implementation {
	modules := indicatortalib.New()
	rsi := modules[slices.IndexFunc(modules, func(module indicator.Implementation) bool {
		return module.Describe().Type == indicatortalib.RSIType
	})]
	return append(modules, pivot.New(), pivot.NewDivergence(rsi), candle.New())
}

// chartDrawers draws the indicators on charts: SUPERTREND, pivots, and RSI
// divergences with drawers of their own, every other indicator with the
// default drawer. Shapes take the palette colors in turn; what rises and
// falls takes the colors of rising and falling candles. Colors are client
// theme color tokens.
func chartDrawers(registry *indicator.Registry) (*chart.Drawers, error) {
	palette := chart.Palette{
		Colors:  []string{"yellow.5", "orange.6", "violet.5", "blue.5", "teal.5", "pink.5", "lime.5", "cyan.5"},
		Bullish: "green.6",
		Bearish: "red.6",
	}
	return chart.NewDrawers(registry, palette, draw.Default{}, map[indicator.Type]chart.Drawer{
		indicatortalib.SupertrendType: draw.Supertrend{},
		pivot.Type:                    draw.Pivot{},
		pivot.DivergenceType:          draw.Divergence{},
	})
}
