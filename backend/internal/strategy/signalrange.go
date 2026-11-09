package strategy

import (
	"context"
	"fmt"
	"iter"
	"slices"
	"time"

	"crypto-scanner/internal/market"
	"crypto-scanner/internal/platform/numeric"
)

// The range of a coin in a signal message is the exceedance percentile
// signalRangePercentile of the ranges of exactly the latest
// signalRangeHours closed hourly candles.
const (
	signalRangeHours      = 24
	signalRangePercentile = 80
)

// signalRangesFailure is the failure of loading the candles of signal ranges.
var signalRangesFailure = failureKey{operation: "load signal ranges"}

// coinRanges are the ranges of the coins of one signal message. Unloaded
// ranges are left out of the message; a coin without a range, its history
// too short, has n/a.
type coinRanges struct {
	loaded bool
	values map[int64]float64
}

// text is the range part of the line of the coin of instrumentID, empty when
// the ranges are unloaded.
func (ranges coinRanges) text(instrumentID int64) string {
	if !ranges.loaded {
		return ""
	}
	value, ok := ranges.values[instrumentID]
	if !ok {
		return "range n/a"
	}
	return fmt.Sprintf("range %.2f%%", value)
}

// signalRanges loads, only for a signal, the ranges of the instruments a
// message of entry lists, at now, the time of the round.
func (monitor *Monitor) signalRanges(ctx context.Context, entry Entry, now time.Time, instruments iter.Seq[Instrument]) coinRanges {
	if !entry.Signal {
		return coinRanges{}
	}
	var ids []int64
	for instrument := range instruments {
		ids = append(ids, instrument.ID)
	}
	if len(ids) == 0 {
		return coinRanges{}
	}
	slices.Sort(ids)
	histories, err := monitor.candles.ListLatestCandles(ctx, slices.Compact(ids), market.IntervalHour, signalRangeHours)
	if err != nil {
		// A stopping monitor sends without ranges and warns about nothing.
		if ctx.Err() == nil {
			monitor.failures.Warn(ctx, signalRangesFailure, "load signal ranges failed", "error", err)
		}
		return coinRanges{}
	}
	monitor.failures.Recover(ctx, signalRangesFailure, "load signal ranges recovered")
	latest := market.IntervalHour.LastClosedOpenTime(now)
	ranges := coinRanges{loaded: true, values: map[int64]float64{}}
	for id, candles := range histories {
		if value, ok := signalRange(candles, latest); ok {
			ranges.values[id] = value
		}
	}
	return ranges
}

// signalRange is the range of the chronological hourly candles, which must
// be exactly the signalRangeHours ending at the candle opening at latest,
// each with a positive open.
func signalRange(candles []market.Candle, latest time.Time) (float64, bool) {
	if len(candles) != signalRangeHours || !candles[len(candles)-1].OpenTime.Equal(latest) ||
		candles[len(candles)-1].OpenTime.Sub(candles[0].OpenTime) != (signalRangeHours-1)*time.Hour {
		return 0, false
	}
	ranges := make([]float64, len(candles))
	for i, candle := range candles {
		if !(candle.Open > 0) {
			return 0, false
		}
		ranges[i] = candle.RangePercent()
	}
	return numeric.ExceedancePercentile(ranges, signalRangePercentile), true
}
