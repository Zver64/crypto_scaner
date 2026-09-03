package analysis

import (
	"context"
	"fmt"
	"time"

	"crypto-scanner/internal/market"
)

func (service *Service) priceHistories(ctx context.Context, instruments []market.Instrument, window market.PriceHistoryWindow) (map[int64][]*float64, error) {
	histories := make(map[int64][]*float64, len(instruments))
	if len(instruments) == 0 {
		return histories, nil
	}
	ids := make([]int64, len(instruments))
	for i, instrument := range instruments {
		ids[i] = instrument.ID
		histories[instrument.ID] = make([]*float64, market.SevenDayPriceSlots)
	}
	prices, err := service.store.ListHourlyPrices(ctx, ids, window.From, window.To)
	if err != nil {
		return nil, fmt.Errorf("load seven-day price history: %w", err)
	}
	for _, price := range prices {
		series, ok := histories[price.InstrumentID]
		offset := price.OpenTime.Sub(window.From)
		if !ok || offset < 0 || offset%time.Hour != 0 || price.OpenTime.After(window.To) {
			continue
		}
		series[int(offset/time.Hour)] = &price.Close
	}
	return histories, nil
}
