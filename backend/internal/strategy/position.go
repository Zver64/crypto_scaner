package strategy

import "time"

// TradeState is the trading state of a strategy on one instrument after the
// last candle it processed. Every buy spends one quote unit at the open of
// the candle after its signal; Filled of the Buys have filled, buying
// Quantity coins in total.
type TradeState struct {
	// OpenTime is the open time of the last processed candle.
	OpenTime time.Time
	// Entry is whether the entry expression was true there; a signal is its
	// turn from false to true.
	Entry bool
	// Buys counts the buys of the open trade, none without one.
	Buys, Filled int
	Quantity     float64
	// OpenedAt is the open time of the candle the first buy filled at; zero
	// until it fills.
	OpenedAt time.Time
	// TakeProfit and StopLoss are the prices the open trade sells at, fixed
	// at its entry signal; 0 without a trade or without that exit.
	TakeProfit, StopLoss float64
}

// EntryPrice is the average price of the filled buys, the quote spent over
// the coins bought.
func (state TradeState) EntryPrice() float64 {
	return float64(state.Filled) / state.Quantity
}

// TradeEventKind tells what a processed candle did.
type TradeEventKind int

const (
	// TradeBuy is an entry signal that buys.
	TradeBuy TradeEventKind = iota + 1
	// TradeSell is an exit that sells every buy of the trade.
	TradeSell
	// TradeSkip is an entry signal that buys nothing, since the strategy
	// exits and holds a trade, or its take profit or stop loss is not on
	// its side of the close.
	TradeSkip
)

// ExitReason tells what sold a trade.
type ExitReason string

const (
	ExitTakeProfit ExitReason = "take_profit"
	ExitStopLoss   ExitReason = "stop_loss"
	ExitRuleSignal ExitReason = "exit"
)

// TradeEvent is what a processed candle did. A buy names its number in the
// trade and the trade after it; a sell the trade it closes, at the state
// before the candle, and what sold it.
type TradeEvent struct {
	Kind  TradeEventKind
	Buy   int
	Trade TradeState
	// Close is the close of the candle. Reason tells what sold a sell;
	// Price is the price a take profit or stop loss filled at on this
	// candle, while an exit rule sells at the next open, unknown here.
	// Return is the fractional return of a sell at Price, or at the close
	// for an exit rule.
	Close, Price, Return float64
	Reason               ExitReason
}

// TradeCandle is the processed candle of the strategy's interval and the
// results of its expressions at the close. The exit result comes from
// evaluating the exit expression over the position variables exit receives;
// it is only asked for while a trade is open. Levels returns the take profit
// and stop loss prices at the close, 0 for those the strategy lacks; it is
// only asked for on a signal that opens a trade.
type TradeCandle struct {
	OpenTime               time.Time
	Open, High, Low, Close float64
	Entry                  bool
	EntryKnown             bool
	Exit                   func(positions map[string]float64) (result, known bool)
	Levels                 func() (takeProfit, stopLoss float64, known bool)
}

// step processes candle after state. Buys pending since earlier signals fill
// at its open, and the first fill starts the trade. A filled trade then sells
// at its stop loss or take profit when the candle reaches them: at the open
// when it opens past one, otherwise at the price itself, the stop loss first
// when the candle reaches both. Otherwise, while a trade is open, a true exit
// rule sells every buy at the next open. A sell counts the entry as false on
// its candle, so that the next candle can signal again. Otherwise an entry
// signal opens a trade, fixing its take profit and stop loss when the close
// lies between them and skipping otherwise, or, for a strategy that never
// sells, adds a buy; a strategy that exits skips signals while it holds a
// trade. A candle waits, changing nothing and processed false, until every
// rule it needs is known: the exit rule while a trade is open, and the entry
// and the levels of a trade it opens unless the trade sells. Nothing changes
// either for a candle not after the last processed one.
func (entry Entry) step(state TradeState, candle TradeCandle) (next TradeState, events []TradeEvent, processed bool) {
	if !candle.OpenTime.After(state.OpenTime) || !(candle.Open > 0) || !(candle.High > 0) || !(candle.Low > 0) || !(candle.Close > 0) {
		return state, nil, false
	}
	next = state
	if pending := next.Buys - next.Filled; pending > 0 {
		if next.Filled == 0 {
			next.OpenedAt = candle.OpenTime
		}
		next.Quantity += float64(pending) / candle.Open
		next.Filled = next.Buys
	}
	if next.Buys > 0 {
		if price, reason, ok := next.levelExit(candle); ok {
			return TradeState{OpenTime: candle.OpenTime}, []TradeEvent{{
				Kind: TradeSell, Trade: next, Close: candle.Close, Price: price, Return: price/next.EntryPrice() - 1, Reason: reason,
			}}, true
		}
	}
	if next.Buys > 0 && entry.Exit != nil {
		exit, known := candle.Exit(entry.positions(next, candle.OpenTime, candle.Close))
		if !known {
			return state, nil, false
		}
		if exit {
			return TradeState{OpenTime: candle.OpenTime}, []TradeEvent{{
				Kind: TradeSell, Trade: next, Close: candle.Close, Return: candle.Close/next.EntryPrice() - 1, Reason: ExitRuleSignal,
			}}, true
		}
	}
	if !candle.EntryKnown {
		return state, nil, false
	}
	next.OpenTime = candle.OpenTime
	signal := candle.Entry && !next.Entry
	next.Entry = candle.Entry
	if !signal {
		return next, nil, true
	}
	switch {
	case next.Buys == 0:
		takeProfit, stopLoss, known := candle.Levels()
		if !known {
			return state, nil, false
		}
		if entry.TakeProfit != nil && !(takeProfit > candle.Close) || entry.StopLoss != nil && !(stopLoss > 0 && stopLoss < candle.Close) {
			return next, []TradeEvent{{Kind: TradeSkip, Close: candle.Close}}, true
		}
		next.Buys, next.TakeProfit, next.StopLoss = 1, takeProfit, stopLoss
	case !entry.Exits():
		next.Buys++
	default:
		return next, []TradeEvent{{Kind: TradeSkip, Close: candle.Close}}, true
	}
	return next, []TradeEvent{{Kind: TradeBuy, Buy: next.Buys, Trade: next, Close: candle.Close}}, true
}

// levelExit returns the price and the reason of the trade's sell at its stop
// loss or take profit on candle, if the candle reaches one.
func (state TradeState) levelExit(candle TradeCandle) (float64, ExitReason, bool) {
	stopLoss, takeProfit := state.StopLoss > 0, state.TakeProfit > 0
	switch {
	case stopLoss && candle.Open <= state.StopLoss:
		return candle.Open, ExitStopLoss, true
	case takeProfit && candle.Open >= state.TakeProfit:
		return candle.Open, ExitTakeProfit, true
	case stopLoss && candle.Low <= state.StopLoss:
		return state.StopLoss, ExitStopLoss, true
	case takeProfit && candle.High >= state.TakeProfit:
		return state.TakeProfit, ExitTakeProfit, true
	}
	return 0, "", false
}

// positions are the position variables of the filled trade at close of the
// candle opening at openTime; rules read pnl in percent, while events report
// fractions.
func (entry Entry) positions(trade TradeState, openTime time.Time, close float64) map[string]float64 {
	price := trade.EntryPrice()
	return map[string]float64{
		entryPriceVariable: price,
		pnlVariable:        100 * (close/price - 1),
		barsHeldVariable:   float64(entry.Interval.CandlesBetween(trade.OpenedAt, openTime)),
	}
}
