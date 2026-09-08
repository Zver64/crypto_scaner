package market

import "time"

const (
	SevenDayPriceSlots  = 169
	ThirtyDayPriceSlots = 721
)

// PriceHistoryWindow identifies hourly closes by their candle open times.
// Both bounds are inclusive; the currently open UTC hour is never included.
type PriceHistoryWindow struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

func SevenDayWindow(at time.Time) PriceHistoryWindow {
	end := lastClosedHour(at)
	return PriceHistoryWindow{From: end.Add(-168 * time.Hour), To: end}
}

func ThirtyDayWindow(at time.Time) PriceHistoryWindow {
	end := lastClosedHour(at)
	return PriceHistoryWindow{From: end.Add(-720 * time.Hour), To: end}
}

func lastClosedHour(at time.Time) time.Time {
	return at.UTC().Truncate(time.Hour).Add(-time.Hour)
}

// HourlyPrice is presentation history, independent of criterion requirements.
type HourlyPrice struct {
	InstrumentID int64
	OpenTime     time.Time
	Close        float64
}

// HourlyCandle is presentation OHLC history for an instrument chart.
type HourlyCandle struct {
	InstrumentID int64     `json:"-"`
	OpenTime     time.Time `json:"open_time"`
	Open         float64   `json:"open"`
	High         float64   `json:"high"`
	Low          float64   `json:"low"`
	Close        float64   `json:"close"`
}
