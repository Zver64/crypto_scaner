package strategy

import (
	"crypto-scanner/internal/market"
)

// baselines fills the baselines from the candles of the result's interval,
// of which every candle from first on is evaluated (none when first is
// negative).
func (result *Backtest) baselines(candles []market.Candle, first int) {
	if first < 0 || first+1 >= len(candles) || !(candles[first+1].Open > 0) {
		return
	}
	last := candles[len(candles)-1].Close
	result.BuyAndHold = new(netReturn(DirectionLong, candles[first+1].Open, last))
	var quantity float64
	buys := 0
	for _, candle := range candles[first+1:] {
		if candle.Open > 0 {
			quantity += 1 / candle.Open
			buys++
		}
	}
	result.DCA = new(netReturn(DirectionLong, float64(buys)/quantity, last))
}

// netReturn is the return of a position in direction opened at entry and
// closed at exit, net of BacktestFee on both sides: a long one buys at entry
// and sells at exit, a short one the other way around. A short one loses at
// most everything, as if liquidated, however far the price rose.
func netReturn(direction Direction, entry, exit float64) float64 {
	if direction == DirectionShort {
		return max(direction.gross(entry, exit)-BacktestFee*(1+exit/entry), -1)
	}
	return exit/entry*(1-BacktestFee)*(1-BacktestFee) - 1
}

// tradeStats describes closed trades by their returns, candles held, and exit
// reasons.
func tradeStats(returns, bars []float64, reasons []ExitReason) TradeStats {
	stats := TradeStats{Count: len(returns)}
	for _, reason := range reasons {
		switch reason {
		case ExitTakeProfit:
			stats.TakeProfits++
		case ExitStopLoss:
			stats.StopLosses++
		case ExitRuleSignal:
			stats.ExitRules++
		}
	}
	if len(returns) == 0 {
		return stats
	}
	var held float64
	for _, value := range bars {
		held += value
	}
	stats.AverageBars = new(held / float64(len(bars)))
	var sum, profit, loss float64
	wins, losses := 0, 0
	for _, value := range returns {
		sum += value
		switch {
		case value > 0:
			wins++
			profit += value
		case value < 0:
			losses++
			loss -= value
		}
	}
	stats.WinRate, stats.AverageTrade = new(float64(wins)/float64(len(returns))), new(sum/float64(len(returns)))
	if wins > 0 {
		stats.AverageWin = new(profit / float64(wins))
	}
	if losses > 0 {
		stats.AverageLoss, stats.ProfitFactor = new(-loss/float64(losses)), new(profit/loss)
	}
	return stats
}
