package market

import "time"

const SevenDayPriceSlots = 169

// PriceHistoryWindow identifies hourly closes by their candle open times.
// Both bounds are inclusive; the currently open UTC hour is never included.
type PriceHistoryWindow struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

func SevenDayWindow(at time.Time) PriceHistoryWindow {
	end := at.UTC().Truncate(time.Hour).Add(-time.Hour)
	return PriceHistoryWindow{From: end.Add(-168 * time.Hour), To: end}
}

// HourlyPrice is presentation history, independent of criterion requirements.
type HourlyPrice struct {
	InstrumentID int64
	OpenTime     time.Time
	Close        float64
}
