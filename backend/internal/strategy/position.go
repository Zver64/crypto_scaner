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
	// selling counts the buys an exit rule sold at the last processed
	// candle, which fill at the open of the next candle. The monitor neither
	// stores nor reads it: it announces no fill.
	selling int
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
	// TradeSkip is an entry signal that buys or announces nothing, since the
	// coin is outside the market cap range, the strategy exits and holds a trade, or
	// its take profit or stop loss is not on its side of the close.
	TradeSkip
	// TradeSignal is an entry signal of a signal, which buys nothing.
	TradeSignal
	// TradeBuyFill fills the buys pending since earlier signals at the open
	// of the candle.
	TradeBuyFill
	// TradeSellFill fills the sell an exit rule signaled at the candle before
	// at the open of the candle.
	TradeSellFill
)

// fill reports whether the event fills orders, which the monitor does not
// announce.
func (kind TradeEventKind) fill() bool {
	return kind == TradeBuyFill || kind == TradeSellFill
}

// ExitReason tells what sold a trade.
type ExitReason string

const (
	ExitTakeProfit ExitReason = "take_profit"
	ExitStopLoss   ExitReason = "stop_loss"
	ExitRuleSignal ExitReason = "exit"
)

// TradeEvent is what a processed candle did. A buy names its number in the
// trade and the trade after it; a sell the trade it closes, at the state
// before the candle, and what sold it. A fill names the open time of its
// candle, Time, the open it filled at, Price, and Count, the buys it filled
// or sold; a buy fill also the trade after it. The fills of a candle come
// before its other events.
type TradeEvent struct {
	Kind  TradeEventKind
	Buy   int
	Trade TradeState
	// Close is the close of the candle. Reason tells what sold a sell;
	// Price is the price a take profit or stop loss filled at on this
	// candle, while an exit rule sells at the next open, which its
	// TradeSellFill reports. Return is the fractional return of a sell at
	// Price, or at the close for an exit rule.
	Close, Price, Return float64
	Reason               ExitReason
	Time                 time.Time
	Count                int
}

// TradeCandle is the processed candle of the strategy's interval and the
// results of its expressions at the close. The exit result comes from
// evaluating the exit expression over the position variables exit receives;
// it is only asked for while a trade is open. Levels returns the take profit
// and stop loss prices at the close, 0 for those the strategy lacks; it is
// only asked for on a signal that opens a trade. OutOfRange skips entry
// signals, of signals too, as for a coin outside the market cap range.
// EntryErr and the errors of Exit and Levels are rule errors that decide
// their result, such as a *ShiftError, naming the rule.
type TradeCandle struct {
	OpenTime               time.Time
	Open, High, Low, Close float64
	Entry                  bool
	EntryKnown             bool
	EntryErr               error
	OutOfRange             bool
	Exit                   func(positions map[string]float64) (result, known bool, err error)
	Levels                 func() (takeProfit, stopLoss float64, known bool, err error)
}

