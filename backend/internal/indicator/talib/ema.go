package talib

import (
	"crypto-scanner/internal/indicator"

	gotalib "github.com/markcheno/go-talib"
)

// EMAType is the stable registry identifier for the exponential moving average.
const EMAType indicator.Type = "ema"

// NewEMA describes EMA through the reusable single-input, single-period TA-Lib
// adapter. go-talib seeds it with an SMA and zero-fills the first period-1
// values.
func NewEMA() indicator.Implementation {
	return newSingleInputPeriod(singleInputPeriodSpec{
		indicatorType: EMAType,
		inputName:     "close",
		outputName:    "ema",
		minimumPeriod: 2,
		maximumPeriod: 500,
		offset: func(period int) int {
			return period - 1
		},
		// The SMA seed's influence fades exponentially; ten periods of history
		// make it negligible, matching RSI.
		lookback: func(period int) int {
			return period * 10
		},
		calculate: gotalib.Ema,
	})
}
