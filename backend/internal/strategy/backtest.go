package strategy

import (
	"context"
	"fmt"
	"math"
	"slices"
	"time"

	"crypto-scanner/internal/closedindicator"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
)

// BacktestFee is the fee of each side of a simulated trade, as a fraction of
// the traded value.
const BacktestFee = 0.001

// Backtest is how a strategy would have traded one instrument over its
// stored closed history: the monitor's trades replayed candle by candle.
// Every buy spends one quote unit at the open of the candle after its
// signal. A take profit or stop loss sells every buy of the trade on the
// candle that reaches it, and an exit rule at the open of the candle after
// its signal, paying BacktestFee on both sides.
type Backtest struct {
	// Interval is the finest interval the rules read, where signals fire
	// and trades count candles.
	Interval market.CandleInterval
	Symbol   string
	// Direction is the strategy's: how it trades or what a signal expects.
	Direction Direction
	// From and To are the open times of the first and the last evaluated
	// candles; both are zero when none is evaluated.
	From, To time.Time
	// Alerts are the open times of the candles whose close signaled a buy,
	// oldest first.
	Alerts []time.Time
	// Trades are the trades, oldest first; the last one is open when the
	// history ends before its sell. Skipped counts entry signals that bought
	// nothing.
	Trades  []Trade
	Skipped int
	// Stats describes the closed trades. NetProfit, Equity, and MaxDrawdown
	// include the open trade, valued at the last close: the equity compounds
	// from 1 through the trades, and the drawdown is its largest fall from an
	// earlier peak, as a fraction of that peak, at the close of every
	// evaluated candle, with an open trade valued there.
	Stats                  TradeStats
	NetProfit, MaxDrawdown float64
	Equity                 []EquityPoint
	// BuyAndHold is the net return from the open of the candle after the
	// first evaluated candle to the close of the last; DCA the net return of
	// buying one quote unit at the open after every evaluated candle but the
	// last, valued at the close of the last. Both are nil without such
	// candles, for a signal, and for a short strategy.
	BuyAndHold, DCA *float64
	// Signal describes the entry signals of a signal, which trades nothing;
	// nil for a trading strategy.
	Signal *SignalReport
}

// SignalWindows are the counts of candles after a signal over which a
// backtest measures how the price moved.
var SignalWindows = []int{3, 6, 12, 24}

// SignalReport is how the price moved after the entry signals of a signal,
// measured from the close of each signal candle over each of SignalWindows,
// beside the same measure after every evaluated candle.
type SignalReport struct {
	// Occurrences are the entry signals, oldest first.
	Occurrences []SignalOccurrence
	Windows     []SignalWindow
}

// SignalOccurrence is an entry signal: Time is the open time of the candle
// whose close signaled it, Close that close, and Values what the entry rule
// read there. Changes holds, for each of SignalWindows, the change from that
// close to the close as many candles later, nil when the stored history ends
// before it.
type SignalOccurrence struct {
	Time    time.Time
	Close   float64
	Values  map[string]float64
	Changes []*float64
}

// SignalWindow compares the moves over the Candles candles after the entry
// signals with those after every evaluated candle. Outcomes may read stored
// candles after the evaluated period; a candle without the Candles later
// candles stored, without a gap, is left out of this window.
type SignalWindow struct {
	Candles      int
	Signals, All SignalStats
}

// SignalStats describes the moves after Count candles: the medians of Rise,
// the highest high over the close minus 1, Fall, the lowest low over the
// close minus 1, and Range, the highest high minus the lowest low over the
// close, and Hits, the share of moves the signal expected: a rise above the
// fall for long, a fall below the rise for short, and for sideways a range
// below the median range after every evaluated candle. All are nil without
// moves.
type SignalStats struct {
	Count                   int
	Rise, Fall, Range, Hits *float64
}

