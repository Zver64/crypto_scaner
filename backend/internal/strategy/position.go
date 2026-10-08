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
	// TradeSell is an exit signal that sells every buy of the trade.
	TradeSell
	// TradeSkip is an entry signal that buys nothing, since the trade does
	// not accumulate or holds max buys.
	TradeSkip
)

// TradeEvent is a signal at the close of a processed candle. A buy names its
// number in the trade; a sell the trade it closes, at the state before the
// candle.
type TradeEvent struct {
	Kind  TradeEventKind
	Buy   int
	Trade TradeState
	// Close is the close of the candle; Return is the fractional return of a sell there.
	Close, Return float64
}

// TradeCandle is the processed candle of the strategy's interval and the
// results of its expressions at the close. The exit result comes from
// evaluating the exit expression over the position variables exit receives;
// it is only asked for while a trade is open.
type TradeCandle struct {
	OpenTime    time.Time
	Open, Close float64
	Entry       bool
	EntryKnown  bool
	Exit        func(positions map[string]float64) (result, known bool)
}

// step processes candle after state. Buys pending since earlier signals fill
// at its open, and the first fill starts the trade. While a trade is open and
// the strategy has an exit, a true exit sells every buy, and the entry counts
// as false on that candle so that the next candle can signal again.
// Otherwise an entry signal opens a trade, or adds a buy when the strategy
// accumulates or has no exit, up to max buys. A candle waits, changing
// nothing and processed false, until every rule it needs is known: the exit
// while a trade is open, and the entry unless the exit sells. Nothing changes
// either for a candle not after the last processed one.
func (entry Entry) step(state TradeState, candle TradeCandle) (next TradeState, events []TradeEvent, processed bool) {
	if !candle.OpenTime.After(state.OpenTime) || !(candle.Open > 0) || !(candle.Close > 0) {
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
	if next.Buys > 0 && entry.Exit != nil {
		price := next.EntryPrice()
		// Rules read pnl in percent; events report fractions.
		pnl := candle.Close/price - 1
		exit, known := candle.Exit(map[string]float64{
			entryPriceVariable: price,
			pnlVariable:        100 * pnl,
			barsHeldVariable:   float64(entry.Interval.CandlesBetween(next.OpenedAt, candle.OpenTime)),
		})
		if !known {
			return state, nil, false
		}
		if exit {
			return TradeState{OpenTime: candle.OpenTime}, []TradeEvent{{Kind: TradeSell, Trade: next, Close: candle.Close, Return: pnl}}, true
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
		next.Buys = 1
	case (entry.Accumulate || entry.Exit == nil) && (entry.MaxBuys == 0 || next.Buys < entry.MaxBuys):
		next.Buys++
	default:
		return next, []TradeEvent{{Kind: TradeSkip, Close: candle.Close}}, true
	}
	return next, []TradeEvent{{Kind: TradeBuy, Buy: next.Buys, Close: candle.Close}}, true
}
