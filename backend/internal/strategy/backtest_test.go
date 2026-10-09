package strategy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"math"
	"reflect"
	"slices"
	"strconv"
	"testing"
	"time"

	"crypto-scanner/internal/closedindicator"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/indicator/candle"
	indicatortalib "crypto-scanner/internal/indicator/talib"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/scannerindicator"
)

var backtestStart = time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)

func TestBacktestReplaysAlertTransitions(t *testing.T) {
	hour := func(index int) time.Time { return backtestStart.Add(time.Duration(index) * time.Hour) }
	// Hour 5 is missing, so prev is unknown at hour 6.
	closes := map[int]float64{0: 6, 1: 7, 2: 8, 3: 2, 4: 9, 6: 9, 7: 9, 8: 1, 9: 9}
	var hourly []market.Candle
	for index := range 10 {
		if value, ok := closes[index]; ok {
			hourly = append(hourly, testCandle(1, market.IntervalHour, hour(index), value))
		}
	}
	store := newBacktestStore(market.IntervalHour, hourly)
	service := newBacktestService(t, store, Strategy{ID: 1, Name: "Breakout", Expression: "h_close > 5 && prev(h_close) > 0"})

	// Hour 0 is unknown, hours 2 and 7 keep matching, and hour 6 is unknown
	// while matching.
	assertReplay(t, service, 1, market.IntervalHour, Backtest{From: hour(0), To: hour(9), Alerts: []time.Time{hour(1), hour(4), hour(9)}})
}

// A history of at least the synchronized depth may have lost older candles,
// so its oldest candles lack the warm-up the tracker loads and are not
// evaluated; every stored candle after them is, however deep.
func TestBacktestSkipsCandlesBeforeTheWarmUpOfAPrunedHistory(t *testing.T) {
	hour := func(index int) time.Time { return backtestStart.Add(time.Duration(index) * time.Hour) }
	hourly := make([]market.Candle, market.SyncDepth+1000)
	for index := range hourly {
		hourly[index] = testCandle(1, market.IntervalHour, hour(index), 9)
	}
	service := newBacktestService(t, newBacktestStore(market.IntervalHour, hourly), Strategy{ID: 1, Name: "Close", Expression: "h_close > 5"})

	depth, err := closedindicator.Depth(testRegistry(t), CandleTarget(market.IntervalHour), 1)
	if err != nil {
		t.Fatal(err)
	}
	first := hour(depth - 1)
	assertReplay(t, service, 1, market.IntervalHour, Backtest{From: first, To: hour(len(hourly) - 1), Alerts: []time.Time{first}})
}

// ETHUSDT has history but is not among the administrator's favorites, so the
// values read through of stay unknown and nothing alerts.
func TestBacktestLeavesCoinsOutsideTheFavoritesUnknown(t *testing.T) {
	var hourly, other []market.Candle
	for index := range 5 {
		open := backtestStart.Add(time.Duration(index) * time.Hour)
		hourly = append(hourly, testCandle(1, market.IntervalHour, open, 9))
		other = append(other, testCandle(2, market.IntervalHour, open, 9))
	}
	store := newBacktestStore(market.IntervalHour, hourly)
	store.candles[market.IntervalHour][2] = other
	service := newBacktestService(t, store, Strategy{ID: 1, Name: "Pair", Expression: `h_close > 5 && of("ETHUSDT", h_close) > 5`})

	assertReplay(t, service, 1, market.IntervalHour, Backtest{From: backtestStart, To: backtestStart.Add(4 * time.Hour)})
}

func TestBacktestWithoutHistoryEvaluatesNothing(t *testing.T) {
	service := newBacktestService(t, newBacktestStore(market.IntervalDay, nil), Strategy{ID: 1, Name: "Daily", Expression: "d_close > 5"})

	assertReplay(t, service, 1, market.IntervalDay, Backtest{})
}

func TestBacktestReadsDailyValuesOnlyAfterTheDayCloses(t *testing.T) {
	day := backtestStart
	daily := []market.Candle{testCandle(1, market.IntervalDay, day.AddDate(0, 0, -1), 100), testCandle(1, market.IntervalDay, day, 10)}
	var hourly []market.Candle
	for index := range 30 {
		hourly = append(hourly, testCandle(1, market.IntervalHour, day.Add(time.Duration(index)*time.Hour), 50))
	}
	store := newBacktestStore(market.IntervalHour, hourly)
	store.candles[market.IntervalDay] = map[int64][]market.Candle{1: daily}
	service := newBacktestService(t, store, Strategy{ID: 1, Name: "Daily", Expression: "h_close > d_close"})

	// The hourly candle opened at 23:00 closes with the day.
	assertReplay(t, service, 1, market.IntervalHour, Backtest{From: day, To: day.Add(29 * time.Hour), Alerts: []time.Time{day.Add(23 * time.Hour)}})
}

func TestBacktestRejectsUnknownAndInvalidStrategiesAndArguments(t *testing.T) {
	store := newBacktestStore(market.IntervalHour, nil)
	service := newBacktestService(t, store, Strategy{ID: 1, Name: "Broken", Expression: "h_unknown > 1"}, Strategy{ID: 2, Name: "Valid", Expression: "h_close > 1"})
	for _, test := range []struct {
		id       int64
		symbol   string
		from, to time.Time
		want     error
	}{
		{id: 3, symbol: "BTCUSDT", want: ErrNotFound},
		{id: 1, symbol: "BTCUSDT", want: ErrInvalidArgument},
		{id: 2, symbol: "XRPUSDT", want: market.ErrInstrumentNotFound},
		{id: 2, symbol: " ", want: ErrInvalidArgument},
		{id: 2, symbol: "BTCUSDT", from: backtestHour(2), to: backtestHour(1), want: ErrInvalidArgument},
	} {
		if _, err := service.Backtest(context.Background(), test.id, test.symbol, test.from, test.to); !errors.Is(err, test.want) {
			t.Fatalf("Backtest(%d, %q, %v, %v) error = %v, want %v", test.id, test.symbol, test.from, test.to, err, test.want)
		}
	}
}

