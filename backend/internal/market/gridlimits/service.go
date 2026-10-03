// Package gridlimits serves what a Binance spot grid bot derives its price
// range limits from: the average price, requested on every call, and the
// order price filters, which change rarely and are cached per symbol for a day
// after a client asks for them. Clients apply the bot's formulas.
package gridlimits

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"crypto-scanner/internal/market"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
)

const (
	// filtersTTL is how long fetched price filters are reused.
	filtersTTL = 24 * time.Hour
	// fetchTimeout bounds a shared price filters request.
	fetchTimeout = 10 * time.Second
)

// Exchange reads the average price and price filters of a symbol.
type Exchange interface {
	AveragePrice(context.Context, string) (float64, error)
	PriceFilters(context.Context, string) (market.PriceFilters, error)
}

type Options struct {
	Now func() time.Time
}

type cachedFilters struct {
	value     market.PriceFilters
	fetchedAt time.Time
}

// Service requests the average price on every call and refreshes a symbol's
// price filters only when a call finds them missing or older than a day.
type Service struct {
	exchange Exchange
	logger   *slog.Logger
	now      func() time.Time
	group    singleflight.Group

	mu      sync.Mutex
	filters map[string]cachedFilters
}

func New(exchange Exchange, logger *slog.Logger, options Options) *Service {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Service{exchange: exchange, logger: logger.With("module", "gridlimits"), now: now, filters: make(map[string]cachedFilters)}
}

// Limits returns the current grid limits of symbol. An unknown symbol is
// market.ErrInstrumentNotFound.
func (service *Service) Limits(ctx context.Context, symbol string) (market.GridLimits, error) {
	symbol = market.NormalizeSymbol(symbol)
	var average float64
	var filters market.PriceFilters
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() (err error) {
		average, err = service.exchange.AveragePrice(groupCtx, symbol)
		return err
	})
	group.Go(func() (err error) {
		filters, err = service.priceFilters(groupCtx, symbol)
		return err
	})
	if err := group.Wait(); err != nil {
		return market.GridLimits{}, err
	}
	return market.GridLimits{
		AveragePrice:      average,
		BidMultiplierDown: max(filters.BidMultiplierDown, filters.MultiplierDown),
		AskMultiplierUp:   stricterUp(filters.AskMultiplierUp, filters.MultiplierUp),
		MinPrice:          filters.MinPrice,
		MaxPrice:          filters.MaxPrice,
		TickSize:          filters.TickSize,
	}, nil
}

// stricterUp returns the smaller of two up multipliers, ignoring a missing
// (zero) one.
func stricterUp(first, second float64) float64 {
	if first == 0 || second == 0 {
		return max(first, second)
	}
	return min(first, second)
}

// priceFilters returns the cached filters while they are fresh. Concurrent
// refreshes of one symbol share a request, and a failed refresh falls back to
// the expired filters when there are any.
func (service *Service) priceFilters(ctx context.Context, symbol string) (market.PriceFilters, error) {
	service.mu.Lock()
	cached, ok := service.filters[symbol]
	service.mu.Unlock()
	if ok && service.now().Sub(cached.fetchedAt) < filtersTTL {
		return cached.value, nil
	}
	result, err, _ := service.group.Do(symbol, func() (any, error) {
		// Callers share this request, so one caller's cancellation must not
		// fail the others.
		fetchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), fetchTimeout)
		defer cancel()
		value, err := service.exchange.PriceFilters(fetchCtx, symbol)
		if err != nil {
			return nil, err
		}
		service.mu.Lock()
		service.filters[symbol] = cachedFilters{value: value, fetchedAt: service.now()}
		service.mu.Unlock()
		return value, nil
	})
	if err != nil {
		if ok {
			service.logger.WarnContext(ctx, "price filters refresh failed; using the expired filters", "symbol", symbol, "error", err)
			return cached.value, nil
		}
		return market.PriceFilters{}, err
	}
	return result.(market.PriceFilters), nil
}
