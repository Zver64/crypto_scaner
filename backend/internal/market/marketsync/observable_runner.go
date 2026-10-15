package marketsync

import (
	"context"
	"errors"
)

// ObservableRunner notifies consumers once a synchronization round ends, so
// they process its committed candle changes together. A failed round may have
// committed some candles, so it notifies as well.
type ObservableRunner struct {
	Runner
	Synced func()
}

func (runner ObservableRunner) Sync(ctx context.Context) error {
	err := runner.Runner.Sync(ctx)
	if !errors.Is(err, ErrSyncInProgress) && runner.Synced != nil {
		runner.Synced()
	}
	return err
}
