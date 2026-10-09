package strategy

import (
	"context"
	"fmt"
	"slices"
	"strings"
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

// signalATRPeriod is the common ATR period used to judge every signal.
const signalATRPeriod = 14

// SignalReport judges the entry signals of a signal by a target and a stop
// over the Window candles after each, beside the same judgment after every
// evaluated candle. A candle's stop lies one ATR(14) away from its close,
// against the expected move, independent of Window. The target lies TargetRatio
// stops away in the expected direction; a sideways signal has targets on both sides and
// no stop. Outcomes may read stored candles after the evaluated period.
type SignalReport struct {
	Window, TargetRatio int
	// Occurrences are the entry signals, oldest first.
	Occurrences []SignalOccurrence
	// Evaluated counts the entry signals with an evaluation; Signals
	// describes the counted ones and All every evaluated candle with an
	// evaluation.
	Evaluated    int
	Signals, All SignalStats
}

// SignalOccurrence is an entry signal: Time is the open time of the candle
// whose close signaled it, Close that close, and Values what the entry rule
// read there. Evaluation is nil when the stored history lacks the candles to
// judge it.
type SignalOccurrence struct {
	Time       time.Time
	Close      float64
	Values     map[string]float64
	Evaluation *SignalEvaluation
}

// SignalEvaluation judges a candle's close by its target and stop. Stop and
// Target are their distances from the close as fractions of it, Stop nil for
// sideways. A long or short one succeeds when the target is reached before
// the stop; a sideways one when neither target is touched. Move is the
// largest move from the close in the expected direction until the stop is
// reached or the window ends, and for sideways the largest move either way.
// Counted tells a signal counted in the statistics: one at least Window
// candles after the previous counted one.
type SignalEvaluation struct {
	Counted      bool
	Stop         *float64
	Target, Move float64
	Success      bool
}

// SignalStats counts evaluated candles and their successes, with the median
// move, nil without candles.
type SignalStats struct {
	Count, Successes int
	MedianMove       *float64
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

// maxBacktests bounds the backtests that run at once, saved or drafts; each
// keeps a core busy for up to the server's time limit.
const maxBacktests = 2

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
// symbol, or from after to, market.ErrInstrumentNotFound, and
// ErrBacktestBusy while maxBacktests others run.
func (service *Service) Backtest(ctx context.Context, id int64, symbol string, from, to time.Time) (Backtest, error) {
	entries := service.List()
	index := slices.IndexFunc(entries, func(entry Entry) bool { return entry.ID == id })
	if index < 0 {
		return Backtest{}, ErrNotFound
	}
	entry := entries[index]
	if entry.Compiled == nil {
		return Backtest{}, fmt.Errorf("%w: %s", ErrInvalidArgument, entry.Problem)
	}
	return service.backtest(ctx, entry, symbol, from, to)
}

// BacktestDraft replays item like Backtest replays a saved strategy, without
// saving it: its rules and trading settings are checked and compiled as a
// save checks them, including that every coin read through of is an active
// favorite of the administrator, while its name, message, and market cap
// range, which backtests ignore, are not. It fails like Backtest, with
// ErrInvalidArgument for an invalid item.
func (service *Service) BacktestDraft(ctx context.Context, item Strategy, symbol string, from, to time.Time) (Backtest, error) {
	entry, err := service.trading(item)
	if err != nil {
		return Backtest{}, err
	}
	if len(entry.symbols) > 0 {
		favorites, err := service.store.ListStrategyInstruments(ctx, service.administratorID)
		if err != nil {
			return Backtest{}, fmt.Errorf("load backtest favorites: %w", err)
		}
		absent := slices.DeleteFunc(slices.Clone(entry.symbols), func(read string) bool {
			return slices.ContainsFunc(favorites, func(favorite Instrument) bool { return favorite.Symbol == read })
		})
		if len(absent) > 0 {
			return Backtest{}, fmt.Errorf("%w: not an active coin in the administrator's favorites: %s", ErrInvalidArgument, strings.Join(absent, ", "))
		}
	}
	return service.backtest(ctx, entry, symbol, from, to)
}

// backtest replays the compiled entry for Backtest and BacktestDraft.
func (service *Service) backtest(ctx context.Context, entry Entry, symbol string, from, to time.Time) (Backtest, error) {
	if !from.IsZero() && !to.IsZero() && from.After(to) {
		return Backtest{}, fmt.Errorf("%w: the period starts after it ends", ErrInvalidArgument)
	}
	if symbol = market.NormalizeSymbol(symbol); symbol == "" {
		return Backtest{}, fmt.Errorf("%w: the symbol is empty", ErrInvalidArgument)
	}
	select {
	case service.backtests <- struct{}{}:
		defer func() { <-service.backtests }()
	default:
		return Backtest{}, ErrBacktestBusy
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
		next, events, processed, err := current.advance(entry, instrument.ID, state, false)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidArgument, err)
		}
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
	var err error
	result.Signal, err = signalReport(replay.registry, entry.Strategy, result.Interval, history, occurrences, signals, first, len(candles)-1)
	return err
}

// signalReport judges the occurrences of signal, at the indexes signals of
// history, candles of interval, and every evaluated candle, from first
// through last (none when first is negative).
func signalReport(registry *indicator.Registry, signal Strategy, interval market.CandleInterval, history []market.Candle, occurrences []SignalOccurrence, signals []int, first, last int) (*SignalReport, error) {
	// Use the existing TA-Lib module, once over each contiguous history run.
	// ATR at a candle reads only that candle and its preceding history.
	selection := indicator.Selection{Type: "atr", Parameters: indicator.Parameters{"period": signalATRPeriod}}
	depth, err := closedindicator.Depth(registry, closedindicator.Target{Interval: interval, Selection: selection}, 1)
	if err != nil {
		return nil, fmt.Errorf("signal ATR warm-up: %w", err)
	}
	calculated, err := registry.CalculateCandles(interval, history, []indicator.Selection{selection})
	if err != nil {
		return nil, fmt.Errorf("calculate signal ATR: %w", err)
	}
	atr := make(map[time.Time]float64, len(history))
	for _, series := range calculated[0].Series {
		for _, point := range series.Points {
			atr[point.Time] = point.Value
		}
	}
	// Match replay.at: a retained history may lack older seed candles;
	// a short history is the instrument's complete history and needs only
	// the indicator's minimum lookback. Apply this to both statistics alike.
	evaluate := func(at int) *SignalEvaluation {
		if len(history) >= market.SyncDepth && at+1 < depth {
			return nil
		}
		return evaluateSignal(signal.Direction, interval, history, at, signal.Window, signal.TargetRatio, atr[history[at].OpenTime.UTC()])
	}
	report := &SignalReport{Window: signal.Window, TargetRatio: signal.TargetRatio, Occurrences: occurrences}
	var moves []float64
	next := 0
	for index, at := range signals {
		evaluation := evaluate(at)
		if evaluation == nil {
			continue
		}
		report.Evaluated++
		if at >= next {
			evaluation.Counted = true
			next = at + signal.Window
			report.Signals.Count++
			if evaluation.Success {
				report.Signals.Successes++
			}
			moves = append(moves, evaluation.Move)
		}
		report.Occurrences[index].Evaluation = evaluation
	}
	report.Signals.MedianMove = median(moves)
	if first < 0 {
		return report, nil
	}
	moves = nil
	for at := first; at <= last; at++ {
		if evaluation := evaluate(at); evaluation != nil {
			report.All.Count++
			if evaluation.Success {
				report.All.Successes++
			}
			moves = append(moves, evaluation.Move)
		}
	}
	report.All.MedianMove = median(moves)
	return report, nil
}

// evaluateSignal judges the close of the candle of history, candles of
// interval, at at by a target ratio stops away in direction over the window
// candles after it. It is nil without a positive ATR or the consecutive future
// window, or when a level would not be a positive price.
func evaluateSignal(direction Direction, interval market.CandleInterval, history []market.Candle, at, window, ratio int, atr float64) *SignalEvaluation {
	if at < signalATRPeriod || at+window >= len(history) ||
		!consecutive(interval, history[at:at+window+1]) {
		return nil
	}
	price := history[at].Close
	stop := atr / price
	target := float64(ratio) * stop
	if !(price > 0) || !(stop > 0 && stop < 1) || (direction != DirectionLong && target >= 1) {
		return nil
	}
	future := history[at+1 : at+window+1]
	if direction == DirectionSideways {
		return sidewaysEvaluation(price, target, future)
	}
	evaluation := &SignalEvaluation{Stop: new(stop), Target: target}
	evaluation.Success, evaluation.Move = directionalOutcome(direction, price, stop, target, future)
	return evaluation
}

// directionalOutcome reports whether the price, from price, reached the
// target, target times price away in direction, before the stop, stop times
// price away against it, over future, and the largest move in direction until
// the stop. A candle reaching both decides by its open when that is already
// at one of them, and fails otherwise; the stop candle's move counts only its
// open, since its high and low may come after the stop.
func directionalOutcome(direction Direction, price, stop, target float64, future []market.Candle) (bool, float64) {
	sign := 1.0
	if direction == DirectionShort {
		sign = -1
	}
	// favorable is the move to value in direction, as a fraction of price.
	favorable := func(value float64) float64 { return sign * (value - price) / price }
	decided, success, move := false, false, 0.0
	for _, candle := range future {
		best, worst := candle.High, candle.Low
		if direction == DirectionShort {
			best, worst = candle.Low, candle.High
		}
		openAtTarget, openAtStop := favorable(candle.Open) >= target, favorable(candle.Open) <= -stop
		reachesTarget, reachesStop := favorable(best) >= target, favorable(worst) <= -stop
		if !decided {
			switch {
			case openAtTarget:
				decided, success = true, true
			case openAtStop, reachesTarget && reachesStop:
				decided = true
			case reachesTarget:
				decided, success = true, true
			case reachesStop:
				decided = true
			}
		}
		if reachesStop || openAtStop {
			if !openAtStop {
				move = max(move, favorable(candle.Open))
			}
			break
		}
		move = max(move, favorable(best))
	}
	return success, move
}

// sidewaysEvaluation judges a sideways signal at price with targets target
// times price above and below it over future: it succeeds when no candle
// touches either, and its move is the largest deviation either way.
func sidewaysEvaluation(price, target float64, future []market.Candle) *SignalEvaluation {
	evaluation := &SignalEvaluation{Target: target, Success: true}
	for _, candle := range future {
		up, down := (candle.High-price)/price, (price-candle.Low)/price
		if up >= target || down >= target {
			evaluation.Success = false
		}
		evaluation.Move = max(evaluation.Move, up, down)
	}
	return evaluation
}

// consecutive reports whether every candle of candles, of interval, opens one
// interval after the one before.
func consecutive(interval market.CandleInterval, candles []market.Candle) bool {
	for index := 1; index < len(candles); index++ {
		if !interval.NextOpenTime(candles[index-1].OpenTime).Equal(candles[index].OpenTime) {
			return false
		}
	}
	return true
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