// step processes candle after state. A sell an exit rule signaled at the
// candle before fills at its open, whatever else the candle holds. Buys
// pending since earlier signals fill at its open, and the first fill starts
// the trade. A filled trade then sells at its stop loss or take profit when
// the candle reaches them: at the open when it opens past one, otherwise at
// the price itself, the stop loss first when the candle reaches both.
// Otherwise, while a trade is open, a true exit rule sells every buy at the
// next open. A sell counts the entry as false on its candle, so that the
// next candle can signal again. An entry signal outside the market cap range
// is skipped. Otherwise an entry signal of a signal only announces itself;
// otherwise it opens a trade, fixing its take profit and stop loss when the
// close lies between them and skipping otherwise, or, for a strategy that
// never sells, adds a buy; a strategy that exits skips signals while it holds
// a trade. A candle waits, changing nothing but the fill of a pending sell
// and processed false, until every rule it needs is known: the exit rule
// while a trade is open, and the entry and the levels of a trade it opens
// unless the trade sells; pending buys wait with it. Nothing changes either
// for a candle not after the last processed one. A rule error of a rule the
// candle needs processes nothing and is returned: it halts the strategy.
func (entry Entry) step(state TradeState, candle TradeCandle) (next TradeState, events []TradeEvent, processed bool, err error) {
	if !candle.OpenTime.After(state.OpenTime) {
		return state, nil, false, nil
	}
	var sold []TradeEvent
	if state.selling > 0 && candle.Open > 0 {
		sold = []TradeEvent{{Kind: TradeSellFill, Time: candle.OpenTime, Price: candle.Open, Count: state.selling}}
		state.selling = 0
	}
	// wait leaves the candle unprocessed, reporting only the fill of a
	// pending sell.
	wait := func(err error) (TradeState, []TradeEvent, bool, error) {
		return state, sold, false, err
	}
	if !(candle.Open > 0) || !(candle.High > 0) || !(candle.Low > 0) || !(candle.Close > 0) {
		return wait(nil)
	}
	next = state
	fills := sold
	if pending := next.Buys - next.Filled; pending > 0 {
		if next.Filled == 0 {
			next.OpenedAt = candle.OpenTime
		}
		next.Quantity += float64(pending) / candle.Open
		next.Filled = next.Buys
		fills = append(fills, TradeEvent{Kind: TradeBuyFill, Trade: next, Time: candle.OpenTime, Price: candle.Open, Count: pending})
	}
	// done processes the candle, reporting its fills before events.
	done := func(next TradeState, events ...TradeEvent) (TradeState, []TradeEvent, bool, error) {
		return next, append(fills, events...), true, nil
	}
	if next.Buys > 0 {
		if price, reason, ok := next.levelExit(entry.Direction, candle); ok {
			return done(TradeState{OpenTime: candle.OpenTime}, TradeEvent{
				Kind: TradeSell, Trade: next, Close: candle.Close, Price: price, Return: entry.Direction.gross(next.EntryPrice(), price), Reason: reason,
			})
		}
	}
	if next.Buys > 0 && entry.Exit != nil {
		exit, known, err := candle.Exit(entry.positions(next, candle.OpenTime, candle.Close))
		if err != nil || !known {
			return wait(err)
		}
		if exit {
			return done(TradeState{OpenTime: candle.OpenTime, selling: next.Filled}, TradeEvent{
				Kind: TradeSell, Trade: next, Close: candle.Close, Return: entry.Direction.gross(next.EntryPrice(), candle.Close), Reason: ExitRuleSignal,
			})
		}
	}
	if candle.EntryErr != nil || !candle.EntryKnown {
		return wait(candle.EntryErr)
	}
	next.OpenTime = candle.OpenTime
	signal := candle.Entry && !next.Entry
	next.Entry = candle.Entry
	if !signal {
		return done(next)
	}
	if candle.OutOfRange {
		return done(next, TradeEvent{Kind: TradeSkip, Close: candle.Close})
	}
	if entry.Signal {
		return done(next, TradeEvent{Kind: TradeSignal, Close: candle.Close})
	}
	switch {
	case next.Buys == 0:
		takeProfit, stopLoss, known, err := candle.Levels()
		if err != nil || !known {
			return wait(err)
		}
		if !entry.levelsAround(takeProfit, stopLoss, candle.Close) {
			return done(next, TradeEvent{Kind: TradeSkip, Close: candle.Close})
		}
		next.Buys, next.TakeProfit, next.StopLoss = 1, takeProfit, stopLoss
	case !entry.Exits():
		next.Buys++
	default:
		return done(next, TradeEvent{Kind: TradeSkip, Close: candle.Close})
	}
	return done(next, TradeEvent{Kind: TradeBuy, Buy: next.Buys, Trade: next, Close: candle.Close})
}

// levelsAround reports whether the take profit and stop loss the strategy
// has lie on their sides of close: a long strategy's take profit above it
// and its stop loss between 0 and it, a short one's the other way around.
func (entry Entry) levelsAround(takeProfit, stopLoss, close float64) bool {
	if entry.Direction == DirectionShort {
		return (entry.TakeProfit == nil || takeProfit > 0 && takeProfit < close) && (entry.StopLoss == nil || stopLoss > close)
	}
	return (entry.TakeProfit == nil || takeProfit > close) && (entry.StopLoss == nil || stopLoss > 0 && stopLoss < close)
}

// levelExit returns the price and the reason of the close of the trade in
// direction at its stop loss or take profit on candle, if the candle reaches
// one.
func (state TradeState) levelExit(direction Direction, candle TradeCandle) (float64, ExitReason, bool) {
	stopLoss, takeProfit := state.StopLoss > 0, state.TakeProfit > 0
	// A short trade loses as the price rises, so its prices are mirrored
	// for the comparisons of a long trade.
	open, low, high, stopLossPrice, takeProfitPrice := candle.Open, candle.Low, candle.High, state.StopLoss, state.TakeProfit
	if direction == DirectionShort {
		open, low, high, stopLossPrice, takeProfitPrice = -open, -high, -low, -stopLossPrice, -takeProfitPrice
	}
	switch {
	case stopLoss && open <= stopLossPrice:
		return candle.Open, ExitStopLoss, true
	case takeProfit && open >= takeProfitPrice:
		return candle.Open, ExitTakeProfit, true
	case stopLoss && low <= stopLossPrice:
		return state.StopLoss, ExitStopLoss, true
	case takeProfit && high >= takeProfitPrice:
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
		pnlVariable:        100 * entry.Direction.gross(price, close),
		barsHeldVariable:   float64(entry.Interval.CandlesBetween(trade.OpenedAt, openTime)),
	}
}