// Trade is a trade of one or more buys. EntryTime is the open time of the
// candle its first buy filled at and EntryPrice the average price of its
// buys; ExitTime is the open time of the candle it sold at, or, for an open
// trade, of the last candle, whose close values it. Return is net of fees.
// TakeProfit and StopLoss are the prices fixed at its entry signal, 0
// without them; Reason tells what sold a closed trade, ExitSignal is the open
// time of the candle that signaled the sell, and ExitValues what an exit
// rule read there.
type Trade struct {
	EntryTime, ExitTime   time.Time
	EntryPrice, ExitPrice float64
	Buys                  int
	Open                  bool
	Return                float64
	Fills                 []Fill
	TakeProfit, StopLoss  float64
	Reason                ExitReason
	ExitSignal            time.Time
	ExitValues            map[string]float64
}

// Fill is one buy of a trade: Signal is the open time of the candle whose
// close signaled it, Values what the entry rule read there, and Time and
// Price the open time and the open of the candle it filled at.
type Fill struct {
	Signal, Time time.Time
	Price        float64
	Values       map[string]float64
}

// EquityPoint is the equity after the trade that exited at Time.
type EquityPoint struct {
	Time   time.Time
	Equity float64
}

// TradeStats describes the net returns of trades. WinRate, AverageTrade, and
// AverageBars, the mean count of candles from the one the first buy filled
// at through the one the trade sold at, are nil
// without trades, AverageWin without wins, and AverageLoss and ProfitFactor,
// the sum of winning returns over the sum of losing ones, without losses.
// TakeProfits, StopLosses, and ExitRules count the trades each sold.
type TradeStats struct {
	Count                                                        int
	WinRate, ProfitFactor, AverageTrade, AverageWin, AverageLoss *float64
	AverageBars                                                  *float64
	TakeProfits, StopLosses, ExitRules                           int
}

// Backtest replays the trades of a saved strategy, enabled or not, on the
// instrument symbol over its stored closed history, the way the monitor
// trades: the same values at the close of every candle of its interval, the
// same freshness, and the same signals, starting without a trade and with a
// false entry. Candles are evaluated once every value is calculated over the
// whole window the tracker loads, so the replay never reads indicators the
// pruned history leaves unsettled; coins read through of are read only among
// the administrator's favorites, like the monitor does. Only candles opening
// from from to to are evaluated, a zero bound leaving that side open; older
// candles still warm the indicators up. It fails with ErrNotFound,
// ErrInvalidArgument for a strategy that no longer compiles, an empty
// symbol, or from after to, and market.ErrInstrumentNotFound.
func (service *Service) Backtest(ctx context.Context, id int64, symbol string, from, to time.Time) (Backtest, error) {
	if !from.IsZero() && !to.IsZero() && from.After(to) {
		return Backtest{}, fmt.Errorf("%w: the period starts after it ends", ErrInvalidArgument)
	}
	entries := service.List()
	index := slices.IndexFunc(entries, func(entry Entry) bool { return entry.ID == id })
	if index < 0 {
		return Backtest{}, ErrNotFound
	}
	entry := entries[index]
	if entry.Compiled == nil {
		return Backtest{}, fmt.Errorf("%w: %s", ErrInvalidArgument, entry.Problem)
	}
	if symbol = market.NormalizeSymbol(symbol); symbol == "" {
		return Backtest{}, fmt.Errorf("%w: the symbol is empty", ErrInvalidArgument)
	}
	found, err := service.store.GetActiveInstrumentBySymbol(ctx, symbol)
	if err != nil {
		return Backtest{}, fmt.Errorf("resolve backtest instrument %s: %w", symbol, err)
	}
	instrument := Instrument{ID: found.ID, Symbol: found.Symbol}
	favorites, err := service.store.ListStrategyInstruments(ctx, service.administratorID)
	if err != nil {
		return Backtest{}, fmt.Errorf("load backtest favorites: %w", err)
	}
	replay, err := newReplay(service.registry, entry, instrument, favorites)
	if err != nil {
		return Backtest{}, err
	}
	ids := map[market.CandleInterval][]int64{}
	for _, subscription := range replay.current.reads.subscriptions {
		if read := subscription.Target.Interval; !slices.Contains(ids[read], subscription.InstrumentID) {
			ids[read] = append(ids[read], subscription.InstrumentID)
		}
	}
	replay.histories = make(map[market.CandleInterval]map[int64][]market.Candle, len(ids))
	for read, readIDs := range ids {
		if replay.histories[read], err = service.store.ListLatestCandles(ctx, readIDs, read, market.RetentionDepth); err != nil {
			return Backtest{}, fmt.Errorf("load backtest %s history: %w", read, err)
		}
	}
	result := Backtest{Interval: entry.Interval, Symbol: instrument.Symbol, Direction: entry.Direction}
	if err := replay.run(ctx, entry, instrument, from, to, &result); err != nil {
		return Backtest{}, err
	}
	return result, nil
}

