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

// replay calculates the values a strategy reads at past times, each over the
// window of stored candles the tracker would have loaded then.
type replay struct {
	registry *indicator.Registry
	// histories hold the subscribed pairs, and any other pair a caller
	// calculates over, and are only read; a history, once set, never
	// changes.
	histories map[market.CandleInterval]map[int64][]market.Candle
	// prepared are the histories the replay calculated over, prepared once
	// for the windows of all their candles.
	prepared map[market.CandleInterval]map[int64]*indicator.History
	// limits are the numbers of candles the histories were loaded with, by
	// interval; a history that fills its limit may lack older candles.
	limits  map[market.CandleInterval]int
	current snapshot
	depths  []int
	// opens are the open times the values were calculated at, so a coarser
	// interval is calculated once per its candle; cut marks values left
	// unknown because the stored history ends inside their window.
	opens []time.Time
	cut   []bool
}

// newReplay reads entry, its candles included, on instrument and on the
// active instruments it reads through of; histories, holding the subscribed pairs, is
// set before at is called.
func newReplay(registry *indicator.Registry, entry Entry, instrument Instrument, active []Instrument) (*replay, error) {
	reads := readsOf([]Entry{entry}, func(Entry, []Instrument) []Instrument { return []Instrument{instrument} }, active)
	depths := make([]int, len(reads.subscriptions))
	for index, subscription := range reads.subscriptions {
		depth, err := closedindicator.Depth(registry, subscription.Target, subscription.Points)
		if err != nil {
			return nil, err
		}
		depths[index] = depth
	}
	return &replay{
		registry: registry, depths: depths, prepared: map[market.CandleInterval]map[int64]*indicator.History{},
		current: snapshot{reads: reads, values: make([]closedindicator.Value, len(reads.subscriptions)), bySymbol: bySymbol(active)},
		opens:   make([]time.Time, len(reads.subscriptions)),
		cut:     make([]bool, len(reads.subscriptions)),
	}, nil
}

// loadHistories reads the latest limit(interval) stored candles of every
// pair replays subscribe, one query per interval, and gives the replays these
// histories.
func loadHistories(ctx context.Context, store CandleStore, limit func(market.CandleInterval) int, replays ...*replay) error {
	ids := map[market.CandleInterval][]int64{}
	for _, replay := range replays {
		for _, subscription := range replay.current.reads.subscriptions {
			if read := subscription.Target.Interval; !slices.Contains(ids[read], subscription.InstrumentID) {
				ids[read] = append(ids[read], subscription.InstrumentID)
			}
		}
	}
	histories := make(map[market.CandleInterval]map[int64][]market.Candle, len(ids))
	limits := make(map[market.CandleInterval]int, len(ids))
	for read, readIDs := range ids {
		limits[read] = limit(read)
		var err error
		if histories[read], err = store.ListLatestCandles(ctx, readIDs, read, limits[read]); err != nil {
			return fmt.Errorf("load %s history: %w", read, err)
		}
	}
	for _, replay := range replays {
		replay.histories, replay.limits = histories, limits
	}
	return nil
}

// cutOff reports whether the window of depth candles ending before the
// exclusive index end of a history of interval holding stored candles reaches
// before that history while older candles may have existed: when it holds at
// least market.SyncDepth candles, or as many as it was loaded with. A shorter
// history is the whole history of the instrument and needs only the window's
// minimum lookback.
func (replay *replay) cutOff(interval market.CandleInterval, stored, end, depth int) bool {
	limit, limited := replay.limits[interval]
	return end < depth && (stored >= market.SyncDepth || limited && stored >= limit)
}

// calculate applies the replay window and retained-history warm-up policy to
// subscription, which needs depth candles, at an exclusive end index.
// Snapshots use the same policy.
func (replay *replay) calculate(subscription closedindicator.Subscription, depth, end int) (closedindicator.Value, bool, error) {
	candles := replay.histories[subscription.Target.Interval][subscription.InstrumentID]
	if replay.cutOff(subscription.Target.Interval, len(candles), end, depth) {
		return closedindicator.Value{Target: subscription.Target}, false, nil
	}
	history := replay.prepared[subscription.Target.Interval][subscription.InstrumentID]
	if history == nil {
		history = indicator.NewHistory(subscription.Target.Interval, candles)
		if replay.prepared[subscription.Target.Interval] == nil {
			replay.prepared[subscription.Target.Interval] = map[int64]*indicator.History{}
		}
		replay.prepared[subscription.Target.Interval][subscription.InstrumentID] = history
	}
	value, err := closedindicator.CalculateRange(replay.registry, subscription.Target, history, max(0, end-depth), end, subscription.Points)
	return value, true, err
}

// at returns the values the monitor would have evaluated at now, valid until
// the next call. A window that reaches before the stored history leaves its
// value unknown and warm false when the history holds at least
// market.SyncDepth candles, or as many as it was loaded with: older candles
// existed then but may be missing, pruned, or not loaded now, whether the
// history was synchronized, loaded deeper on demand, or grew since. A shorter history is the whole history of the instrument,
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
		value, warm, err := replay.calculate(subscription, replay.depths[index], end)
		if err != nil {
			return snapshot{}, false, err
		}
		replay.cut[index] = !warm
		replay.current.values[index] = value
	}
	replay.current.now = now
	return replay.current, !slices.Contains(replay.cut, true), nil
}
