package strategy

import (
	"context"
	"errors"
	"math"
	"testing"

	"crypto-scanner/internal/market"
)

// A candle waits for every rule it needs, and the trade starts at the candle
// its first buy fills at, even when that is not the candle after the signal.
func TestStepWaitsForEveryRuleAndStartsTheTradeAtTheFill(t *testing.T) {
	entry := Entry{Strategy: Strategy{ExitExpression: "x"}, Exit: &Expression{}, Interval: market.IntervalHour}
	exit := func(result, known bool) func(map[string]float64) (bool, bool, error) {
		return func(map[string]float64) (bool, bool, error) { return result, known, nil }
	}
	signaled := TradeState{OpenTime: backtestHour(0), Entry: true, Buys: 1}

	// Unknown entry: the pending buy does not fill and nothing is processed.
	if next, _, processed, _ := entry.step(signaled, flatCandle(1, 10, false, false, exit(false, true))); processed || next != signaled {
		t.Fatalf("unknown entry processed %v, state %+v", processed, next)
	}
	// The buy fills at hour 2, which starts the trade.
	filled, _, processed, _ := entry.step(signaled, flatCandle(2, 20, true, true, exit(false, true)))
	if !processed || !filled.OpenedAt.Equal(backtestHour(2)) || filled.Filled != 1 || filled.EntryPrice() != 20 {
		t.Fatalf("fill processed %v, state %+v", processed, filled)
	}
	// Unknown exit during the trade: the candle waits even with a known entry.
	if next, _, processed, _ := entry.step(filled, flatCandle(3, 20, true, true, exit(false, false))); processed || next != filled {
		t.Fatalf("unknown exit processed %v, state %+v", processed, next)
	}
	// A true exit sells without the entry.
	if next, events, processed, _ := entry.step(filled, flatCandle(3, 22, false, false, exit(true, true))); !processed || next.Buys != 0 || len(events) != 1 || events[0].Kind != TradeSell {
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
			candle.Levels = func() (float64, float64, bool, error) { return test.takeProfit, test.stopLoss, test.known, nil }
			next, events, processed, _ := entry.step(idle, candle)
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

// A short trade opens with its take profit below and its stop loss above the
// close, closes at the stop loss when the price rises to it, the stop loss
// first, and at the take profit when the price falls to it, and gains as the
// price falls.
func TestStepTradesShort(t *testing.T) {
	entry := Entry{Strategy: Strategy{Direction: DirectionShort}, TakeProfit: &Expression{}, StopLoss: &Expression{}, Interval: market.IntervalHour}
	for _, test := range []struct {
		takeProfit, stopLoss float64
		kind                 TradeEventKind
	}{{takeProfit: 11, stopLoss: 12, kind: TradeSkip}, {takeProfit: 8, stopLoss: 9, kind: TradeSkip}, {takeProfit: 8, stopLoss: 12, kind: TradeBuy}} {
		candle := flatCandle(1, 10, true, true, nil)
		candle.Levels = func() (float64, float64, bool, error) { return test.takeProfit, test.stopLoss, true, nil }
		if _, events, _, _ := entry.step(TradeState{OpenTime: backtestHour(0)}, candle); len(events) != 1 || events[0].Kind != test.kind {
			t.Fatalf("levels %v/%v: events %+v", test.takeProfit, test.stopLoss, events)
		}
	}

	trade := TradeState{OpenTime: backtestHour(1), Entry: true, Buys: 1, Filled: 1, Quantity: 0.1, OpenedAt: backtestHour(1), TakeProfit: 8, StopLoss: 12}
	for _, test := range []struct {
		name            string
		open, high, low float64
		reason          ExitReason
		price, want     float64
	}{
		{name: "gap over the stop loss", open: 13, high: 13, low: 13, reason: ExitStopLoss, price: 13, want: -0.3},
		{name: "rise to the stop loss", open: 11, high: 12.5, low: 11, reason: ExitStopLoss, price: 12, want: -0.2},
		{name: "fall to the take profit", open: 9, high: 9, low: 7.5, reason: ExitTakeProfit, price: 8, want: 0.2},
		{name: "both, stop loss first", open: 10, high: 12.5, low: 7.5, reason: ExitStopLoss, price: 12, want: -0.2},
	} {
		t.Run(test.name, func(t *testing.T) {
			candle := flatCandle(2, test.open, false, true, nil)
			candle.High, candle.Low = test.high, test.low
			_, events, processed, _ := entry.step(trade, candle)
			if !processed || len(events) != 1 || events[0].Reason != test.reason || events[0].Price != test.price ||
				math.Abs(events[0].Return-test.want) > 1e-9 {
				t.Fatalf("processed %v, events %+v", processed, events)
			}
		})
	}
	if pnl := entry.positions(trade, backtestHour(2), 9)[pnlVariable]; math.Abs(pnl-10) > 1e-9 {
		t.Fatalf("pnl %v", pnl)
	}
}

// flatCandle is the candle of backtestHour(index) at price throughout.
func flatCandle(index int, price float64, entry, known bool, exit func(map[string]float64) (bool, bool, error)) TradeCandle {
	return TradeCandle{
		OpenTime: backtestHour(index), Open: price, High: price, Low: price, Close: price, Entry: entry, EntryKnown: known, Exit: exit,
		Levels: func() (float64, float64, bool, error) { return 0, 0, true, nil },
	}
}

// A signal outside the market cap range buys or announces nothing but still
// counts as the entry, while an open trade sells as usual.
func TestStepSkipsSignalsOutOfRange(t *testing.T) {
	for _, entry := range []Entry{
		{Interval: market.IntervalHour},
		{Strategy: Strategy{Signal: true, Direction: DirectionLong}, Interval: market.IntervalHour},
	} {
		candle := flatCandle(1, 10, true, true, nil)
		candle.OutOfRange = true
		next, events, processed, _ := entry.step(TradeState{OpenTime: backtestHour(0)}, candle)
		if !processed || next.Buys != 0 || !next.Entry || len(events) != 1 || events[0].Kind != TradeSkip {
			t.Fatalf("signal %v: processed %v, state %+v, events %+v", entry.Signal, processed, next, events)
		}
	}

	exiting := Entry{StopLoss: &Expression{}, Interval: market.IntervalHour}
	trade := TradeState{OpenTime: backtestHour(1), Entry: true, Buys: 1, Filled: 1, Quantity: 0.1, OpenedAt: backtestHour(1), StopLoss: 9}
	candle := flatCandle(2, 8, false, true, nil)
	candle.OutOfRange = true
	if next, events, processed, _ := exiting.step(trade, candle); !processed || next.Buys != 0 || len(events) != 1 || events[0].Reason != ExitStopLoss {
		t.Fatalf("processed %v, state %+v, events %+v", processed, next, events)
	}
}

// A signal announces every turn of its entry from false to true and never
// buys.
func TestStepSignalsWithoutBuying(t *testing.T) {
	entry := Entry{Strategy: Strategy{Signal: true, Direction: DirectionShort}, Interval: market.IntervalHour}
	state := TradeState{OpenTime: backtestHour(0)}
	var kinds []TradeEventKind
	for index, value := range []bool{true, true, false, true} {
		next, events, processed, _ := entry.step(state, flatCandle(index+1, 10, value, true, nil))
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

// A signal is long, short, or sideways, has a target ratio and a window, and
// has no trading settings but may have a market cap range; a strategy trades
// long or short, and a short one always has a take profit and a stop loss.
func TestDirectionRules(t *testing.T) {
	service := newBacktestService(t, newBacktestStore(market.IntervalHour, nil))
	usd := 1e9
	for _, item := range []Strategy{
		{Signal: true, Direction: "up"},
		{Signal: true, Direction: DirectionShort, ExitExpression: "h_close > 1"},
		{Signal: true, Direction: DirectionShort, TakeProfitExpression: "h_close * 2"},
		{Signal: true, Direction: DirectionShort, StopLossExpression: "h_close / 2"},
		{Signal: true, Direction: DirectionLong, Window: 6},
		{Signal: true, Direction: DirectionLong, TargetRatio: 1, Window: 6},
		{Signal: true, Direction: DirectionLong, TargetRatio: 2, Window: 5},
		{Direction: DirectionLong, TargetRatio: 2, Window: 6},
		{Direction: DirectionSideways},
		{Direction: DirectionShort, StopLossExpression: "h_close * 2"},
		{Direction: DirectionShort, TakeProfitExpression: "h_close / 2", ExitExpression: "h_close > 1"},
	} {
		item.Name, item.Expression = "Strategy", "h_close > 5"
		if _, err := service.entry(item); !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("%+v: %v", item, err)
		}
	}
	for _, item := range []Strategy{
		{Signal: true, Direction: DirectionSideways, TargetRatio: 5, Window: 24},
		{Signal: true, Direction: DirectionShort, MarketCap: MarketCapRange{MaxUSD: &usd}, TargetRatio: 2, Window: 3},
		{Direction: DirectionShort, TakeProfitExpression: "h_close / 2", StopLossExpression: "h_close * 2"},
	} {
		item.Name, item.Expression = "Strategy", "h_close > 5"
		if _, err := service.entry(item); err != nil {
			t.Fatalf("%+v: %v", item, err)
		}
	}

	// A saved strategy keeps its kind and direction.
	saved := newBacktestService(t, newBacktestStore(market.IntervalHour, nil), Strategy{ID: 1, Name: "Trade", Direction: DirectionLong, Expression: "h_close > 5"})
	for _, item := range []Strategy{
		{Signal: true, Direction: DirectionLong},
		{Direction: DirectionShort, TakeProfitExpression: "h_close / 2", StopLossExpression: "h_close * 2"},
	} {
		item.ID, item.Name, item.Expression = 1, "Trade", "h_close > 5"
		if _, err := saved.Update(context.Background(), item); !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("%+v: %v", item, err)
		}
	}
}
