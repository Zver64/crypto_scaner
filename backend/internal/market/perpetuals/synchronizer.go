// Package perpetuals keeps the stored list of Binance COIN-M perpetual
// contracts current. Requests only read the stored list; the exchange is asked
// once an interval, in the background.
package perpetuals

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"crypto-scanner/internal/market"
	"crypto-scanner/internal/platform/backoff"
)

// Exchange lists the base assets of trading COIN-M perpetual contracts.
type Exchange interface {
	BaseAssets(context.Context) ([]string, error)
}

// Store keeps the contract list and the time of its last refresh.
type Store interface {
	GetSyncState(context.Context, market.SyncProfile) (market.SyncState, error)
	ReplaceCoinMPerpetuals(context.Context, []string, market.SyncState) error
}

type Synchronizer struct {
	exchange   Exchange
	store      Store
	logger     *slog.Logger
	interval   time.Duration
	retryDelay time.Duration
}

// NewSynchronizer refreshes the list every interval; a failed refresh is
// retried after retryDelay, doubling up to interval.
func NewSynchronizer(exchange Exchange, store Store, logger *slog.Logger, interval, retryDelay time.Duration) (*Synchronizer, error) {
	if exchange == nil || store == nil || logger == nil || interval <= 0 || retryDelay <= 0 {
		return nil, errors.New("invalid COIN-M perpetuals synchronizer")
	}
	return &Synchronizer{exchange: exchange, store: store, logger: logger.With("module", "coinm_perpetuals"), interval: interval, retryDelay: retryDelay}, nil
}

// Run refreshes the list when the stored one is an interval old, so restarts
// do not ask the exchange again.
func (s *Synchronizer) Run(ctx context.Context) error {
	var delay time.Duration
	state, err := s.store.GetSyncState(ctx, market.CoinMPerpetualsSyncProfile())
	switch {
	case ctx.Err() != nil:
		return nil
	case err != nil:
		s.logger.WarnContext(ctx, "COIN-M perpetuals refresh state unavailable", "error", err)
	case state.LastSucceededAt != nil:
		delay = s.interval - time.Since(*state.LastSucceededAt)
	}
	for failures := 0; ; {
		if backoff.Sleep(ctx, max(delay, 0)) != nil {
			return nil
		}
		if err := s.refresh(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			s.logger.WarnContext(ctx, "COIN-M perpetuals refresh failed", "error", err)
			delay = backoff.Exponential(s.retryDelay, s.interval, failures)
			failures++
			continue
		}
		failures = 0
		delay = s.interval
	}
}

func (s *Synchronizer) refresh(ctx context.Context) error {
	startedAt := time.Now().UTC()
	assets, err := s.exchange.BaseAssets(ctx)
	if err != nil {
		return err
	}
	if len(assets) == 0 {
		return errors.New("Binance lists no COIN-M perpetuals")
	}
	succeededAt := time.Now().UTC()
	state := market.SyncState{Profile: market.CoinMPerpetualsSyncProfile(), LastStartedAt: &startedAt, LastSucceededAt: &succeededAt, Status: market.SyncStatusSucceeded}
	if err := s.store.ReplaceCoinMPerpetuals(ctx, assets, state); err != nil {
		return fmt.Errorf("store COIN-M perpetuals: %w", err)
	}
	return nil
}
