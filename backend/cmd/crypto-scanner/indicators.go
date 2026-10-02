package main

import (
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/indicator/candle"
	indicatortalib "crypto-scanner/internal/indicator/talib"
)

// indicatorModules are the calculation algorithms available to every consumer:
// every TA-Lib function generated in the talib package, which the
// administrator chooses for the scanner, charts, and tables, and the internal
// candle fields strategies read.
func indicatorModules() []indicator.Implementation {
	return append(indicatortalib.New(), candle.New())
}

// chartPalette colors the chart lines of the configured indicators in turn.
// Entries are client theme color tokens.
var chartPalette = []string{"yellow.5", "orange.6", "violet.5", "blue.5", "teal.5", "pink.5", "lime.5", "cyan.5"}
