package marketsync

import (
	"context"
	"slices"
	"sync"

	"crypto-scanner/internal/market"
)

// ObservableStore notifies consumers only after a closed-candle batch
// commits, and after an instrument snapshot leaves other active instruments
// than it last reported, with all of them. Consumers must not block.
// Every write of the latest market.SyncDepth candles goes through Changed:
// the closed indicator windows rely on it and never reread the store.
type ObservableStore struct {
	Store
	Changed            func([]market.Candle)
	InstrumentsChanged func([]market.Instrument)

	// mu orders snapshots, so notifications follow their commits; last are
	// the active instruments last reported, nil before the first report.
	mu   sync.Mutex
	last []market.Instrument
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

// ApplyInstrumentSnapshot reports the active instruments the snapshot leaves
// when they differ from the last report; a snapshot whose result cannot be
// read is reported by the next one.
func (store *ObservableStore) ApplyInstrumentSnapshot(ctx context.Context, items []market.Instrument) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := store.Store.ApplyInstrumentSnapshot(ctx, items); err != nil {
		return err
	}
	active, err := store.Store.ListActiveInstruments(ctx)
	if err != nil {
		return err
	}
	same := store.last != nil && slices.EqualFunc(store.last, active, func(left, right market.Instrument) bool {
		return left.ID == right.ID && left.Symbol == right.Symbol
	})
	if !same && store.InstrumentsChanged != nil {
		store.InstrumentsChanged(active)
	}
	store.last = active
	return nil
}
