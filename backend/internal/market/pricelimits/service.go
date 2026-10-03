// Package pricelimits serves the exchange price limits of a symbol: the
// current reference price and the price range rule, which changes rarely and
// is cached per symbol for a day after a client asks for it.
package pricelimits

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"crypto-scanner/internal/market"

	"golang.org/x/sync/singleflight"
)

const (
	// rangeTTL is how long a fetched price range rule is reused.
	rangeTTL = 24 * time.Hour
	// fetchTimeout bounds a shared price range request.
	fetchTimeout = 10 * time.Second
)

// Exchange reads the price limits of a symbol from the exchange.
type Exchange interface {
	ReferencePrice(context.Context, string) (float64, error)
	PriceRange(context.Context, string) (*market.PriceRange, error)
}

type Options struct {
	Now func() time.Time
}

type cachedRange struct {
	value     *market.PriceRange
	fetchedAt time.Time
}

// Service requests the reference price on every call and refreshes a symbol's
// price range rule only when a call finds it missing or older than a day.
type Service struct {
	exchange Exchange
	logger   *slog.Logger
	now      func() time.Time
	group    singleflight.Group

	mu     sync.Mutex
	ranges map[string]cachedRange
}

func New(exchange Exchange, logger *slog.Logger, options Options) *Service {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Service{exchange: exchange, logger: logger.With("module", "pricelimits"), now: now, ranges: make(map[string]cachedRange)}
}

// Limits returns the current reference price and price range rule of symbol.
// An unknown symbol is market.ErrInstrumentNotFound.
func (service *Service) Limits(ctx context.Context, symbol string) (market.PriceLimits, error) {
	symbol = market.NormalizeSymbol(symbol)
	price, err := service.exchange.ReferencePrice(ctx, symbol)
	if err != nil {
		return market.PriceLimits{}, err
	}
	priceRange, err := service.priceRange(ctx, symbol)
	if err != nil {
		return market.PriceLimits{}, err
	}
	return market.PriceLimits{ReferencePrice: price, Range: priceRange}, nil
}

// priceRange returns the cached rule while it is fresh. Concurrent refreshes
// of one symbol share a request, and a failed refresh falls back to the
// expired rule when there is one.
func (service *Service) priceRange(ctx context.Context, symbol string) (*market.PriceRange, error) {
	service.mu.Lock()
	cached, ok := service.ranges[symbol]
	service.mu.Unlock()
	if ok && service.now().Sub(cached.fetchedAt) < rangeTTL {
		return cached.value, nil
	}
	result, err, _ := service.group.Do(symbol, func() (any, error) {
		// Callers share this request, so one caller's cancellation must not
		// fail the others.
		fetchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), fetchTimeout)
		defer cancel()
		value, err := service.exchange.PriceRange(fetchCtx, symbol)
		if err != nil {
			return nil, err
		}
		service.mu.Lock()
		service.ranges[symbol] = cachedRange{value: value, fetchedAt: service.now()}
		service.mu.Unlock()
		return value, nil
	})
	if err != nil {
		if ok {
			service.logger.WarnContext(ctx, "price range refresh failed; using the expired rule", "symbol", symbol, "error", err)
			return cached.value, nil
		}
		return nil, err
	}
	return result.(*market.PriceRange), nil
}
