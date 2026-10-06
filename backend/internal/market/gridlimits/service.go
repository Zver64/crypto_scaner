// Package gridlimits serves what a Binance spot grid bot derives its price
// range limits from: the average price, cached per symbol for a few seconds,
// and the order price filters, which change rarely and are cached per symbol
// for a day after a client asks for them. Clients apply the bot's formulas.
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
	// averagePriceTTL is how long a fetched average price is reused, so
	// repeated requests do not spend the shared Binance request weight.
	averagePriceTTL = 5 * time.Second
	// fetchTimeout bounds a shared exchange request.
	fetchTimeout = 10 * time.Second
)

// PriceFilters are the order price filters of a symbol. A down multiplier is
// the lowest share of the average price a buy order may have, an up
// multiplier the highest share a sell order may have; each is zero when the
// symbol has no such filter.
type PriceFilters struct {
	MinPrice          float64 // PRICE_FILTER
	MaxPrice          float64 // PRICE_FILTER
	TickSize          float64 // PRICE_FILTER
	BidMultiplierDown float64 // PERCENT_PRICE_BY_SIDE
	AskMultiplierUp   float64 // PERCENT_PRICE_BY_SIDE
	MultiplierDown    float64 // PERCENT_PRICE
	MultiplierUp      float64 // PERCENT_PRICE
}

// Limits are the current average price of a symbol and the order price
// filters a spot grid bot derives its price range limits from. The
// multipliers are the stricter of the two percent price filters, zero when
// the symbol has neither.
type Limits struct {
	Symbol            string
	AveragePrice      float64
	BidMultiplierDown float64
	AskMultiplierUp   float64
	MinPrice          float64
	MaxPrice          float64
	TickSize          float64
}

// Exchange reads the average price and price filters of a symbol.
type Exchange interface {
	AveragePrice(context.Context, string) (float64, error)
	PriceFilters(context.Context, string) (PriceFilters, error)
}

// Instruments finds active instruments; an unknown or inactive symbol is
// market.ErrInstrumentNotFound.
type Instruments interface {
	GetActiveInstrumentBySymbol(context.Context, string) (market.Instrument, error)
}

type Options struct {
	Now func() time.Time
}

type cached[T any] struct {
	value     T
	fetchedAt time.Time
}

// cache keeps one value per symbol for ttl. Concurrent refreshes of a symbol
// share one request.
type cache[T any] struct {
	ttl   time.Duration
	group singleflight.Group

	mu      sync.Mutex
	entries map[string]cached[T]
}

// get returns the fresh cached value or fetches a new one. When the fetch
// fails, it also returns the expired value, if any, with expired set.
func (cache *cache[T]) get(ctx context.Context, now func() time.Time, symbol string, fetch func(context.Context, string) (T, error)) (value T, expired bool, err error) {
	cache.mu.Lock()
	entry, ok := cache.entries[symbol]
	cache.mu.Unlock()
	if ok && now().Sub(entry.fetchedAt) < cache.ttl {
		return entry.value, false, nil
	}
	result, err, _ := cache.group.Do(symbol, func() (any, error) {
		// Callers share this request, so one caller's cancellation must not
		// fail the others.
		fetchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), fetchTimeout)
		defer cancel()
		value, err := fetch(fetchCtx, symbol)
		if err != nil {
			return nil, err
		}
		cache.mu.Lock()
		cache.entries[symbol] = cached[T]{value: value, fetchedAt: now()}
		cache.mu.Unlock()
		return value, nil
	})
	if err != nil {
		return entry.value, ok, err
	}
	return result.(T), false, nil
}

// Service checks that a symbol is an active instrument and serves its limits
// from the per-symbol caches.
type Service struct {
	exchange    Exchange
	instruments Instruments
	logger      *slog.Logger
	now         func() time.Time
	prices      cache[float64]
	filters     cache[PriceFilters]
}

func New(exchange Exchange, instruments Instruments, logger *slog.Logger, options Options) *Service {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Service{
		exchange: exchange, instruments: instruments, logger: logger.With("module", "gridlimits"), now: now,
		prices:  cache[float64]{ttl: averagePriceTTL, entries: make(map[string]cached[float64])},
		filters: cache[PriceFilters]{ttl: filtersTTL, entries: make(map[string]cached[PriceFilters])},
	}
}

// Limits returns the current grid limits of a normalized symbol. A symbol
// that is not an active instrument is market.ErrInstrumentNotFound.
func (service *Service) Limits(ctx context.Context, symbol string) (Limits, error) {
	instrument, err := service.instruments.GetActiveInstrumentBySymbol(ctx, symbol)
	if err != nil {
		return Limits{}, err
	}
	symbol = instrument.Symbol
	var average float64
	var filters PriceFilters
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() (err error) {
		// An expired average price is never served: it bounds live orders.
		average, _, err = service.prices.get(groupCtx, service.now, symbol, service.exchange.AveragePrice)
		return err
	})
	group.Go(func() error {
		value, expired, err := service.filters.get(groupCtx, service.now, symbol, service.exchange.PriceFilters)
		if err != nil && expired {
			service.logger.WarnContext(ctx, "price filters refresh failed; using the expired filters", "symbol", symbol, "error", err)
			err = nil
		}
		filters = value
		return err
	})
	if err := group.Wait(); err != nil {
		return Limits{}, err
	}
	return Limits{
		Symbol:            symbol,
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
