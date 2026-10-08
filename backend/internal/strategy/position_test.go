package strategy

import (
	"errors"
	"testing"

	"crypto-scanner/internal/market"
)

// A candle waits for every rule it needs, and the trade starts at the candle
// its first buy fills at, even when that is not the candle after the signal.
func TestStepWaitsForEveryRuleAndStartsTheTradeAtTheFill(t *testing.T) {
	entry := Entry{Strategy: Strategy{ExitExpression: "x"}, Exit: &Expression{}, Interval: market.IntervalHour}
	exit := func(result, known bool) func(map[string]float64) (bool, bool) {
		return func(map[string]float64) (bool, bool) { return result, known }
	}
	signaled := TradeState{OpenTime: backtestHour(0), Entry: true, Buys: 1}

	// Unknown entry: the pending buy does not fill and nothing is processed.
	if next, _, processed := entry.step(signaled, flatCandle(1, 10, false, false, exit(false, true))); processed || next != signaled {
		t.Fatalf("unknown entry processed %v, state %+v", processed, next)
	}
	// The buy fills at hour 2, which starts the trade.
	filled, _, processed := entry.step(signaled, flatCandle(2, 20, true, true, exit(false, true)))
	if !processed || !filled.OpenedAt.Equal(backtestHour(2)) || filled.Filled != 1 || filled.EntryPrice() != 20 {
		t.Fatalf("fill processed %v, state %+v", processed, filled)
	}
	// Unknown exit during the trade: the candle waits even with a known entry.
	if next, _, processed := entry.step(filled, flatCandle(3, 20, true, true, exit(false, false))); processed || next != filled {
		t.Fatalf("unknown exit processed %v, state %+v", processed, next)
	}
	// A true exit sells without the entry.
	if next, events, processed := entry.step(filled, flatCandle(3, 22, false, false, exit(true, true))); !processed || next.Buys != 0 || len(events) != 1 || events[0].Kind != TradeSell {
		t.Fatalf("exit processed %v, state %+v, events %+v", processed, next, events)
	}
}

// A signal waits for unknown levels, skips levels on the wrong side of the
// close, and otherwise fixes them in the trade it opens.
func TestStepFixesTheLevelsOfTheTradeItOpens(t *testing.T) {
	entry := Entry{TakeProfit: &Expression{}, StopLoss: &Expression{}, Interval: market.IntervalHour}
	idle := TradeState{OpenTime: backtestHour(0)}
	for _, test := range []struct {
		name                 string
		takeProfit, stopLoss float64
		known                bool
		processed            bool
		kind                 TradeEventKind
	}{
		{name: "unknown", takeProfit: 12, stopLoss: 9, known: false, processed: false},
		{name: "take profit below the close", takeProfit: 9, stopLoss: 8, known: true, processed: true, kind: TradeSkip},
		{name: "stop loss above the close", takeProfit: 12, stopLoss: 11, known: true, processed: true, kind: TradeSkip},
		{name: "valid", takeProfit: 12, stopLoss: 9, known: true, processed: true, kind: TradeBuy},
	} {
		t.Run(test.name, func(t *testing.T) {
			candle := flatCandle(1, 10, true, true, nil)
			candle.Levels = func() (float64, float64, bool) { return test.takeProfit, test.stopLoss, test.known }
			next, events, processed := entry.step(idle, candle)
			if processed != test.processed || test.processed && (len(events) != 1 || events[0].Kind != test.kind) {
				t.Fatalf("processed %v, events %+v", processed, events)
			}
			bought := test.kind == TradeBuy
			if bought != (next.Buys == 1) || bought && (next.TakeProfit != test.takeProfit || next.StopLoss != test.stopLoss) {
				t.Fatalf("state %+v", next)
			}
		})
	}
}

// flatCandle is the candle of backtestHour(index) at price throughout.
func flatCandle(index int, price float64, entry, known bool, exit func(map[string]float64) (bool, bool)) TradeCandle {
	return TradeCandle{
		OpenTime: backtestHour(index), Open: price, High: price, Low: price, Close: price, Entry: entry, EntryKnown: known, Exit: exit,
		Levels: func() (float64, float64, bool) { return 0, 0, true },
	}
}

// A signal outside the market cap range buys nothing but still counts as the
// entry, while an open trade sells as usual.
func TestStepSkipsSignalsOutOfRange(t *testing.T) {
	entry := Entry{Interval: market.IntervalHour}
	candle := flatCandle(1, 10, true, true, nil)
	candle.OutOfRange = true
	next, events, processed := entry.step(TradeState{OpenTime: backtestHour(0)}, candle)
	if !processed || next.Buys != 0 || !next.Entry || len(events) != 1 || events[0].Kind != TradeSkip {
		t.Fatalf("processed %v, state %+v, events %+v", processed, next, events)
	}

	exiting := Entry{StopLoss: &Expression{}, Interval: market.IntervalHour}
	trade := TradeState{OpenTime: backtestHour(1), Entry: true, Buys: 1, Filled: 1, Quantity: 0.1, OpenedAt: backtestHour(1), StopLoss: 9}
	candle = flatCandle(2, 8, false, true, nil)
	candle.OutOfRange = true
	if next, events, processed := exiting.step(trade, candle); !processed || next.Buys != 0 || len(events) != 1 || events[0].Reason != ExitStopLoss {
		t.Fatalf("processed %v, state %+v, events %+v", processed, next, events)
	}
}

// A signal announces every turn of its entry from false to true and never
// buys.
func TestStepSignalsWithoutBuying(t *testing.T) {
	entry := Entry{Strategy: Strategy{Signal: SignalShort}, Interval: market.IntervalHour}
	state := TradeState{OpenTime: backtestHour(0)}
	var kinds []TradeEventKind
	for index, value := range []bool{true, true, false, true} {
		next, events, processed := entry.step(state, flatCandle(index+1, 10, value, true, nil))
		if !processed || next.Buys != 0 || next.Entry != value {
			t.Fatalf("candle %d processed %v, state %+v", index+1, processed, next)
		}
		for _, event := range events {
			kinds = append(kinds, event.Kind)
		}
		state = next
	}
	if len(kinds) != 2 || kinds[0] != TradeSignal || kinds[1] != TradeSignal {
		t.Fatalf("events %v", kinds)
	}
}

// A signal is long, short, or sideways and has no trading settings.
func TestSignalsRejectTradingSettings(t *testing.T) {
	service := newBacktestService(t, newBacktestStore(market.IntervalHour, nil))
	usd := 1e9
	for _, item := range []Strategy{
		{Signal: "up"},
		{Signal: SignalShort, ExitExpression: "h_close > 1"},
		{Signal: SignalShort, TakeProfitExpression: "h_close * 2"},
		{Signal: SignalShort, StopLossExpression: "h_close / 2"},
		{Signal: SignalShort, MarketCap: MarketCapRange{MaxUSD: &usd}},
	} {
		item.Name, item.Expression = "Signal", "h_close > 5"
		if _, err := service.entry(item); !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("%+v: %v", item, err)
		}
	}
	if _, err := service.entry(Strategy{Name: "Signal", Signal: SignalSideways, Expression: "h_close > 5"}); err != nil {
		t.Fatal(err)
	}
}
