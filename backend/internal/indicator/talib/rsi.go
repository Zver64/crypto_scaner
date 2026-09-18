package talib

import (
	"crypto-scanner/internal/indicator"

	gotalib "github.com/markcheno/go-talib"
)

const (
	// RSIType is the stable registry identifier for the relative strength index.
	RSIType indicator.Type = "rsi"
	// DefaultRSIPeriod is the period used by the chart experience.
	DefaultRSIPeriod = 14
)

// NewRSI describes RSI through the reusable single-input, single-period TA-Lib
// adapter. go-talib owns the mathematical implementation.
func NewRSI() indicator.Implementation {
	return newSingleInputPeriod(singleInputPeriodSpec{
		indicatorType: RSIType,
		inputName:     "close",
		outputName:    "rsi",
		minimumPeriod: 2,
		maximumPeriod: 500,
		offset: func(period int) int {
			return period
		},
		lookback: func(period int) int {
			return period * 10
		},
		calculate: gotalib.Rsi,
	})
}
