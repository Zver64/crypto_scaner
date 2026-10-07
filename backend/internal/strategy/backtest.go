package strategy

import (
	"context"
	"fmt"
	"slices"
	"time"

	"crypto-scanner/internal/closedindicator"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
)

// BacktestFee is the fee of each side of a simulated trade, as a fraction of
// the traded value.
const BacktestFee = 0.001

// MaxBacktestHold bounds the hold a backtest request may choose.
const MaxBacktestHold = 1000

// backtestHolds are the default holds in candles of the backtest interval: a
// day of hours, a week of days, about a month of weeks, and a quarter of
// months.
var backtestHolds = map[market.CandleInterval]int{
	market.IntervalHour:  24,
	market.IntervalDay:   7,
	market.IntervalWeek:  4,
	market.IntervalMonth: 3,
}

// Backtest is where a strategy would have alerted on one instrument over its
// stored closed history, and the trades of one position at a time that
// followed. Every trade enters at the open of the candle after its alert and
// exits at the close of its Hold-th consecutive candle, paying BacktestFee on
// both sides.
type Backtest struct {
	// Interval is the finest interval the expression reads, where alerts
	// fire and trades count candles.
	Interval market.CandleInterval
	Symbol   string
	Hold     int
	// From and To are the open times of the first and the last evaluated
	// candles; both are zero when none is evaluated.
	From, To time.Time
	// Alerts are the open times of the candles whose close would have
	// alerted, oldest first.
	Alerts []time.Time
	// Trades are the closed trades, oldest first. Unfinished counts trades
	// whose hold the stored consecutive candles do not reach, at the end of
	// the history or at a gap; Skipped counts alerts that fired while a
	// position was open.
	Trades              []Trade
	Unfinished, Skipped int
	// Stats, NetProfit, Equity, and MaxDrawdown describe the closed trades:
	// the equity compounds from 1 through them, and the drawdown is its
	// largest fall from an earlier peak, as a fraction of that peak, at trade
	// exits.
	Stats                  TradeStats
	NetProfit, MaxDrawdown float64
	Equity                 []EquityPoint
	// BuyAndHold is the net return from the open of the candle after the
	// first evaluated candle to the close of the last; nil without such
	// candles.
	BuyAndHold *float64
	// EveryCandle describes the trades of every evaluated candle taken as an
	// alert, overlapping, as a reference distribution.
	EveryCandle TradeStats
}

// Trade is a closed trade; the times are the open times of the entry and the
// exit candles, and Return is net of fees.
type Trade struct {
	EntryTime, ExitTime   time.Time
	EntryPrice, ExitPrice float64
	Return                float64
}

// EquityPoint is the equity after the trade whose exit candle opened at Time.
type EquityPoint struct {
	Time   time.Time
	Equity float64
}

// TradeStats describes the net returns of trades. WinRate and AverageTrade
// are nil without trades, AverageWin without wins, and AverageLoss and
// ProfitFactor, the sum of winning returns over the sum of losing ones,
// without losses.
type TradeStats struct {
	Count                                                        int
	WinRate, ProfitFactor, AverageTrade, AverageWin, AverageLoss *float64
}

