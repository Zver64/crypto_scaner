package chart

import (
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
)

const (
	// settleFactor scales the lookback into the warm-up of every indicator.
	// The lookback is only the minimum: recursive indicators (EMA, RSI, ADX,
	// MACD, and so on) start from a seed such as an SMA and need many more
	// candles before the seed stops mattering.
	settleFactor = 10
	// maxWindow bounds the warm-up so that half of the synchronized history
	// remains for scroll-back and earlier points.
	maxWindow = market.SyncDepth / 2
)

// Window is the number of closed candles, ending at a candle, over which the
// value of selection at that candle is calculated at least. Charts, the
// closed indicator tracker, and backtests all give every point this warm-up,
// so a value does not depend on how many points are read or on the other
// selected indicators: a longer history only changes a settled value by the
// remainder of the seed.
func Window(registry *indicator.Registry, selection indicator.Selection) (int, error) {
	lookback, err := registry.Lookback(selection.Type, selection.Parameters)
	if err != nil {
		return 0, err
	}
	return max(lookback+1, min(maxWindow, (lookback+1)*settleFactor)), nil
}
