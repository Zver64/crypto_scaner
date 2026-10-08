package market

import (
	"slices"
	"time"
)

// CandleInterval identifies a Binance Spot candlestick granularity supported by
// synchronization and chart history.
type CandleInterval string

const (
	IntervalHour  CandleInterval = "1h"
	IntervalDay   CandleInterval = "1d"
	IntervalWeek  CandleInterval = "1w"
	IntervalMonth CandleInterval = "1M"
)

// SyncDepth is the number of closed candles synchronization backfills and
// keeps complete per instrument and interval. It bounds chart ranges,
// indicator warm-ups and parameters, analysis periods, and what strategies
// read, so everything live works on the synchronized history alone.
const SyncDepth = 2000

// RetentionDepth is the number of closed candles kept per instrument and
// interval; the retention pruner deletes older ones. Candles beyond SyncDepth
// come from on-demand history loads and from synchronized candles
// accumulating, and serve backtests.
const RetentionDepth = 20000

// CandleIntervals returns the supported intervals in increasing granularity.
func CandleIntervals() []CandleInterval {
	return []CandleInterval{IntervalHour, IntervalDay, IntervalWeek, IntervalMonth}
}

func (interval CandleInterval) Valid() bool {
	return slices.Contains(CandleIntervals(), interval)
}

// OpenTime returns the UTC open boundary of the interval containing at.
func (interval CandleInterval) OpenTime(at time.Time) time.Time {
	utc := at.UTC()
	switch interval {
	case IntervalHour:
		return utc.Truncate(time.Hour)
	case IntervalDay:
		return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
	case IntervalWeek:
		day := time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
		daysSinceMonday := (int(day.Weekday()) + 6) % 7
		return day.AddDate(0, 0, -daysSinceMonday)
	case IntervalMonth:
		return time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
	default:
		return time.Time{}
	}
}

// NextOpenTime advances one calendar interval from an aligned open time.
func (interval CandleInterval) NextOpenTime(open time.Time) time.Time {
	open = interval.OpenTime(open)
	switch interval {
	case IntervalHour:
		return open.Add(time.Hour)
	case IntervalDay:
		return open.AddDate(0, 0, 1)
	case IntervalWeek:
		return open.AddDate(0, 0, 7)
	case IntervalMonth:
		return open.AddDate(0, 1, 0)
	default:
		return time.Time{}
	}
}

// PreviousOpenTime moves one calendar interval before an aligned open time.
func (interval CandleInterval) PreviousOpenTime(open time.Time) time.Time {
	open = interval.OpenTime(open)
	switch interval {
	case IntervalHour:
		return open.Add(-time.Hour)
	case IntervalDay:
		return open.AddDate(0, 0, -1)
	case IntervalWeek:
		return open.AddDate(0, 0, -7)
	case IntervalMonth:
		return open.AddDate(0, -1, 0)
	default:
		return time.Time{}
	}
}

// LastClosedOpenTime returns the latest interval open whose close boundary is
// not after at.
func (interval CandleInterval) LastClosedOpenTime(at time.Time) time.Time {
	return interval.PreviousOpenTime(interval.OpenTime(at))
}

// CandlesBetween counts the candles from the one opening at from through the
// one opening at to, both aligned; 0 when to is before from.
func (interval CandleInterval) CandlesBetween(from, to time.Time) int {
	if to.Before(from) {
		return 0
	}
	switch interval {
	case IntervalHour, IntervalDay, IntervalWeek:
		return int(to.Sub(from)/interval.NextOpenTime(from).Sub(from)) + 1
	case IntervalMonth:
		from, to = from.UTC(), to.UTC()
		return (to.Year()-from.Year())*12 + int(to.Month()-from.Month()) + 1
	default:
		return 0
	}
}
