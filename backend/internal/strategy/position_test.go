package strategy

import (
	"testing"
	"time"

	"crypto-scanner/internal/market"
)

// A candle waits for every rule it needs, and the trade starts at the candle
// its first buy fills at, even when that is not the candle after the signal.
func TestStepWaitsForEveryRuleAndStartsTheTradeAtTheFill(t *testing.T) {
	entry := Entry{Strategy: Strategy{ExitExpression: "x"}, Exit: &Expression{}, Interval: market.IntervalHour}
	hour := func(index int) time.Time { return backtestStart.Add(time.Duration(index) * time.Hour) }
	exit := func(result, known bool) func(map[string]float64) (bool, bool) {
		return func(map[string]float64) (bool, bool) { return result, known }
	}
	signaled := TradeState{OpenTime: hour(0), Entry: true, Buys: 1}

	// Unknown entry: the pending buy does not fill and nothing is processed.
	if next, _, processed := entry.step(signaled, TradeCandle{OpenTime: hour(1), Open: 10, Close: 10, Exit: exit(false, true)}); processed || next != signaled {
		t.Fatalf("unknown entry processed %v, state %+v", processed, next)
	}
	// The buy fills at hour 2, which starts the trade.
	filled, _, processed := entry.step(signaled, TradeCandle{OpenTime: hour(2), Open: 20, Close: 20, Entry: true, EntryKnown: true, Exit: exit(false, true)})
	if !processed || !filled.OpenedAt.Equal(hour(2)) || filled.Filled != 1 || filled.EntryPrice() != 20 {
		t.Fatalf("fill processed %v, state %+v", processed, filled)
	}
	// Unknown exit during the trade: the candle waits even with a known entry.
	if next, _, processed := entry.step(filled, TradeCandle{OpenTime: hour(3), Open: 20, Close: 20, Entry: true, EntryKnown: true, Exit: exit(false, false)}); processed || next != filled {
		t.Fatalf("unknown exit processed %v, state %+v", processed, next)
	}
	// A true exit sells without the entry.
	if next, events, processed := entry.step(filled, TradeCandle{OpenTime: hour(3), Open: 20, Close: 22, Exit: exit(true, true)}); !processed || next.Buys != 0 || len(events) != 1 || events[0].Kind != TradeSell {
		t.Fatalf("exit processed %v, state %+v, events %+v", processed, next, events)
	}
}
