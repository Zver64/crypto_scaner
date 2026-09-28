// Package retention bounds stored market history. It keeps a fixed number of
// the newest candles per instrument and interval, and removes instruments the
// exchange no longer trades once nobody has them as a favorite.
package retention

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"crypto-scanner/internal/market"
	"crypto-scanner/internal/platform/backoff"
)

const (
	period = 7 * 24 * time.Hour
	// startupDelay lets startup synchronization finish before pruning.
	startupDelay = 10 * time.Minute
	retryDelay   = time.Hour
	// delistingGrace keeps the history of an instrument whose trading is only
	// briefly halted.
	delistingGrace = 7 * 24 * time.Hour
)

// Store is the persistence boundary required by the pruner.
type Store interface {
	// PruneCandles keeps the newest keep candles of an interval per instrument.
	PruneCandles(context.Context, market.CandleInterval, int) (int64, error)
	// DeleteDelistedInstruments removes, with their candles, instruments
	// inactive since before the given time that nobody has favorited.
	DeleteDelistedInstruments(context.Context, time.Time) ([]string, error)
}

// Pruner deletes history beyond the kept depth once a week.
type Pruner struct {
	store  Store
	logger *slog.Logger
	keep   int
}

// New creates a pruner that keeps keep candles per instrument and interval.
func New(store Store, logger *slog.Logger, keep int) *Pruner {
	return &Pruner{store: store, logger: logger.With("module", "market_retention"), keep: keep}
}

// Run prunes shortly after startup and then weekly until ctx is cancelled.
func (pruner *Pruner) Run(ctx context.Context) error {
	delay := startupDelay
	for {
		if backoff.Sleep(ctx, delay) != nil {
			return nil
		}
		delay = period
		if err := pruner.Prune(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			pruner.logger.WarnContext(ctx, "market history pruning failed", "operation", "prune", "outcome", "failure", "retry_after", retryDelay, "error", err)
			delay = retryDelay
		}
	}
}

// Prune removes delisted instruments past their grace period, then candles
// beyond the kept depth.
func (pruner *Pruner) Prune(ctx context.Context) error {
	symbols, err := pruner.store.DeleteDelistedInstruments(ctx, time.Now().Add(-delistingGrace))
	if err != nil {
		return fmt.Errorf("delete delisted instruments: %w", err)
	}
	var candles int64
	for _, interval := range market.CandleIntervals() {
		deleted, err := pruner.store.PruneCandles(ctx, interval, pruner.keep)
		if err != nil {
			return err
		}
		candles += deleted
	}
	pruner.logger.InfoContext(ctx, "market history pruned", "operation", "prune", "outcome", "success",
		"candles_deleted", candles, "instruments_deleted", len(symbols), "symbols", symbols)
	return nil
}