// run replays the trades of entry on instrument over the candles of the
// result's interval opening from from to to.
func (replay *replay) run(ctx context.Context, entry Entry, instrument Instrument, from, to time.Time, result *Backtest) error {
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
	// their candle opens.
	var fills []Fill
	// signals are the indexes of the candles a signal signaled at.
	var signals []int
	var occurrences []SignalOccurrence
	first := -1
	equity, peak := 1.0, 1.0
	var returns, bars []float64
	var reasons []ExitReason
	drawdown := func(value float64) {
		peak = max(peak, value)
		result.MaxDrawdown = max(result.MaxDrawdown, 1-value/peak)
	}
	record := func(trade Trade) {
		trade.Return = netReturn(entry.Direction, trade.EntryPrice, trade.ExitPrice)
		trade.Fills = slices.DeleteFunc(fills, func(fill Fill) bool { return fill.Time.IsZero() })
		fills = nil
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
		if !warm {
			continue
		}
		if first < 0 {
			first = index
			result.From = candle.OpenTime.UTC()
		}
		result.To = candle.OpenTime.UTC()
		pending := state.Buys > state.Filled
		next, events, processed := current.advance(entry, instrument.ID, state, false)
		if processed && pending {
			for i := range fills {
				if fills[i].Time.IsZero() {
					fills[i].Time, fills[i].Price = candle.OpenTime.UTC(), candle.Open
				}
			}
		}
		state = next
		for _, event := range events {
			switch event.Kind {
			case TradeBuy:
				result.Alerts = append(result.Alerts, candle.OpenTime.UTC())
				fills = append(fills, Fill{Signal: candle.OpenTime.UTC(), Values: entry.Compiled.Values(current.resolver(entry, instrument.ID, nil))})
			case TradeSkip:
				result.Skipped++
			case TradeSignal:
				signals = append(signals, index)
				occurrences = append(occurrences, SignalOccurrence{
					Time: candle.OpenTime.UTC(), Close: candle.Close, Values: entry.Compiled.Values(current.resolver(entry, instrument.ID, nil)),
					Changes: signalChanges(result.Interval, history, index),
				})
			case TradeSell:
				trade := Trade{
					EntryTime: event.Trade.OpenedAt, EntryPrice: event.Trade.EntryPrice(), Buys: event.Trade.Buys,
					TakeProfit: event.Trade.TakeProfit, StopLoss: event.Trade.StopLoss,
					Reason: event.Reason, ExitSignal: candle.OpenTime.UTC(),
					ExitTime: candle.OpenTime.UTC(), ExitPrice: event.Price,
				}
				if event.Reason == ExitRuleSignal {
					trade.ExitValues = entry.Exit.Values(current.resolver(entry, instrument.ID, entry.positions(event.Trade, candle.OpenTime, candle.Close)))
					// The sell fills at the next open, or at this close when
					// the history ends here.
					trade.ExitPrice = candle.Close
					if index+1 < len(candles) && candles[index+1].Open > 0 {
						trade.ExitTime, trade.ExitPrice = candles[index+1].OpenTime.UTC(), candles[index+1].Open
					}
				}
				record(trade)
			}
		}
		if state.Filled > 0 {
			drawdown(equity * (1 + netReturn(entry.Direction, state.EntryPrice(), candle.Close)))
		}
	}
	if state.Filled > 0 {
		last := candles[len(candles)-1]
		record(Trade{
			EntryTime: state.OpenedAt, EntryPrice: state.EntryPrice(), Buys: state.Filled,
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
	result.Signal = &SignalReport{Occurrences: occurrences}
	if first >= 0 {
		result.Signal.Windows = signalWindows(entry.Direction, result.Interval, history, signals, first, len(candles)-1)
	}
	return nil
}

// signalWindows measures the moves of history, candles of interval, after
// the candles at signals and after every evaluated candle, from first through
// last, over each of SignalWindows.
func signalWindows(direction Direction, interval market.CandleInterval, history []market.Candle, signals []int, first, last int) []SignalWindow {
	all, after := make([][]signalMove, len(SignalWindows)), make([][]signalMove, len(SignalWindows))
	collect := func(moves [][]signalMove, at int) {
		for index, move := range signalMovesAt(interval, history, at) {
			moves[index] = append(moves[index], move)
		}
	}
	for at := first; at <= last; at++ {
		collect(all, at)
	}
	for _, at := range signals {
		collect(after, at)
	}
	windows := make([]SignalWindow, len(SignalWindows))
	for index, size := range SignalWindows {
		every := signalStats(direction, all[index], nil)
		windows[index] = SignalWindow{Candles: size, Signals: signalStats(direction, after[index], every.Range), All: every}
	}
	return windows
}

// signalChanges are the changes from the close of the candle of history, of
// interval, at at to the close each of SignalWindows later, nil past the
// history or across a gap in it.
func signalChanges(interval market.CandleInterval, history []market.Candle, at int) []*float64 {
	changes := make([]*float64, len(SignalWindows))
	for index, size := range SignalWindows {
		if signalWindowStored(interval, history, at, size) {
			changes[index] = new(history[at+size].Close/history[at].Close - 1)
		}
	}
	return changes
}

// signalWindowStored reports whether history, candles of interval, holds the
// size candles right after the one at at, without a gap, and that candle has
// a close to measure from.
func signalWindowStored(interval market.CandleInterval, history []market.Candle, at, size int) bool {
	return at+size < len(history) && history[at].Close > 0 &&
		interval.CandlesBetween(history[at].OpenTime, history[at+size].OpenTime) == size+1
}

// signalMove is how far the price rose, fell, and ranged after a candle, as
// fractions of its close.
type signalMove struct {
	rise, fall, size float64
}

// signalMovesAt measures the moves over each of SignalWindows after the
// candle of history at at, in their order, extending one running high and
// low; it stops at the first window without its candles stored, since the
// longer ones lack them too.
func signalMovesAt(interval market.CandleInterval, history []market.Candle, at int) []signalMove {
	var moves []signalMove
	closing := history[at].Close
	high, low := 0.0, math.Inf(1)
	next := at + 1
	for _, size := range SignalWindows {
		if !signalWindowStored(interval, history, at, size) {
			break
		}
		for _, candle := range history[next : at+size+1] {
			high, low = max(high, candle.High), min(low, candle.Low)
		}
		next = at + size + 1
		moves = append(moves, signalMove{rise: high/closing - 1, fall: low/closing - 1, size: (high - low) / closing})
	}
	return moves
}

// signalStats describes moves, counting hits by direction; a sideways move
// hits when its range is below typical, or below the median range of moves
// when typical is nil.
func signalStats(direction Direction, moves []signalMove, typical *float64) SignalStats {
	stats := SignalStats{Count: len(moves)}
	if len(moves) == 0 {
		return stats
	}
	rises, falls, sizes := make([]float64, len(moves)), make([]float64, len(moves)), make([]float64, len(moves))
	for index, move := range moves {
		rises[index], falls[index], sizes[index] = move.rise, move.fall, move.size
	}
	stats.Rise, stats.Fall, stats.Range = median(rises), median(falls), median(sizes)
	if typical == nil {
		typical = stats.Range
	}
	hits := 0
	for _, move := range moves {
		switch {
		case direction == DirectionLong && move.rise > -move.fall,
			direction == DirectionShort && -move.fall > move.rise,
			direction == DirectionSideways && typical != nil && move.size < *typical:
			hits++
		}
	}
	stats.Hits = new(float64(hits) / float64(len(moves)))
	return stats
}

// median is the median of values, nil without values; it sorts values.
func median(values []float64) *float64 {
	if len(values) == 0 {
		return nil
	}
	slices.Sort(values)
	middle := len(values) / 2
	if len(values)%2 == 0 {
		return new((values[middle-1] + values[middle]) / 2)
	}
	return new(values[middle])
}

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

// replay calculates the values a strategy reads at past times, each over the
// window of stored candles the tracker would have loaded then.
type replay struct {
	registry *indicator.Registry
	// histories hold the subscribed pairs and are only read.
	histories map[market.CandleInterval]map[int64][]market.Candle
	current   snapshot
	depths    []int
	// opens are the open times the values were calculated at, so a coarser
	// interval is calculated once per its candle; cut marks values left
	// unknown because the stored history ends inside their window.
	opens []time.Time
	cut   []bool
}

// newReplay reads entry, its candles included, on instrument and on the
// favorites it reads through of; histories, holding the subscribed pairs, is
// set before at is called.
func newReplay(registry *indicator.Registry, entry Entry, instrument Instrument, favorites []Instrument) (*replay, error) {
	reads := readsOf([]Entry{entry}, []Instrument{instrument}, favorites)
	depths := make([]int, len(reads.subscriptions))
	for index, subscription := range reads.subscriptions {
		depth, err := closedindicator.Depth(registry, subscription.Target, subscription.Points)
		if err != nil {
			return nil, err
		}
		depths[index] = depth
	}
	return &replay{
		registry: registry, depths: depths,
		current: snapshot{reads: reads, values: make([]closedindicator.Value, len(reads.subscriptions)), bySymbol: bySymbol(favorites)},
		opens:   make([]time.Time, len(reads.subscriptions)),
		cut:     make([]bool, len(reads.subscriptions)),
	}, nil
}

// at returns the values the monitor would have evaluated at now, valid until
// the next call. A window that reaches before the stored history leaves its
// value unknown and warm false when the history holds at least
// market.SyncDepth candles: older candles existed then but may be missing or
// pruned now, whether the history was synchronized, loaded deeper on demand,
// or grew since. A shorter history is the whole history of the instrument,
// which the tracker read the same way.
func (replay *replay) at(now time.Time) (snapshot, bool, error) {
	for index, subscription := range replay.current.reads.subscriptions {
		open := subscription.Target.Interval.LastClosedOpenTime(now)
		if replay.opens[index].Equal(open) {
			continue
		}
		replay.opens[index] = open
		candles := replay.histories[subscription.Target.Interval][subscription.InstrumentID]
		end, found := slices.BinarySearchFunc(candles, open, func(candle market.Candle, open time.Time) int { return candle.OpenTime.Compare(open) })
		if found {
			end++
		}
		if replay.cut[index] = end < replay.depths[index] && len(candles) >= market.SyncDepth; replay.cut[index] {
			replay.current.values[index] = closedindicator.Value{Target: subscription.Target}
			continue
		}
		value, err := closedindicator.Calculate(replay.registry, subscription.Target, candles[max(0, end-replay.depths[index]):end], subscription.Points)
		if err != nil {
			return snapshot{}, false, err
		}
		replay.current.values[index] = value
	}
	replay.current.now = now
	return replay.current, !slices.Contains(replay.cut, true), nil
}
