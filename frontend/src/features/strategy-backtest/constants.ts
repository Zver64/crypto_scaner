// Candles of the strategy's interval a backtest trade holds; the backend
// accepts 1–1000 and picks a default per interval when none is given.
export const backtestHold = { max: 1000, min: 1 };

// Meanings of the backtest baselines, shown on hover or tap.
export const everyCandleHint =
	"The same hold entered on every candle, without the strategy; the strategy should beat it.";
export const buyAndHoldHint =
	"Bought at the start of the period and sold at its end.";
