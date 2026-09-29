package main

import (
	"crypto-scanner/internal/indicator"
	indicatortalib "crypto-scanner/internal/indicator/talib"
)

// indicatorModules are the calculation algorithms available to every consumer:
// every TA-Lib function generated in the talib package. The administrator
// chooses which of them the scanner calculates, charts draw, and tables show.
func indicatorModules() []indicator.Implementation {
	return indicatortalib.New()
}

// chartPalette colors the chart lines of the configured indicators in turn.
// Entries are client theme color tokens.
var chartPalette = []string{"yellow.5", "orange.6", "violet.5", "blue.5", "teal.5", "pink.5", "lime.5", "cyan.5"}