// Backtest replays the alerts of a saved strategy, enabled or not, on the
// instrument symbol over its stored closed history, the way the monitor
// evaluates them: the same values at the close of every candle of the finest
// interval read, the same freshness, and the same transitions, starting from
// not matching. Candles are evaluated once every value is calculated over
// the whole window the tracker loads, so the replay never reads indicators
// the pruned history leaves unsettled; coins read through of are read only
// among the administrator's favorites, like the monitor does. Trades hold
// hold candles, or the default of the interval when hold is 0. It fails with
// ErrNotFound, ErrInvalidArgument for a strategy that no longer compiles, an
// empty symbol, or a hold out of range, and market.ErrInstrumentNotFound.
func (service *Service) Backtest(ctx context.Context, id int64, symbol string, hold int) (Backtest, error) {
	entries := service.List()
	index := slices.IndexFunc(entries, func(entry Entry) bool { return entry.ID == id })
	if index < 0 {
		return Backtest{}, ErrNotFound
	}
	entry := entries[index]
	if entry.Compiled == nil {
		return Backtest{}, fmt.Errorf("%w: %s", ErrInvalidArgument, entry.Problem)
	}
	// The finest interval read is replayed on the candles of the instrument,
	// even when only of reads it; compiled expressions read at least one.
	var interval market.CandleInterval
	for _, candidate := range market.CandleIntervals() {
		if slices.ContainsFunc(entry.Compiled.Reads(), func(read Read) bool { return read.Variable.Target.Interval == candidate }) {
			interval = candidate
			break
		}
	}
	if hold == 0 {
		hold = backtestHolds[interval]
	}
	if hold < 1 || hold > MaxBacktestHold {
		return Backtest{}, fmt.Errorf("%w: the hold must be from 1 to %d candles", ErrInvalidArgument, MaxBacktestHold)
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
	ids := map[market.CandleInterval][]int64{interval: {instrument.ID}}
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
	result := Backtest{Interval: interval, Symbol: instrument.Symbol, Hold: hold}
	if err := replay.run(ctx, entry, instrument, &result); err != nil {
		return Backtest{}, err
	}
	return result, nil
}

// run replays entry on instrument over the candles of the result's interval
// and simulates its trades.
func (replay *replay) run(ctx context.Context, entry Entry, instrument Instrument, result *Backtest) error {
	candles := replay.histories[result.Interval][instrument.ID]
	matching := false
	first := -1
	var alerts []int
	for index, candle := range candles {
		if err := ctx.Err(); err != nil {
			return err
		}
		current, warm, err := replay.at(result.Interval.NextOpenTime(candle.OpenTime))
		if err != nil {
			return err
		}
		if !warm {
			continue
		}
		matched, known := current.match(entry, instrument.ID)
		if known && matched && !matching {
			alerts = append(alerts, index)
			result.Alerts = append(result.Alerts, candle.OpenTime.UTC())
		}
		if known {
			matching = matched
		}
		if first < 0 {
			first = index
			result.From = candle.OpenTime.UTC()
		}
		result.To = candle.OpenTime.UTC()
	}
	result.simulate(candles, first, alerts)
	return nil
}

// simulate fills the trades, their statistics, and the baselines of result
// from the candles of its interval, of which every candle from first on is
// evaluated (none when first is negative), and the indexes of the alerting
// candles, ascending. One position is open at a time: it holds the candles
// up to the scheduled exit, even across a gap that leaves it unfinished, and
// alerts before its last candle are skipped.
func (result *Backtest) simulate(candles []market.Candle, first int, alerts []int) {
	if first < 0 {
		return
	}
	// runs[index] counts the consecutive candles from index on.
	runs := make([]int, len(candles))
	for index := len(candles) - 1; index >= 0; index-- {
		runs[index] = 1
		if index+1 < len(candles) && candles[index+1].OpenTime.Equal(result.Interval.NextOpenTime(candles[index].OpenTime)) {
			runs[index] += runs[index+1]
		}
	}
	// trade is the trade after the candle at index, if the history holds it.
	trade := func(index int) (Trade, bool) {
		if runs[index] <= result.Hold || !(candles[index+1].Open > 0) {
			return Trade{}, false
		}
		entry, exit := candles[index+1], candles[index+result.Hold]
		return Trade{
			EntryTime: entry.OpenTime.UTC(), EntryPrice: entry.Open,
			ExitTime: exit.OpenTime.UTC(), ExitPrice: exit.Close, Return: netReturn(entry.Open, exit.Close),
		}, true
	}

	var held time.Time // the open time of the last candle of the open position
	equity, peak := 1.0, 1.0
	returns := make([]float64, 0, len(alerts))
	for _, index := range alerts {
		if candles[index].OpenTime.Before(held) {
			result.Skipped++
			continue
		}
		held = candles[index].OpenTime
		for range result.Hold {
			held = result.Interval.NextOpenTime(held)
		}
		closed, ok := trade(index)
		if !ok {
			result.Unfinished++
			continue
		}
		result.Trades = append(result.Trades, closed)
		returns = append(returns, closed.Return)
		equity *= 1 + closed.Return
		peak = max(peak, equity)
		result.MaxDrawdown = max(result.MaxDrawdown, 1-equity/peak)
		result.Equity = append(result.Equity, EquityPoint{Time: closed.ExitTime, Equity: equity})
	}
	result.NetProfit = equity - 1
	result.Stats = tradeStats(returns)

	returns = returns[:0]
	for index := first; index < len(candles); index++ {
		if every, ok := trade(index); ok {
			returns = append(returns, every.Return)
		}
	}
	result.EveryCandle = tradeStats(returns)
	if first+1 < len(candles) && candles[first+1].Open > 0 {
		result.BuyAndHold = new(netReturn(candles[first+1].Open, candles[len(candles)-1].Close))
	}
}

// netReturn is the return of buying at entry and selling at exit, net of
// BacktestFee on both sides.
func netReturn(entry, exit float64) float64 {
	return exit/entry*(1-BacktestFee)*(1-BacktestFee) - 1
}

func tradeStats(returns []float64) TradeStats {
	stats := TradeStats{Count: len(returns)}
	if len(returns) == 0 {
		return stats
	}
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

// newReplay reads entry on instrument and on the favorites it reads through
// of; histories, holding the subscribed pairs, is set before at is called.
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
