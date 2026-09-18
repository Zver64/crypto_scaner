package marketcap

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"crypto-scanner/internal/market"
)

// InstrumentSource supplies the active exchange universe to the background
// market-cap refresh. It is intentionally not used by HTTP analysis.
type InstrumentSource interface {
	ListActiveInstruments(context.Context) ([]market.Instrument, error)
}

type syncStateStore interface {
	SaveSyncState(context.Context, market.SyncState) error
}

// CoinMetadataSynchronizer owns CoinGecko mapping bootstrap, market-cap and
// stablecoin-classification refreshes, retry timing, and observable outcomes.
const defaultStateSaveTimeout = 2 * time.Second

var errInstrumentCatalogEmpty = fmt.Errorf("active instrument catalog is empty")

type CoinMetadataSynchronizer struct {
	resolver         *Resolver
	source           InstrumentSource
	logger           *slog.Logger
	interval         time.Duration
	retryDelay       time.Duration
	stateSaveTimeout time.Duration
}

func NewCoinMetadataSynchronizer(resolver *Resolver, source InstrumentSource, logger *slog.Logger, interval, retryDelay time.Duration) (*CoinMetadataSynchronizer, error) {
	if resolver == nil || source == nil || interval <= 0 || retryDelay <= 0 {
		return nil, fmt.Errorf("invalid coin metadata synchronizer")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &CoinMetadataSynchronizer{resolver: resolver, source: source, logger: logger, interval: interval, retryDelay: retryDelay, stateSaveTimeout: defaultStateSaveTimeout}, nil
}

func (s *CoinMetadataSynchronizer) Run(ctx context.Context) error {
	for {
		err := s.runOperation(ctx, "bootstrap", s.resolver.Bootstrap)
		if ctx.Err() != nil {
			return nil
		}
		if err == nil {
			break
		}
		s.logger.WarnContext(ctx, "coin metadata mapping bootstrap failed", "module", "coin_metadata", "error", err.Error())
		if !wait(ctx, s.retryDelay) {
			return nil
		}
	}

	for {
		delay := s.interval
		if err := s.runRefresh(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			delay = s.retryDelay
			s.logger.WarnContext(ctx, "coin metadata refresh failed", "module", "coin_metadata", "error", err.Error())
		} else {
			s.logger.InfoContext(ctx, "coin metadata refresh completed", "module", "coin_metadata")
		}
		if !wait(ctx, delay) {
			return nil
		}
	}
}

func wait(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (s *CoinMetadataSynchronizer) runRefresh(ctx context.Context) error {
	return s.runOperation(ctx, "refresh", s.Refresh)
}

func (s *CoinMetadataSynchronizer) runOperation(ctx context.Context, operation string, run func(context.Context) error) error {
	startedAt := time.Now().UTC()
	if err := s.saveState(ctx, market.SyncState{Profile: market.CoinMetadataSyncProfile(), LastStartedAt: &startedAt, Status: market.SyncStatusRunning}); err != nil {
		return fmt.Errorf("save coin metadata %s start: %w", operation, err)
	}
	err := run(ctx)
	state := market.SyncState{Profile: market.CoinMetadataSyncProfile(), LastStartedAt: &startedAt}
	if err == nil {
		completedAt := time.Now().UTC()
		state.Status = market.SyncStatusSucceeded
		state.LastSucceededAt = &completedAt
	} else {
		state.Status = market.SyncStatusFailed
		state.ErrorMessage = err.Error()
	}
	// Result persistence is best-effort during shutdown, but it must never hold
	// the service lifecycle open indefinitely.
	stateCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.stateSaveTimeout)
	defer cancel()
	if saveErr := s.saveState(stateCtx, state); saveErr != nil {
		return fmt.Errorf("save coin metadata %s result: %w", operation, saveErr)
	}
	return err
}

func (s *CoinMetadataSynchronizer) saveState(ctx context.Context, state market.SyncState) error {
	store, ok := s.source.(syncStateStore)
	if !ok {
		return nil
	}
	return store.SaveSyncState(ctx, state)
}

// Refresh is never called from an HTTP request path.
func (s *CoinMetadataSynchronizer) Refresh(ctx context.Context) error {
	instruments, err := s.source.ListActiveInstruments(ctx)
	if err != nil {
		return fmt.Errorf("list active instruments: %w", err)
	}
	if len(instruments) == 0 {
		return errInstrumentCatalogEmpty
	}
	batch, err := s.resolver.ResolveBatch(ctx, instruments)
	if err != nil {
		return fmt.Errorf("refresh market caps: %w", err)
	}
	if err := s.resolver.RefreshStablecoinClassifications(ctx); err != nil {
		return fmt.Errorf("refresh stablecoin classifications: %w", err)
	}
	if batch.ProviderWarning {
		return fmt.Errorf("market cap provider temporarily unavailable")
	}
	return nil
}
