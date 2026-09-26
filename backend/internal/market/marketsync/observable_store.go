package marketsync

import (
	"context"

	"crypto-scanner/internal/market"
)

// ObservableStore notifies consumers only after a closed-candle batch commits.
// Notifications are bounded by the subscriber queues and never await analysis.
type ObservableStore struct {
	Store
	Changed func([]market.Candle)
}

func (store ObservableStore) UpsertCandlesWithChanges(ctx context.Context, candles []market.Candle) ([]market.Candle, error) {
	changed, err := store.Store.UpsertCandlesWithChanges(ctx, candles)
	if err != nil {
		return nil, err
	}
	if len(changed) > 0 && store.Changed != nil {
		store.Changed(changed)
	}
	return changed, nil
}
