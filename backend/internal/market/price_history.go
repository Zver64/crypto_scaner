package market

import "time"

const SevenDayPriceSlots = 169

// PriceHistoryWindow identifies hourly closes by their candle open times.
// Both bounds are inclusive; the currently open UTC hour is never included.
type PriceHistoryWindow struct {
	From time.Time
	To   time.Time
}

func SevenDayWindow(at time.Time) PriceHistoryWindow {
	end := IntervalHour.LastClosedOpenTime(at)
	return PriceHistoryWindow{From: end.Add(-168 * time.Hour), To: end}
}

// HourlyPrice is presentation history, independent of criterion requirements.
type HourlyPrice struct {
	InstrumentID int64
	OpenTime     time.Time
	Close        float64
}
