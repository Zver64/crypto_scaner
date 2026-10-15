package marketsync

import (
	"context"
	"maps"
	"sync"

	"crypto-scanner/internal/market"
)

// ObservableStore notifies consumers only after a closed-candle batch
// commits, and after a read of the active instruments finds another set of
// them (by ID and symbol, in any order) than it last reported, with all of
// them. Consumers must not block.
// Every write of the latest market.SyncDepth candles goes through Changed:
// the closed indicator windows rely on it and never reread the store.
type ObservableStore struct {
	Store
	Changed            func([]market.Candle)
	InstrumentsChanged func([]market.Instrument)

	// mu orders reads of the active instruments, so notifications follow
	// them; last is the set last reported, nil before the first report.
	mu   sync.Mutex
	last map[instrumentKey]struct{}
}

type instrumentKey struct {
	id     int64
	symbol string
}

func (store *ObservableStore) UpsertCandlesWithChanges(ctx context.Context, candles []market.Candle) ([]market.Candle, error) {
	changed, err := store.Store.UpsertCandlesWithChanges(ctx, candles)
	if err != nil {
		return nil, err
	}
	if len(changed) > 0 && store.Changed != nil {
		store.Changed(changed)
	}
	return changed, nil
}

// ListActiveInstrumentsByMarketCap reports the active instruments it reads
// when their set differs from the last report.
func (store *ObservableStore) ListActiveInstrumentsByMarketCap(ctx context.Context) ([]market.Instrument, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	active, err := store.Store.ListActiveInstrumentsByMarketCap(ctx)
	if err != nil {
		return nil, err
	}
	set := make(map[instrumentKey]struct{}, len(active))
	for _, instrument := range active {
		set[instrumentKey{instrument.ID, instrument.Symbol}] = struct{}{}
	}
	if (store.last == nil || !maps.Equal(store.last, set)) && store.InstrumentsChanged != nil {
		store.InstrumentsChanged(active)
	}
	store.last = set
	return active, nil
}
