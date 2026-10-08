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

// Backtest is how a strategy would have traded one instrument over its
// stored closed history: the monitor's trades replayed candle by candle.
// Every buy spends one quote unit at the open of the candle after its
// signal, and a sell sells every buy of the trade at the open of the candle
// after its signal, paying BacktestFee on both sides.
type Backtest struct {
	// Interval is the finest interval the rules read, where signals fire
	// and trades count candles.
	Interval market.CandleInterval
	Symbol   string
	// From and To are the open times of the first and the last evaluated
	// candles; both are zero when none is evaluated.
	From, To time.Time
	// Alerts are the open times of the candles whose close signaled a buy,
	// oldest first.
	Alerts []time.Time
	// Trades are the trades, oldest first; the last one is open when the
	// history ends before its sell. Skipped counts entry signals that bought
	// nothing, since the trade does not accumulate or holds max buys.
	Trades  []Trade
	Skipped int
	// Stats describes the closed trades. NetProfit, Equity, and MaxDrawdown
	// include the open trade, valued at the last close: the equity compounds
	// from 1 through the trades, and the drawdown is its largest fall from an
	// earlier peak, as a fraction of that peak, at trade exits.
	Stats                  TradeStats
	NetProfit, MaxDrawdown float64
	Equity                 []EquityPoint
	// BuyAndHold is the net return from the open of the candle after the
	// first evaluated candle to the close of the last; DCA the net return of
	// buying one quote unit at the open after every evaluated candle but the
	// last, valued at the close of the last. Both are nil without such
	// candles.
	BuyAndHold, DCA *float64
}

// Trade is a trade of one or more buys. EntryTime is the open time of the
// candle its first buy filled at and EntryPrice the average price of its
// buys; ExitTime is the open time of the candle it sold at, or, for an open
// trade, of the last candle, whose close values it. Return is net of fees.
type Trade struct {
	EntryTime, ExitTime   time.Time
	EntryPrice, ExitPrice float64
	Buys                  int
	Open                  bool
	Return                float64
}

// EquityPoint is the equity after the trade that exited at Time.
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

// Backtest replays the trades of a saved strategy, enabled or not, on the
// instrument symbol over its stored closed history, the way the monitor
// trades: the same values at the close of every candle of its interval, the
// same freshness, and the same signals, starting without a trade and with a
// false entry. Candles are evaluated once every value is calculated over the
// whole window the tracker loads, so the replay never reads indicators the
// pruned history leaves unsettled; coins read through of are read only among
// the administrator's favorites, like the monitor does. It fails with
// ErrNotFound, ErrInvalidArgument for a strategy that no longer compiles or
// an empty symbol, and market.ErrInstrumentNotFound.
func (service *Service) Backtest(ctx context.Context, id int64, symbol string) (Backtest, error) {
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
	result := Backtest{Interval: entry.Interval, Symbol: instrument.Symbol}
	if err := replay.run(ctx, entry, instrument, &result); err != nil {
		return Backtest{}, err
	}
	return result, nil
}

// run replays the trades of entry on instrument over the candles of the
// result's interval.
func (replay *replay) run(ctx context.Context, entry Entry, instrument Instrument, result *Backtest) error {
	candles := replay.histories[result.Interval][instrument.ID]
	var state TradeState
	first := -1
	equity, peak := 1.0, 1.0
	var returns []float64
	record := func(trade Trade) {
		trade.Return = netReturn(trade.EntryPrice, trade.ExitPrice)
		result.Trades = append(result.Trades, trade)
		if !trade.Open {
			returns = append(returns, trade.Return)
		}
		equity *= 1 + trade.Return
		peak = max(peak, equity)
		result.MaxDrawdown = max(result.MaxDrawdown, 1-equity/peak)
		result.Equity = append(result.Equity, EquityPoint{Time: trade.ExitTime, Equity: equity})
	}
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
		if first < 0 {
			first = index
			result.From = candle.OpenTime.UTC()
		}
		result.To = candle.OpenTime.UTC()
		var events []TradeEvent
		state, events, _ = current.advance(entry, instrument.ID, state)
		for _, event := range events {
			switch event.Kind {
			case TradeBuy:
				result.Alerts = append(result.Alerts, candle.OpenTime.UTC())
			case TradeSkip:
				result.Skipped++
			case TradeSell:
				// The sell fills at the next open, or at this close when the
				// history ends here.
				exit := candle
				price := candle.Close
				if index+1 < len(candles) && candles[index+1].Open > 0 {
					exit, price = candles[index+1], candles[index+1].Open
				}
				record(Trade{
					EntryTime: event.Trade.OpenedAt, EntryPrice: event.Trade.EntryPrice(), Buys: event.Trade.Buys,
					ExitTime: exit.OpenTime.UTC(), ExitPrice: price,
				})
			}
		}
	}
	if state.Filled > 0 {
		last := candles[len(candles)-1]
		record(Trade{
			EntryTime: state.OpenedAt, EntryPrice: state.EntryPrice(), Buys: state.Filled,
			ExitTime: last.OpenTime.UTC(), ExitPrice: last.Close, Open: true,
		})
	}
	result.NetProfit = equity - 1
	result.Stats = tradeStats(returns)
	result.baselines(candles, first)
	return nil
}

// baselines fills the baselines from the candles of the result's interval,
// of which every candle from first on is evaluated (none when first is
// negative).
func (result *Backtest) baselines(candles []market.Candle, first int) {
	if first < 0 || first+1 >= len(candles) || !(candles[first+1].Open > 0) {
		return
	}
	last := candles[len(candles)-1].Close
	result.BuyAndHold = new(netReturn(candles[first+1].Open, last))
	var quantity float64
	buys := 0
	for _, candle := range candles[first+1:] {
		if candle.Open > 0 {
			quantity += 1 / candle.Open
			buys++
		}
	}
	result.DCA = new(netReturn(float64(buys)/quantity, last))
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
