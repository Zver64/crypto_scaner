package sync

import (
	"context"
	"errors"
	"fmt"
	"time"

	"crypto-scanner/internal/market"
)

// Exchange is the discovery boundary required by instrument synchronization.
type Exchange interface {
	ListInstruments(context.Context) ([]market.Instrument, error)
}

// Store is the persistence boundary required by instrument synchronization.
type Store interface {
	GetSyncState(context.Context, market.SyncProfile) (market.SyncState, error)
	SaveSyncState(context.Context, market.SyncState) error
	ApplyInstrumentSnapshot(context.Context, []market.Instrument) error
}

// MVPProfile returns the code-owned Binance Spot synchronization profile.
func MVPProfile() market.SyncProfile {
	return market.SyncProfile{
		Exchange: "binance", Market: "spot", QuoteAsset: "USDT", Interval: "1d", TimeZone: "UTC",
	}
}

// Synchronizer coordinates one complete instrument discovery operation.
type Synchronizer struct {
	exchange Exchange
	store    Store
}

// New creates an instrument catalog synchronizer.
func New(exchange Exchange, store Store) *Synchronizer {
	return &Synchronizer{exchange: exchange, store: store}
}

// Sync discovers and transactionally applies the complete instrument catalog.
func (synchronizer *Synchronizer) Sync(ctx context.Context) error {
	profile := MVPProfile()
	state, err := synchronizer.store.GetSyncState(ctx, profile)
	if err != nil {
		return fmt.Errorf("load synchronization state: %w", err)
	}
	startedAt := time.Now().UTC()
	state.Profile = profile
	state.LastStartedAt = &startedAt
	state.Status = market.SyncStatusRunning
	state.ErrorMessage = ""
	if err := synchronizer.store.SaveSyncState(ctx, state); err != nil {
		return fmt.Errorf("record running synchronization: %w", err)
	}

	items, err := synchronizer.exchange.ListInstruments(ctx)
	if err != nil {
		return synchronizer.recordFailure(ctx, &state, fmt.Errorf("discover instruments: %w", err))
	}
	if len(items) == 0 {
		return synchronizer.recordFailure(ctx, &state, fmt.Errorf("discover instruments: empty snapshot"))
	}
	if err := synchronizer.store.ApplyInstrumentSnapshot(ctx, items); err != nil {
		return synchronizer.recordFailure(ctx, &state, fmt.Errorf("apply instrument snapshot: %w", err))
	}

	succeededAt := time.Now().UTC()
	previousSucceededAt := state.LastSucceededAt
	state.LastSucceededAt = &succeededAt
	state.Status = market.SyncStatusSucceeded
	if err := synchronizer.store.SaveSyncState(ctx, state); err != nil {
		state.LastSucceededAt = previousSucceededAt
		return synchronizer.recordFailure(ctx, &state, fmt.Errorf("record successful synchronization: %w", err))
	}
	return nil
}

func (synchronizer *Synchronizer) recordFailure(ctx context.Context, state *market.SyncState, failure error) error {
	state.Status = market.SyncStatusFailed
	state.ErrorMessage = failure.Error()
	if err := synchronizer.store.SaveSyncState(ctx, *state); err != nil {
		return errors.Join(failure, fmt.Errorf("record failed synchronization: %w", err))
	}
	return failure
}
