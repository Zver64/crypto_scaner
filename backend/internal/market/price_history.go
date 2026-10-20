package market

import "time"

const (
	SevenDayPriceSlots = 169
	// ThirtyDayPriceSlots are the closes of the last 31 closed UTC days
	// followed by the latest hourly close, which stands for the open day.
	ThirtyDayPriceSlots = 32
)

// PriceHistoryWindow spans a price series by the open times of the candles
// whose closes fill its first and last slot. Both bounds are inclusive; the
// currently open UTC hour is never included.
type PriceHistoryWindow struct {
	From time.Time
	To   time.Time
}

// PriceWindows are the windows of the presentation price series, fixed when
// an analysis starts.
type PriceWindows struct {
	SevenDay  PriceHistoryWindow
	ThirtyDay PriceHistoryWindow
}

func NewPriceWindows(at time.Time) PriceWindows {
	end := IntervalHour.LastClosedOpenTime(at)
	return PriceWindows{
		SevenDay:  PriceHistoryWindow{From: end.Add(-168 * time.Hour), To: end},
		ThirtyDay: PriceHistoryWindow{From: IntervalDay.LastClosedOpenTime(at).AddDate(0, 0, 2-ThirtyDayPriceSlots), To: end},
	}
}

// ClosePrice is presentation history, independent of criterion requirements.
type ClosePrice struct {
	InstrumentID int64
	OpenTime     time.Time
	Close        float64
}
