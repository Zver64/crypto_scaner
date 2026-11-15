package strategy

import (
	"context"
	"fmt"
	"slices"
	"time"

	"crypto-scanner/internal/market"
)

// simulate replays the trades of entry on instrument over the candles of the
// result's interval opening from from to to, stepping each with the values
// replay calculates for it, and records the events of the steps in result.
// Signal occurrences show the values of displayed. observe, unless nil, sees
// replay after it calculated the values of each candle from from on.
func simulate(ctx context.Context, replay *replay, entry Entry, instrument Instrument, from, to time.Time, displayed []displayedIndicator, observe func(*replay), result *Backtest) error {
	history := replay.histories[result.Interval][instrument.ID]
	candles := history
	if !to.IsZero() {
		end, found := slices.BinarySearchFunc(candles, to, func(candle market.Candle, to time.Time) int { return candle.OpenTime.Compare(to) })
		if found {
			end++
		}
		candles = candles[:end]
	}
	var state TradeState
	// fills are the buys of the open trade, the last ones pending until
	// they fill.
	var fills []Fill
	// selling is the trade an exit rule sold, until its sell fills.
	var selling *Trade
	// signals are the indexes of the candles a signal signaled at.
	var signals []int
	var occurrences []SignalOccurrence
	first, candleCount := -1, 0
	equity, peak := 1.0, 1.0
	var returns, bars []float64
	var reasons []ExitReason
	drawdown := func(value float64) {
		peak = max(peak, value)
		result.MaxDrawdown = max(result.MaxDrawdown, 1-value/peak)
	}
	record := func(trade Trade) {
		trade.Return = netReturn(entry.Direction, trade.EntryPrice, trade.ExitPrice)
		result.Trades = append(result.Trades, trade)
		if !trade.Open {
			returns = append(returns, trade.Return)
			bars = append(bars, float64(result.Interval.CandlesBetween(trade.EntryTime, trade.ExitTime)))
			reasons = append(reasons, trade.Reason)
		}
		equity *= 1 + trade.Return
		drawdown(equity)
		result.Equity = append(result.Equity, EquityPoint{Time: trade.ExitTime, Equity: equity})
	}
	// filled are the fills of the trade; those still pending are left out.
	filled := func() []Fill {
		trade := slices.DeleteFunc(fills, func(fill Fill) bool { return fill.Time.IsZero() })
		fills = nil
		return trade
	}
	for index, candle := range candles {
		if err := ctx.Err(); err != nil {
			return err
		}
		if candle.OpenTime.Before(from) {
			continue
		}
		current, warm, err := replay.at(result.Interval.NextOpenTime(candle.OpenTime))
		if err != nil {
			return err
		}
		if observe != nil {
			observe(replay)
		}
		if !warm {
			continue
		}
		if first < 0 {
			first = index
			result.From = candle.OpenTime.UTC()
		}
		result.To = candle.OpenTime.UTC()
		candleCount++
		next, events, _, err := current.advance(entry, instrument.ID, state, false)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidArgument, err)
		}
		state = next
		for _, event := range events {
			switch event.Kind {
			case TradeBuyFill:
				for i := len(fills) - event.Count; i < len(fills); i++ {
					fills[i].Time, fills[i].Price = event.Time.UTC(), event.Price
				}
			case TradeSellFill:
				selling.ExitTime, selling.ExitPrice = event.Time.UTC(), event.Price
				record(*selling)
				selling = nil
			case TradeBuy:
				result.Alerts = append(result.Alerts, candle.OpenTime.UTC())
				fills = append(fills, Fill{Signal: candle.OpenTime.UTC(), Values: entry.Compiled.Values(current.resolver(entry, instrument.ID, nil))})
			case TradeSkip:
				result.Skipped++
			case TradeSignal:
				signals = append(signals, index)
				occurrences = append(occurrences, SignalOccurrence{
					Time: candle.OpenTime.UTC(), Close: candle.Close, Values: entry.Compiled.Values(current.resolver(entry, instrument.ID, nil)),
					IndicatorValues: current.indicatorValues(entry, instrument.ID, displayed),
				})
			case TradeSell:
				trade := Trade{
					EntryTime: event.Trade.OpenedAt, EntryPrice: event.Trade.EntryPrice(), Buys: event.Trade.Buys, Fills: filled(),
					TakeProfit: event.Trade.TakeProfit, StopLoss: event.Trade.StopLoss,
					Reason: event.Reason, ExitSignal: candle.OpenTime.UTC(),
					ExitTime: candle.OpenTime.UTC(), ExitPrice: event.Price,
				}
				if event.Reason != ExitRuleSignal {
					record(trade)
					continue
				}
				trade.ExitValues = entry.Exit.Values(current.resolver(entry, instrument.ID, entry.positions(event.Trade, candle.OpenTime, candle.Close)))
				// The sell fills with the next processed candle, unless the
				// period ends first.
				selling = &trade
			}
		}
		if state.Filled > 0 {
			drawdown(equity * (1 + netReturn(entry.Direction, state.EntryPrice(), candle.Close)))
		}
	}
	// The end of the period fills a pending sell, and values an open trade,
	// at its last close.
	if selling != nil {
		last := candles[len(candles)-1]
		selling.ExitTime, selling.ExitPrice = last.OpenTime.UTC(), last.Close
		record(*selling)
	}
	if state.Filled > 0 {
		last := candles[len(candles)-1]
		record(Trade{
			EntryTime: state.OpenedAt, EntryPrice: state.EntryPrice(), Buys: state.Filled, Fills: filled(),
			TakeProfit: state.TakeProfit, StopLoss: state.StopLoss,
			ExitTime: last.OpenTime.UTC(), ExitPrice: last.Close, Open: true,
		})
	}
	result.NetProfit = equity - 1
	result.Stats = tradeStats(returns, bars, reasons)
	if !entry.Signal {
		if entry.Direction != DirectionShort {
			result.baselines(candles, first)
		}
		return nil
	}
	result.Signal = &SignalReport{Occurrences: occurrences, Candles: candleCount}
	if entry.Direction == DirectionSimple {
		return nil
	}
	var err error
	result.Signal.Judgement, err = replay.judgeSignal(entry.Strategy, result.Interval, history, occurrences, signals, first, len(candles)-1)
	return err
}
