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

// Backtest is where a strategy would have alerted on one instrument over its
// kept closed history.
type Backtest struct {
	// Interval is the finest interval the expression reads, where alerts
	// fire.
	Interval market.CandleInterval
	// From and To are the open times of the first and the last evaluated
	// candles of Interval; both are zero when none is evaluated.
	From, To time.Time
	// Alerts are the open times of the candles whose close would have
	// alerted, oldest first.
	Alerts []time.Time
}

// Backtest replays the alerts of any strategy, enabled or not, on the
// instrument symbol over its kept closed history, the way the monitor
// evaluates them: the same values at the close of every candle of the finest
// interval read, the same freshness, and the same transitions, starting from
// not matching. Candles are evaluated once every value is calculated over the
// whole window the tracker loads, so the replay never reads indicators the
// pruned history leaves unsettled. It fails with ErrNotFound,
// market.ErrInstrumentNotFound, or ErrInvalidArgument for a strategy that no
// longer compiles.
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
	found, err := service.store.GetActiveInstrumentBySymbol(ctx, symbol)
	if err != nil {
		return Backtest{}, fmt.Errorf("resolve backtest instrument: %w", err)
	}
	favorites, err := service.store.ListStrategyInstruments(ctx, service.administratorID)
	if err != nil {
		return Backtest{}, fmt.Errorf("load backtest favorites: %w", err)
	}
	instrument := Instrument{ID: found.ID, Symbol: found.Symbol}
	replay, err := newReplay(service.registry, entry, instrument, favorites)
	if err != nil {
		return Backtest{}, err
	}
	// The finest interval read is replayed on the candles of the instrument,
	// even when only of reads it; compiled expressions read at least one.
	var result Backtest
	for _, interval := range market.CandleIntervals() {
		if slices.ContainsFunc(entry.Compiled.Reads(), func(read Read) bool { return read.Variable.Target.Interval == interval }) {
			result.Interval = interval
			break
		}
	}
	ids := map[market.CandleInterval][]int64{result.Interval: {instrument.ID}}
	for _, subscription := range replay.current.reads.subscriptions {
		if interval := subscription.Target.Interval; !slices.Contains(ids[interval], subscription.InstrumentID) {
			ids[interval] = append(ids[interval], subscription.InstrumentID)
		}
	}
	for interval, intervalIDs := range ids {
		if replay.histories[interval], err = service.store.ListLatestCandles(ctx, intervalIDs, interval, market.HistoryDepth); err != nil {
			return Backtest{}, fmt.Errorf("load backtest %s history: %w", interval, err)
		}
	}
	matching := false
	for _, candle := range replay.histories[result.Interval][instrument.ID] {
		if err := ctx.Err(); err != nil {
			return Backtest{}, err
		}
		current, warm, err := replay.at(result.Interval.NextOpenTime(candle.OpenTime))
		if err != nil {
			return Backtest{}, err
		}
		if !warm {
			continue
		}
		if result.From.IsZero() {
			result.From = candle.OpenTime.UTC()
		}
		result.To = candle.OpenTime.UTC()
		matched, known := current.match(entry, instrument.ID)
		if !known {
			continue
		}
		if matched && !matching {
			result.Alerts = append(result.Alerts, candle.OpenTime.UTC())
		}
		matching = matched
	}
	return result, nil
}

// replay calculates the values a strategy reads at past times, each over the
// window of kept candles the tracker would have loaded then.
type replay struct {
	registry  *indicator.Registry
	histories map[market.CandleInterval]map[int64][]market.Candle
	current   snapshot
	depths    []int
	// opens are the open times the values were calculated at, so a coarser
	// interval is calculated once per its candle; cut marks values left
	// unknown because the kept history ends inside their window.
	opens []time.Time
	cut   []bool
}

// newReplay reads entry on instrument and on the favorites it reads through
// of; the histories of the subscribed pairs are added before at is called.
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
		registry: registry, histories: map[market.CandleInterval]map[int64][]market.Candle{}, depths: depths,
		current: snapshot{reads: reads, values: make([]closedindicator.Value, len(reads.subscriptions)), bySymbol: bySymbol(favorites)},
		opens:   make([]time.Time, len(reads.subscriptions)),
		cut:     make([]bool, len(reads.subscriptions)),
	}, nil
}

// at returns the values the monitor would have evaluated at now, valid until
// the next call. A window that reaches before a pruned history, one holding
// market.HistoryDepth candles, leaves its value unknown and warm false; a
// shorter history is the whole history of the instrument, which the tracker
// read the same way.
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
		if replay.cut[index] = end < replay.depths[index] && len(candles) >= market.HistoryDepth; replay.cut[index] {
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