// A draft replays exactly like the saved strategy with the same rules, and is
// checked like a save.
func TestBacktestDraftReplaysLikeASavedStrategy(t *testing.T) {
	saved := Strategy{
		ID: 1, Name: "Top", Direction: DirectionShort, Expression: "h_close > 5",
		TakeProfitExpression: "h_close * 0.8", StopLossExpression: "h_close * 1.5",
	}
	service := newBacktestService(t, newBacktestStore(market.IntervalHour, hourlyCloses(1, 10, 10, 4)), saved)

	want, err := service.Backtest(context.Background(), 1, "BTCUSDT", time.Time{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := service.BacktestDraft(context.Background(), Strategy{
		Direction: saved.Direction, Expression: saved.Expression,
		TakeProfitExpression: saved.TakeProfitExpression, StopLossExpression: saved.StopLossExpression,
	}, "BTCUSDT", time.Time{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) || len(got.Trades) != 1 {
		t.Fatalf("BacktestDraft() = %+v, want %+v", got, want)
	}
	if _, err := service.BacktestDraft(context.Background(), Strategy{Direction: DirectionLong, Expression: `h_close > 1 && of("BTCUSDT", h_close) > 5`}, "BTCUSDT", time.Time{}, time.Time{}); err != nil {
		t.Fatalf("BacktestDraft() reading a favorite: %v", err)
	}
	for _, draft := range []Strategy{
		{Expression: "h_close > 5"},
		{Direction: DirectionLong, Expression: "h_unknown > 5"},
		{Direction: DirectionShort, Expression: "h_close > 5"},
		{Signal: true, Direction: DirectionSideways, Expression: "h_close > 5"},
		{Direction: DirectionLong, Expression: `h_close > 1 && of("ETHUSDT", h_close) > 5`},
	} {
		if _, err := service.BacktestDraft(context.Background(), draft, "BTCUSDT", time.Time{}, time.Time{}); !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("BacktestDraft(%+v) error = %v, want %v", draft, err, ErrInvalidArgument)
		}
	}
}

func TestBacktestRefusesBeyondTheRunningLimit(t *testing.T) {
	service := newBacktestService(t, newBacktestStore(market.IntervalHour, hourlyCloses(1, 10)), Strategy{ID: 1, Name: "Close", Expression: "h_close > 5"})
	for range maxBacktests {
		service.backtests <- struct{}{}
	}

	if _, err := service.Backtest(context.Background(), 1, "BTCUSDT", time.Time{}, time.Time{}); !errors.Is(err, ErrBacktestBusy) {
		t.Fatalf("Backtest() error = %v, want %v", err, ErrBacktestBusy)
	}
	<-service.backtests
	if _, err := service.Backtest(context.Background(), 1, "BTCUSDT", time.Time{}, time.Time{}); err != nil {
		t.Fatalf("Backtest() with a free slot: %v", err)
	}
}

// A short strategy gains as the price falls to its take profit and has no
// baselines.
func TestBacktestShortGainsOnAFall(t *testing.T) {
	service := newBacktestService(t, newBacktestStore(market.IntervalHour, hourlyCloses(1, 10, 10, 4)), Strategy{
		ID: 1, Name: "Top", Direction: DirectionShort, Expression: "h_close > 5",
		TakeProfitExpression: "h_close * 0.8", StopLossExpression: "h_close * 1.5",
	})

	result, err := service.Backtest(context.Background(), 1, "BTCUSDT", time.Time{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	want := Trade{EntryTime: backtestHour(2), EntryPrice: 10, ExitTime: backtestHour(2), ExitPrice: 8, Buys: 1, Return: 0.2 - BacktestFee*1.8}
	if len(result.Trades) != 1 || !tradeNear(result.Trades[0], want) || result.Trades[0].Reason != ExitTakeProfit {
		t.Fatalf("trades = %+v", result.Trades)
	}
	if result.Direction != DirectionShort || result.BuyAndHold != nil || result.DCA != nil {
		t.Fatalf("direction %s, buy and hold %v, DCA %v", result.Direction, result.BuyAndHold, result.DCA)
	}
	// A short loses at most everything, however far the price rose.
	if loss := netReturn(DirectionShort, 10, 25); loss != -1 {
		t.Fatalf("loss after a rise to 2.5 times the entry = %v", loss)
	}
}

// A strategy with an exit holds one buy per trade and skips the signals of
// hours 4 and 6 meanwhile. It sells at the open after its exit signal; the
// exit wins over a true entry on its candle, and the entry counts as false
// there, so the next candle opens a new trade, which the end of the history
// sells at its close.
func TestBacktestExitingStrategySkipsSignalsDuringATrade(t *testing.T) {
	service := newBacktestService(t, newBacktestStore(market.IntervalHour, hourlyCloses(1, 9, 9, 1, 9, 1, 9, 30, 30, 30)), Strategy{
		ID: 1, Name: "Dip", Expression: "h_close > 5", ExitExpression: "h_close > 20",
	})

	result, err := service.Backtest(context.Background(), 1, "BTCUSDT", time.Time{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	first, second := netReturn(DirectionLong, 9, 30), netReturn(DirectionLong, 30, 30)
	wantTrades := []Trade{
		{EntryTime: backtestHour(2), EntryPrice: 9, ExitTime: backtestHour(8), ExitPrice: 30, Buys: 1, Return: first},
		{EntryTime: backtestHour(9), EntryPrice: 30, ExitTime: backtestHour(9), ExitPrice: 30, Buys: 1, Return: second},
	}
	if !slices.EqualFunc(result.Trades, wantTrades, tradeNear) || result.Skipped != 2 ||
		!slices.EqualFunc(result.Alerts, []time.Time{backtestHour(1), backtestHour(8)}, time.Time.Equal) {
		t.Fatalf("trades = %+v, skipped %d, alerts %v", result.Trades, result.Skipped, result.Alerts)
	}
	exit := result.Trades[0]
	if exit.Reason != ExitRuleSignal || !exit.ExitSignal.Equal(backtestHour(7)) || !maps.Equal(exit.ExitValues, map[string]float64{"h_close": 30}) {
		t.Fatalf("exit = %s at %v, values %v", exit.Reason, exit.ExitSignal, exit.ExitValues)
	}
	// The trade falls from 9 to 1 at the close of hour 3 before it profits.
	if !near(result.NetProfit, (1+first)*(1+second)-1) || !near(result.MaxDrawdown, -netReturn(DirectionLong, 9, 1)) {
		t.Fatalf("net %v, drawdown %v", result.NetProfit, result.MaxDrawdown)
	}
	assertStats(t, "strategy", result.Stats, TradeStats{
		Count: 2, WinRate: new(0.5), AverageTrade: new((first + second) / 2), AverageWin: new(first),
		AverageLoss: new(second), ProfitFactor: new(first / -second), AverageBars: new(4.0), ExitRules: 2,
	})
}

// Take profit and stop loss are fixed at the signal and sell on the candle
// that reaches them: at the price itself, at the stop loss when the candle
// reaches both, and at the open when it gaps past one.
func TestBacktestSellsAtTakeProfitAndStopLoss(t *testing.T) {
	type bar struct{ open, high, low, close float64 }
	bars := []bar{
		{1, 1, 1, 1},
		{10, 10, 10, 10},    // signal: take profit 12, stop loss 9
		{10, 11, 9.5, 10},   // fills at 10
		{10, 12.5, 9.8, 12}, // take profit at 12
		{12, 12, 12, 12},    // signal: take profit 14.4, stop loss 10.8
		{12, 15, 10.5, 12},  // fills at 12 and reaches both: stop loss at 10.8
		{12, 12, 12, 12},    // signal: take profit 14.4, stop loss 10.8
		{10, 10, 10, 10},    // fills at 10, past the stop loss: sells at 10
		{1, 1, 1, 1},
	}
	candles := make([]market.Candle, len(bars))
	for index, bar := range bars {
		candles[index] = testCandle(1, market.IntervalHour, backtestHour(index), bar.close)
		candles[index].Open, candles[index].High, candles[index].Low = bar.open, bar.high, bar.low
	}
	service := newBacktestService(t, newBacktestStore(market.IntervalHour, candles), Strategy{
		ID: 1, Name: "Range", Expression: "h_close > 5", TakeProfitExpression: "h_close * 1.2", StopLossExpression: "h_close * 0.9",
	})

	result, err := service.Backtest(context.Background(), 1, "BTCUSDT", time.Time{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	want := []Trade{
		{EntryTime: backtestHour(2), EntryPrice: 10, ExitTime: backtestHour(3), ExitPrice: 12, Buys: 1, Return: netReturn(DirectionLong, 10, 12), TakeProfit: 12, StopLoss: 9, Reason: ExitTakeProfit},
		{EntryTime: backtestHour(5), EntryPrice: 12, ExitTime: backtestHour(5), ExitPrice: 10.8, Buys: 1, Return: netReturn(DirectionLong, 12, 10.8), TakeProfit: 14.4, StopLoss: 10.8, Reason: ExitStopLoss},
		{EntryTime: backtestHour(7), EntryPrice: 10, ExitTime: backtestHour(7), ExitPrice: 10, Buys: 1, Return: netReturn(DirectionLong, 10, 10), TakeProfit: 14.4, StopLoss: 10.8, Reason: ExitStopLoss},
	}
	if !slices.EqualFunc(result.Trades, want, func(left, right Trade) bool {
		return tradeNear(left, right) && near(left.TakeProfit, right.TakeProfit) && near(left.StopLoss, right.StopLoss) && left.Reason == right.Reason
	}) {
		t.Fatalf("trades = %+v, want %+v", result.Trades, want)
	}
	wantFill := Fill{Signal: backtestHour(1), Time: backtestHour(2), Price: 10, Values: map[string]float64{"h_close": 10}}
	if fills := result.Trades[0].Fills; len(fills) != 1 || !fills[0].Signal.Equal(wantFill.Signal) || !fills[0].Time.Equal(wantFill.Time) ||
		fills[0].Price != wantFill.Price || !maps.Equal(fills[0].Values, wantFill.Values) {
		t.Fatalf("fills = %+v, want %+v", fills, wantFill)
	}
	if result.Stats.TakeProfits != 1 || result.Stats.StopLosses != 2 || result.Stats.ExitRules != 0 {
		t.Fatalf("stats = %+v", result.Stats)
	}
}

// Only candles from from to to are evaluated, while older ones still feed
// prev, and the open trade is valued at the close of the last evaluated
// candle.
func TestBacktestEvaluatesThePeriod(t *testing.T) {
	service := newBacktestService(t, newBacktestStore(market.IntervalHour, hourlyCloses(9, 9, 1, 9, 2, 9)), Strategy{
		ID: 1, Name: "Stack", Expression: "h_close > 5 && prev(h_close) > 0",
	})

	result, err := service.Backtest(context.Background(), 1, "BTCUSDT", backtestHour(2), backtestHour(4))
	if err != nil {
		t.Fatal(err)
	}
	want := []Trade{{EntryTime: backtestHour(4), EntryPrice: 2, ExitTime: backtestHour(4), ExitPrice: 2, Buys: 1, Open: true, Return: netReturn(DirectionLong, 2, 2)}}
	if !result.From.Equal(backtestHour(2)) || !result.To.Equal(backtestHour(4)) || !slices.EqualFunc(result.Trades, want, tradeNear) ||
		!slices.EqualFunc(result.Alerts, []time.Time{backtestHour(3)}, time.Time.Equal) {
		t.Fatalf("period %v to %v, trades %+v, alerts %v", result.From, result.To, result.Trades, result.Alerts)
	}
	if result.BuyAndHold == nil || !near(*result.BuyAndHold, netReturn(DirectionLong, 9, 2)) {
		t.Fatalf("buy and hold %v", result.BuyAndHold)
	}
}

// Position variables count the candles from the first fill: the trade fills
// at the open of hour 2 and sells at the open of hour 4, after the close of
// its second candle.
func TestBacktestExitsOnPositionVariables(t *testing.T) {
	service := newBacktestService(t, newBacktestStore(market.IntervalHour, hourlyCloses(1, 9, 9, 9, 9)), Strategy{
		ID: 1, Name: "Hold", Expression: "h_close > 5", ExitExpression: "bars_held >= 2 && pnl > -1",
	})

	result, err := service.Backtest(context.Background(), 1, "BTCUSDT", time.Time{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	// Hour 4 signals again after the exit, but its buy never fills.
	want := []Trade{{EntryTime: backtestHour(2), EntryPrice: 9, ExitTime: backtestHour(4), ExitPrice: 9, Buys: 1, Return: netReturn(DirectionLong, 9, 9)}}
	if !slices.EqualFunc(result.Trades, want, tradeNear) || !slices.EqualFunc(result.Alerts, []time.Time{backtestHour(1), backtestHour(4)}, time.Time.Equal) {
		t.Fatalf("trades = %+v, alerts %v", result.Trades, result.Alerts)
	}
}

// A calculated prev shift outside 1 to 500 fails the backtest at the first
// candle whose rule it decides: bars_held is 1 at the fill of hour 2.
func TestBacktestFailsOnAnInvalidPrevShift(t *testing.T) {
	service := newBacktestService(t, newBacktestStore(market.IntervalHour, hourlyCloses(1, 9, 9, 9, 9)), Strategy{
		ID: 1, Name: "Hold", Expression: "h_close > 5", ExitExpression: "prev(h_close, bars_held - 1) > 100",
	})
	_, err := service.Backtest(context.Background(), 1, "BTCUSDT", time.Time{}, time.Time{})
	var shift *ShiftError
	want := "invalid strategy: at the candle opening 2026-03-02 02:00 UTC, the exit rule: the prev shift is 0, not a whole number from 1 to 500"
	if !errors.Is(err, ErrInvalidArgument) || !errors.As(err, &shift) || err.Error() != want {
		t.Fatalf("Backtest() error = %v, want %q", err, want)
	}
}

func TestBacktestPnLExitUsesPercent(t *testing.T) {
	for _, test := range []struct {
		name   string
		exit   string
		closes []float64
	}{
		{name: "profit", exit: "pnl >= 10", closes: []float64{1, 9, 100, 105, 120, 120}},
		{name: "loss", exit: "pnl <= -10", closes: []float64{1, 9, 100, 95, 80, 80}},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := newBacktestService(t, newBacktestStore(market.IntervalHour, hourlyCloses(test.closes...)), Strategy{
				ID: 1, Name: "Percent", Expression: "h_close > 5", ExitExpression: test.exit,
			})
			result, err := service.Backtest(context.Background(), 1, "BTCUSDT", time.Time{}, time.Time{})
			if err != nil {
				t.Fatal(err)
			}
			// A 5% move must not exit; a 20% move signals the sell,
			// which fills at the next open. Reported returns stay fractional.
			want := []Trade{{EntryTime: backtestHour(2), EntryPrice: 100, ExitTime: backtestHour(5), ExitPrice: test.closes[5], Buys: 1, Return: netReturn(DirectionLong, 100, test.closes[5])}}
			if !slices.EqualFunc(result.Trades, want, tradeNear) {
				t.Fatalf("trades = %+v, want %+v", result.Trades, want)
			}
		})
	}
}

// Without an exit every entry signal buys and nothing sells: the trade stays
// open, valued at the last close, and counts in the net profit but not in
// the statistics. A buy signaled at the last candle never fills.
func TestBacktestWithoutExitHoldsAnOpenTrade(t *testing.T) {
	service := newBacktestService(t, newBacktestStore(market.IntervalHour, hourlyCloses(1, 9, 1, 9)), Strategy{ID: 1, Name: "Stack", Expression: "h_close > 5"})

	result, err := service.Backtest(context.Background(), 1, "BTCUSDT", time.Time{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	want := []Trade{{EntryTime: backtestHour(2), EntryPrice: 1, ExitTime: backtestHour(3), ExitPrice: 9, Buys: 1, Open: true, Return: netReturn(DirectionLong, 1, 9)}}
	if !slices.EqualFunc(result.Trades, want, tradeNear) || result.Stats.Count != 0 || !near(result.NetProfit, netReturn(DirectionLong, 1, 9)) {
		t.Fatalf("trades = %+v, stats %+v, net %v", result.Trades, result.Stats, result.NetProfit)
	}
	// Buying and holding enters at the open of hour 1; DCA buys at the
	// opens of hours 1 to 3: 9, 1, and 9.
	if result.BuyAndHold == nil || !near(*result.BuyAndHold, netReturn(DirectionLong, 9, 9)) || result.DCA == nil || !near(*result.DCA, netReturn(DirectionLong, 3/(1.0/9+1+1.0/9), 9)) {
		t.Fatalf("buy and hold %v, DCA %v", result.BuyAndHold, result.DCA)
	}
}

func TestBacktestReplayMatchesTrackedValuesAtTheLatestCandle(t *testing.T) {
	registry := testRegistry(t)
	indicators := testIndicators{entries: []scannerindicator.Entry{
		testEMA(t, registry, 1, market.IntervalHour, 100),
		testEMA(t, registry, 2, market.IntervalDay, 3),
	}}
	var hourly, other, daily []market.Candle
	for index := range 300 {
		open := backtestStart.Add(time.Duration(index) * time.Hour)
		hourly = append(hourly, testCandle(1, market.IntervalHour, open, 100+10*math.Sin(float64(index)/7)))
		other = append(other, testCandle(2, market.IntervalHour, open, 50+float64(index%13)))
	}
	for index := range 30 {
		daily = append(daily, testCandle(1, market.IntervalDay, backtestStart.AddDate(0, 0, index-18), 100+float64(index*index%17)))
	}
	store := newBacktestStore(market.IntervalHour, hourly)
	store.candles[market.IntervalHour][2] = other
	store.candles[market.IntervalDay] = map[int64][]market.Candle{1: daily}
	instruments := []Instrument{{ID: 1, Symbol: "BTCUSDT"}, {ID: 2, Symbol: "ETHUSDT"}}
	compiled, err := Compile(`h_ema_100 > prev(h_ema_100) && d_ema_3 < d_close && of("ETHUSDT", h_close) > 0`, Variables(indicators.entries))
	if err != nil {
		t.Fatal(err)
	}
	entry := Entry{Strategy: Strategy{ID: 1, Enabled: true}, Compiled: compiled, Interval: market.IntervalHour, reads: compiled.Reads()}
	now := backtestStart.Add(300 * time.Hour)

	replay, err := newReplay(registry, entry, instruments[0], instruments)
	if err != nil {
		t.Fatal(err)
	}
	replay.histories = store.candles
	replayed, warm, err := replay.at(now)
	if err != nil || !warm {
		t.Fatalf("at() warm = %v, error = %v", warm, err)
	}

	tracked := snapshot{reads: readsOf([]Entry{entry}, instruments, instruments), bySymbol: bySymbol(instruments), now: now}
	tracker, err := closedindicator.New(store, registry, subscriptionSource(tracked.reads.subscriptions), nil, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	changed := make(chan struct{}, 1)
	tracker.Listen(func(closedindicator.Change) {
		select {
		case changed <- struct{}{}:
		default:
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- tracker.Run(ctx) }()
	defer func() { cancel(); <-done }()
	deadline := time.After(10 * time.Second)
	for {
		var missing []closedindicator.Target
		if tracked.values, missing = tracker.Snapshot(tracked.reads.subscriptions); len(missing) == 0 {
			break
		}
		select {
		case <-changed:
		case <-deadline:
			t.Fatalf("tracker values missing: %v", missing)
		}
	}

	for _, read := range compiled.Reads() {
		want, wantOK := tracked.output(entry, read, 1)
		got, gotOK := replayed.output(entry, read, 1)
		if !wantOK || !gotOK {
			t.Fatalf("%s: tracked fresh = %v, replayed fresh = %v", read.Variable.Name, wantOK, gotOK)
		}
		wantValue, wantKnown := want.At(read.Shift)
		gotValue, gotKnown := got.At(read.Shift)
		if wantValue != gotValue || wantKnown != gotKnown || !wantKnown {
			t.Fatalf("%s shift %d of %q = %v (%v), tracked %v (%v)", read.Variable.Name, read.Shift, read.Symbol, gotValue, gotKnown, wantValue, wantKnown)
		}
	}
	wantResult, wantKnown, _ := tracked.match(compiled, entry, 1, nil)
	gotResult, gotKnown, _ := replayed.match(compiled, entry, 1, nil)
	if gotResult != wantResult || gotKnown != wantKnown || !wantKnown {
		t.Fatalf("replayed match = %v (%v), tracked %v (%v)", gotResult, gotKnown, wantResult, wantKnown)
	}
}

func assertReplay(t *testing.T, service *Service, id int64, interval market.CandleInterval, want Backtest) {
	t.Helper()
	got, err := service.Backtest(context.Background(), id, "BTCUSDT", time.Time{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Interval != interval || got.Symbol != "BTCUSDT" || !symbolReplayEqual(got, want) {
		t.Fatalf("Backtest() = %+v, want %s %+v", got, interval, want)
	}
}

func symbolReplayEqual(left, right Backtest) bool {
	return left.From.Equal(right.From) && left.To.Equal(right.To) && slices.EqualFunc(left.Alerts, right.Alerts, time.Time.Equal)
}

func near(left, right float64) bool { return math.Abs(left-right) < 1e-9 }

func tradeNear(left, right Trade) bool {
	return left.EntryTime.Equal(right.EntryTime) && left.ExitTime.Equal(right.ExitTime) && near(left.EntryPrice, right.EntryPrice) &&
		near(left.ExitPrice, right.ExitPrice) && left.Buys == right.Buys && left.Open == right.Open && near(left.Return, right.Return)
}

func assertStats(t *testing.T, name string, got, want TradeStats) {
	t.Helper()
	optional := func(left, right *float64) bool {
		return left == nil && right == nil || left != nil && right != nil && near(*left, *right)
	}
	if got.Count != want.Count || got.TakeProfits != want.TakeProfits || got.StopLosses != want.StopLosses || got.ExitRules != want.ExitRules ||
		!optional(got.AverageBars, want.AverageBars) || !optional(got.WinRate, want.WinRate) || !optional(got.AverageTrade, want.AverageTrade) ||
		!optional(got.AverageWin, want.AverageWin) || !optional(got.AverageLoss, want.AverageLoss) || !optional(got.ProfitFactor, want.ProfitFactor) {
		t.Fatalf("%s stats = %s, want %s", name, formatStats(got), formatStats(want))
	}
}

func formatStats(stats TradeStats) string {
	value := func(pointer *float64) string {
		if pointer == nil {
			return "nil"
		}
		return strconv.FormatFloat(*pointer, 'g', 6, 64)
	}
	return fmt.Sprintf("{count %d, win rate %s, average %s, win %s, loss %s, profit factor %s, bars %s, exits %d/%d/%d}", stats.Count,
		value(stats.WinRate), value(stats.AverageTrade), value(stats.AverageWin), value(stats.AverageLoss), value(stats.ProfitFactor),
		value(stats.AverageBars), stats.TakeProfits, stats.StopLosses, stats.ExitRules)
}

func backtestHour(index int) time.Time { return backtestStart.Add(time.Duration(index) * time.Hour) }

// hourlyCloses are BTCUSDT candles from backtestHour(0), each opening at
// its close.
func hourlyCloses(closes ...float64) []market.Candle {
	candles := make([]market.Candle, len(closes))
	for index, close := range closes {
		candles[index] = testCandle(1, market.IntervalHour, backtestHour(index), close)
	}
	return candles
}

func testCandle(instrumentID int64, interval market.CandleInterval, open time.Time, close float64) market.Candle {
	return market.Candle{
		InstrumentID: instrumentID, Interval: interval, OpenTime: open, CloseTime: interval.NextOpenTime(open).Add(-time.Millisecond),
		Open: close, High: close + 1, Low: close / 2, Close: close, Volume: 10,
	}
}

func testRegistry(t *testing.T) *indicator.Registry {
	t.Helper()
	registry, err := indicator.NewRegistry(append(indicatortalib.New(), candle.New())...)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func testEMA(t *testing.T, registry *indicator.Registry, id int64, interval market.CandleInterval, period int) scannerindicator.Entry {
	t.Helper()
	selection, err := registry.Normalize(indicator.Selection{Type: "ema", Parameters: indicator.Parameters{"period": period}})
	if err != nil {
		t.Fatal(err)
	}
	title := scannerindicator.IntervalPrefix(interval) + "-ema-" + strconv.Itoa(period)
	return scannerindicator.Entry{Indicator: scannerindicator.Indicator{ID: id, Interval: interval, Selection: selection}, Title: title, Outputs: []string{"ema"}}
}

func newBacktestService(t *testing.T, store *backtestStore, items ...Strategy) *Service {
	t.Helper()
	store.strategies = items
	service, err := NewService(store, testIndicators{}, testRegistry(t), 7, slog.New(slog.DiscardHandler), func([]int64) {})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	return service
}

type testIndicators struct{ entries []scannerindicator.Entry }

func (indicators testIndicators) List() []scannerindicator.Entry { return indicators.entries }
func (testIndicators) Preview(scannerindicator.Indicator) (scannerindicator.Entry, error) {
	return scannerindicator.Entry{}, errors.New("not configured")
}

type subscriptionSource []closedindicator.Subscription

func (source subscriptionSource) Subscriptions(context.Context) ([]closedindicator.Subscription, error) {
	return source, nil
}

// backtestStore holds BTCUSDT (1), the administrator's only favorite, and
// ETHUSDT (2), which is active but not a favorite. Methods the backtest does
// not use panic.
type backtestStore struct {
	Store
	strategies []Strategy
	candles    map[market.CandleInterval]map[int64][]market.Candle
}

func newBacktestStore(interval market.CandleInterval, candles []market.Candle) *backtestStore {
	return &backtestStore{candles: map[market.CandleInterval]map[int64][]market.Candle{interval: {1: candles}}}
}

func (store *backtestStore) ListStrategies(context.Context) ([]Strategy, error) {
	return store.strategies, nil
}

func (store *backtestStore) ListStrategyInstruments(context.Context, int64) ([]Instrument, error) {
	return []Instrument{{ID: 1, Symbol: "BTCUSDT"}}, nil
}

func (store *backtestStore) GetActiveInstrumentBySymbol(_ context.Context, symbol string) (market.Instrument, error) {
	id, ok := map[string]int64{"BTCUSDT": 1, "ETHUSDT": 2}[symbol]
	if !ok {
		return market.Instrument{}, market.ErrInstrumentNotFound
	}
	return market.Instrument{ID: id, Symbol: symbol, Active: true}, nil
}

func (store *backtestStore) ListLatestCandles(_ context.Context, ids []int64, interval market.CandleInterval, limit int) (map[int64][]market.Candle, error) {
	result := map[int64][]market.Candle{}
	for _, id := range ids {
		if candles := store.candles[interval][id]; len(candles) > 0 {
			result[id] = candles[max(0, len(candles)-limit):]
		}
	}
	return result, nil
}

// BenchmarkBacktest20000Candles replays an hourly EMA and candle expression
// over RetentionDepth synthetic candles of one coin.
func BenchmarkBacktest20000Candles(b *testing.B) {
	registry, err := indicator.NewRegistry(append(indicatortalib.New(), candle.New())...)
	if err != nil {
		b.Fatal(err)
	}
	selection, err := registry.Normalize(indicator.Selection{Type: "ema", Parameters: indicator.Parameters{"period": 100}})
	if err != nil {
		b.Fatal(err)
	}
	indicators := testIndicators{entries: []scannerindicator.Entry{{Indicator: scannerindicator.Indicator{ID: 1, Interval: market.IntervalHour, Selection: selection}, Title: "h-ema-100", Outputs: []string{"ema"}}}}
	hourly := make([]market.Candle, market.RetentionDepth)
	for index := range hourly {
		hourly[index] = testCandle(1, market.IntervalHour, backtestStart.Add(time.Duration(index)*time.Hour), 100+10*math.Sin(float64(index)/50))
	}
	store := newBacktestStore(market.IntervalHour, hourly)
	store.strategies = []Strategy{{ID: 1, Name: "Cross", Expression: "crosses_above(h_close, h_ema_100)"}}
	service, err := NewService(store, indicators, registry, 7, slog.New(slog.DiscardHandler), func([]int64) {})
	if err != nil {
		b.Fatal(err)
	}
	if err := service.Load(context.Background()); err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		if _, err := service.Backtest(context.Background(), 1, "BTCUSDT", time.Time{}, time.Time{}); err != nil {
			b.Fatal(err)
		}
	}
}

const signalHistoryCandles = 100

// signalHistory is hourly history whose first 101 closes alternate between
// 100 and 101, each true range 1, followed by the candles of future;
// the signal candle, index 100, closes at 100.
func signalHistory(future ...market.Candle) []market.Candle {
	history := make([]market.Candle, 0, signalHistoryCandles+1+len(future))
	for index := range signalHistoryCandles + 1 {
		price := 100.0 + float64(index%2)
		history = append(history, market.Candle{OpenTime: backtestHour(index), Open: price, Close: price, High: price, Low: price})
	}
	for index, candle := range future {
		candle.OpenTime = backtestHour(signalHistoryCandles + 1 + index)
		history = append(history, candle)
	}
	return history
}

// ohlc is a candle without its open time.
func ohlc(open, high, low, closing float64) market.Candle {
	return market.Candle{Open: open, High: high, Low: low, Close: closing}
}

// A long or short signal succeeds when its target comes before its stop; a
// candle reaching both fails unless its open already is at one of them, and
// the move counts only the stop candle's open. A sideways signal fails on any
// touch of a target.
func TestEvaluateSignal(t *testing.T) {
	stop := 0.01 // Each preceding true range is 1 at a signal close of 100.
	quiet := ohlc(100, 100.1, 99.9, 100)
	for _, test := range []struct {
		name      string
		direction Direction
		future    []market.Candle
		success   bool
		move      float64
	}{
		{"target first", DirectionLong, []market.Candle{ohlc(100, 105, 99.5, 102), ohlc(102, 106, 95, 96), quiet, quiet, quiet, quiet}, true, 0.05},
		{"stop first", DirectionLong, []market.Candle{ohlc(100, 100.5, 97, 98), ohlc(98, 110, 98, 109), quiet, quiet, quiet, quiet}, false, 0},
		{"both in one candle", DirectionLong, []market.Candle{ohlc(100.5, 106, 97, 100), quiet, quiet, quiet, quiet, quiet}, false, 0.005},
		{"open past the target", DirectionLong, []market.Candle{quiet, ohlc(105, 106, 97, 100), quiet, quiet, quiet, quiet}, true, 0.05},
		{"open past the stop", DirectionShort, []market.Candle{quiet, ohlc(103, 104, 94, 100), quiet, quiet, quiet, quiet}, false, 0.001},
		{"short target first", DirectionShort, []market.Candle{ohlc(100, 100.5, 95, 98), quiet, quiet, quiet, quiet, quiet}, true, 0.05},
		{"sideways quiet", DirectionSideways, []market.Candle{quiet, ohlc(100, 101.5, 99, 100), quiet, quiet, quiet, quiet}, true, 0.015},
		{"sideways touch", DirectionSideways, []market.Candle{quiet, quiet, ohlc(100, 100.5, 95, 99), quiet, quiet, quiet}, false, 0.05},
	} {
		t.Run(test.name, func(t *testing.T) {
			// The target lies 2 stops away, 2%.
			evaluation := evaluateTestSignal(t, test.direction, market.IntervalHour, signalHistory(test.future...), signalHistoryCandles, 6, 2)
			if evaluation == nil || evaluation.Success != test.success || math.Abs(evaluation.Move-test.move) > 1e-9 ||
				math.Abs(evaluation.Target-2*stop) > 1e-12 || (evaluation.Stop == nil) != (test.direction == DirectionSideways) {
				t.Fatalf("evaluation %+v, want success %v and move %v", evaluation, test.success, test.move)
			}
		})
	}
}

// A candle without enough preceding ranges for ATR or the whole window
// after it, or with a recent gap in either, has no evaluation.
func TestEvaluateSignalNeedsConsecutiveCandles(t *testing.T) {
	quiet := ohlc(100, 100.1, 99.9, 100)
	history := signalHistory(quiet, quiet, quiet, quiet, quiet, quiet)
	if evaluateTestSignal(t, DirectionLong, market.IntervalHour, history, 13, 6, 2) != nil {
		t.Fatal("evaluated without enough ranges for ATR")
	}
	if evaluateTestSignal(t, DirectionLong, market.IntervalHour, history[:len(history)-1], signalHistoryCandles, 6, 2) != nil {
		t.Fatal("evaluated without the whole window")
	}
	for _, at := range []int{signalHistoryCandles - 5, signalHistoryCandles + 3} {
		gapped := slices.Clone(history)
		for index := at; index < len(gapped); index++ {
			gapped[index].OpenTime = gapped[index].OpenTime.Add(time.Hour)
		}
		if evaluateTestSignal(t, DirectionLong, market.IntervalHour, gapped, signalHistoryCandles, 6, 2) != nil {
			t.Fatalf("evaluated across a gap at %d", at)
		}
	}
}

// evaluateTestSignal exercises ATR calculation and the outcome together.
func evaluateTestSignal(t *testing.T, direction Direction, interval market.CandleInterval, history []market.Candle, at, window, ratio int) *SignalEvaluation {
	t.Helper()
	report, err := signalReport(testRegistry(t), Strategy{Direction: direction, Window: window, TargetRatio: ratio}, interval, history, []SignalOccurrence{{}}, []int{at}, -1, -1)
	if err != nil {
		t.Fatal(err)
	}
	return report.Occurrences[0].Evaluation
}

// Neither future OHLC nor the window changes the stop fixed at the signal.
func TestSignalATRUsesPastRangesOnly(t *testing.T) {
	quiet := ohlc(100, 100.1, 99.9, 100)
	history := signalHistory(quiet, quiet, quiet, quiet, quiet, quiet)
	baseline := evaluateTestSignal(t, DirectionLong, market.IntervalHour, history, signalHistoryCandles, 3, 2)
	history[signalHistoryCandles+1].High = 500
	longer := evaluateTestSignal(t, DirectionLong, market.IntervalHour, history, signalHistoryCandles, 6, 2)
	if baseline == nil || longer == nil || *baseline.Stop != *longer.Stop || baseline.Target != longer.Target {
		t.Fatalf("future or window changed levels: %+v, %+v", baseline, longer)
	}
	// A shadow on the signal candle increases ATR with unchanged closes.
	history[signalHistoryCandles].High = 110
	wider := evaluateTestSignal(t, DirectionLong, market.IntervalHour, history, signalHistoryCandles, 6, 2)
	if wider == nil || *wider.Stop <= *baseline.Stop {
		t.Fatalf("ignored signal shadow: %+v, %+v", baseline, wider)
	}
}

// Retained history may have lost the volatility before its artificial seed.
// Both signal and all-candle outcomes must wait for the shared warm-up.
func TestSignalATRWarmsUpPrunedHistory(t *testing.T) {
	registry := testRegistry(t)
	depth, err := closedindicator.Depth(registry, closedindicator.Target{
		Interval:  market.IntervalHour,
		Selection: indicator.Selection{Type: "atr", Parameters: indicator.Parameters{"period": signalATRPeriod}},
	}, 1)
	if err != nil {
		t.Fatal(err)
	}
	history := make([]market.Candle, market.SyncDepth+1000)
	for index := range history {
		spread := 1.0
		if index < 10 {
			spread = 20
		}
		history[index] = ohlc(100, 100+spread, 100-spread, 100)
		history[index].OpenTime = backtestHour(index)
	}
	signal := Strategy{Direction: DirectionLong, Window: 6, TargetRatio: 2}
	signals := []int{14, depth - 2, depth - 1}
	report, err := signalReport(registry, signal, market.IntervalHour, history, make([]SignalOccurrence, len(signals)), signals, 0, len(history)-1)
	if err != nil {
		t.Fatal(err)
	}
	if report.Occurrences[0].Evaluation != nil || report.Occurrences[1].Evaluation != nil {
		t.Fatal("evaluated a signal before its retained-history ATR warm-up")
	}
	last := report.Occurrences[2].Evaluation
	if last == nil || !last.Counted || *last.Stop >= 0.03 || report.Evaluated != 1 {
		t.Fatalf("settled signal evaluation %+v, evaluated %d", last, report.Evaluated)
	}
	if want := len(history) - signal.Window - (depth - 1); report.All.Count != want {
		t.Fatalf("all-candle count %d, want %d warmed outcomes", report.All.Count, want)
	}
	// A newly listed instrument has no missing older history to settle from.
	if evaluateTestSignal(t, signal.Direction, market.IntervalHour, history[:depth-1], signals[0], signal.Window, signal.TargetRatio) == nil {
		t.Fatal("required retained-history warm-up for complete short history")
	}
}

// Counted signals are at least a window apart; the others are repeats.
func TestSignalReportCountsSignalsAWindowApart(t *testing.T) {
	quiet := ohlc(100, 100.1, 99.9, 100)
	future := make([]market.Candle, 20)
	for index := range future {
		future[index] = quiet
	}
	history := signalHistory(future...)
	signals := []int{100, 103, 106, 107, 112}
	occurrences := make([]SignalOccurrence, len(signals))
	report, err := signalReport(testRegistry(t), Strategy{Direction: DirectionSideways, TargetRatio: 2, Window: 6}, market.IntervalHour, history, occurrences, signals, 100, 105)
	if err != nil {
		t.Fatal(err)
	}
	var counted []bool
	for _, occurrence := range report.Occurrences {
		counted = append(counted, occurrence.Evaluation != nil && occurrence.Evaluation.Counted)
	}
	if !slices.Equal(counted, []bool{true, false, true, false, true}) || report.Evaluated != 5 || report.Signals.Count != 3 || report.Signals.Successes != 3 {
		t.Fatalf("counted %v, report %+v", counted, report)
	}
	if report.All.Count != 6 || report.All.MedianMove == nil || math.Abs(*report.All.MedianMove-0.001) > 1e-9 {
		t.Fatalf("all %+v", report.All)
	}
}
