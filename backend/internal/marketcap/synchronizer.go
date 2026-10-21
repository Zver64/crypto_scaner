package marketcap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"crypto-scanner/internal/market"
	"crypto-scanner/internal/platform/backoff"
	"crypto-scanner/internal/platform/logging"
)

// InstrumentSource supplies the active exchange universe to the background
// market-cap refresh and persists its outcomes. It is intentionally not used
// by HTTP analysis.
type InstrumentSource interface {
	ListActiveInstruments(context.Context) ([]market.Instrument, error)
	SaveSyncState(context.Context, market.SyncState) error
}

// CoinMetadataSynchronizer owns CoinGecko mapping bootstrap, market-cap and
// stablecoin-classification refreshes, retry timing, and observable outcomes.
const defaultStateSaveTimeout = 2 * time.Second

var (
	errInstrumentCatalogEmpty = errors.New("active instrument catalog is empty")
	errProviderUnavailable    = errors.New("market cap provider temporarily unavailable")
)

type CoinMetadataSynchronizer struct {
	resolver         *Resolver
	source           InstrumentSource
	logger           *slog.Logger
	interval         time.Duration
	retryDelay       time.Duration
	stateSaveTimeout time.Duration
}

func NewCoinMetadataSynchronizer(resolver *Resolver, source InstrumentSource, logger *slog.Logger, interval, retryDelay time.Duration) (*CoinMetadataSynchronizer, error) {
	if resolver == nil || source == nil || logger == nil || interval <= 0 || retryDelay <= 0 {
		return nil, fmt.Errorf("invalid coin metadata synchronizer")
	}
	return &CoinMetadataSynchronizer{resolver: resolver, source: source, logger: logger, interval: interval, retryDelay: retryDelay, stateSaveTimeout: defaultStateSaveTimeout}, nil
}

func (s *CoinMetadataSynchronizer) Run(ctx context.Context) error {
	// Each failure warns once while it keeps recurring, such as during a
	// provider outage.
	bootstrapFailure, refreshFailure := logging.NewFailure(s.logger), logging.NewFailure(s.logger)
	for {
		err := s.runOperation(ctx, "bootstrap", s.resolver.Bootstrap)
		if ctx.Err() != nil {
			return nil
		}
		if err == nil {
			break
		}
		bootstrapFailure.Warn(ctx, "coin metadata mapping bootstrap failed", "module", "coin_metadata", "error", err)
		if backoff.Sleep(ctx, s.retryDelay) != nil {
			return nil
		}
	}

	bootstrapFailure.Recover(ctx, "coin metadata mapping bootstrap recovered", "module", "coin_metadata")
	for {
		delay := s.interval
		err := s.runRefresh(ctx)
		if err != nil && ctx.Err() != nil {
			return nil
		}
		if err != nil {
			delay = s.retryDelay
		}
		switch {
		case err == nil:
			refreshFailure.Forget()
			s.logger.InfoContext(ctx, "coin metadata refresh completed", "module", "coin_metadata")
		case errors.Is(err, errInstrumentCatalogEmpty):
			s.logger.InfoContext(ctx, "coin metadata refresh not ready yet", "module", "coin_metadata", "error", err)
		default:
			refreshFailure.Warn(ctx, "coin metadata refresh failed", "module", "coin_metadata", "error", err)
		}
		if backoff.Sleep(ctx, delay) != nil {
			return nil
		}
	}
}

func (s *CoinMetadataSynchronizer) runRefresh(ctx context.Context) error {
	return s.runOperation(ctx, "refresh", s.Refresh)
}

func (s *CoinMetadataSynchronizer) runOperation(ctx context.Context, operation string, run func(context.Context) error) error {
	startedAt := time.Now().UTC()
	if err := s.source.SaveSyncState(ctx, market.SyncState{Profile: market.CoinMetadataSyncProfile(), LastStartedAt: &startedAt, Status: market.SyncStatusRunning}); err != nil {
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
	if saveErr := s.source.SaveSyncState(stateCtx, state); saveErr != nil {
		return fmt.Errorf("save coin metadata %s result: %w", operation, saveErr)
	}
	return err
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
		return errProviderUnavailable
	}
	return nil
}
