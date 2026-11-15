package main

import (
	"slices"

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

// chartPalette colors the chart lines of the configured indicators in turn.
// Entries are client theme color tokens.
var chartPalette = []string{"yellow.5", "orange.6", "violet.5", "blue.5", "teal.5", "pink.5", "lime.5", "cyan.5"}

// chartTones colors the chart lines of outputs with a fixed meaning, with
// the client theme color tokens of rising and falling candles.
var chartTones = map[indicator.Tone]string{indicator.ToneBullish: "green.6", indicator.ToneBearish: "red.6"}
