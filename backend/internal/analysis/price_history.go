package analysis

import (
	"context"
	"fmt"
	"time"

	"crypto-scanner/internal/market"
)

// PriceHistories are the presentation close series of an instrument over the
// market.PriceWindows; missing closes are nil.
type PriceHistories struct {
	SevenDay  []*float64
	ThirtyDay []*float64
}

// PriceHistories loads the price histories of the instruments over windows.
func (service *Service) PriceHistories(ctx context.Context, ids []int64, windows market.PriceWindows) (map[int64]PriceHistories, error) {
	histories := make(map[int64]PriceHistories, len(ids))
	if len(ids) == 0 {
		return histories, nil
	}
	hourly, err := service.closes(ctx, ids, market.IntervalHour, windows.SevenDay.From, windows.SevenDay.To, market.SevenDayPriceSlots)
	if err != nil {
		return nil, fmt.Errorf("load seven-day price history: %w", err)
	}
	lastDay := windows.ThirtyDay.From.AddDate(0, 0, market.ThirtyDayPriceSlots-2)
	daily, err := service.closes(ctx, ids, market.IntervalDay, windows.ThirtyDay.From, lastDay, market.ThirtyDayPriceSlots)
	if err != nil {
		return nil, fmt.Errorf("load thirty-day price history: %w", err)
	}
	for _, id := range ids {
		// The latest hourly close stands for the open day.
		daily[id][market.ThirtyDayPriceSlots-1] = hourly[id][market.SevenDayPriceSlots-1]
		histories[id] = PriceHistories{SevenDay: hourly[id], ThirtyDay: daily[id]}
	}
	return histories, nil
}

// closes places the closes of the interval candles opening from through to
// into series of the given length, one slot per candle from the first.
func (service *Service) closes(ctx context.Context, ids []int64, interval market.CandleInterval, from, to time.Time, slots int) (map[int64][]*float64, error) {
	series := make(map[int64][]*float64, len(ids))
	for _, id := range ids {
		series[id] = make([]*float64, slots)
	}
	prices, err := service.store.ListClosePrices(ctx, ids, interval, from, to)
	if err != nil {
		return nil, err
	}
	for _, price := range prices {
		slot := interval.CandlesBetween(from, price.OpenTime) - 1
		values, ok := series[price.InstrumentID]
		if !ok || slot < 0 || slot >= slots || !interval.OpenTime(price.OpenTime).Equal(price.OpenTime) {
			continue
		}
		values[slot] = &price.Close
	}
	return series, nil
}
