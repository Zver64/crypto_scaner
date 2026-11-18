// Package gridlimits serves what Binance grid bots derive their price range
// limits and minimum investment from, for the spot market and the USD-M and
// COIN-M perpetual contracts of an instrument's base asset. Prices (the spot
// average price, futures mark prices) are cached per symbol for a few
// seconds; order filters change rarely and are cached for a day after a
// client asks for them. Clients apply the bots' formulas.
package gridlimits

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"crypto-scanner/internal/market"
	"crypto-scanner/internal/platform/logging"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
)

const (
	// filtersTTL is how long fetched order filters are reused.
	filtersTTL = 24 * time.Hour
	// priceTTL is how long a fetched average or mark price is reused, so
	// repeated requests do not spend the shared Binance request weight.
	priceTTL = 5 * time.Second
	// fetchTimeout bounds a shared exchange request.
	fetchTimeout = 10 * time.Second
)

// SpotFilters are the order filters of a spot symbol. A down multiplier is
// the lowest share of the average price a buy order may have, an up
// multiplier the highest share a sell order may have; each value is zero
// when the symbol has no such filter.
type SpotFilters struct {
	MinPrice          float64 // PRICE_FILTER
	MaxPrice          float64 // PRICE_FILTER
	TickSize          float64 // PRICE_FILTER
	BidMultiplierDown float64 // PERCENT_PRICE_BY_SIDE
	AskMultiplierUp   float64 // PERCENT_PRICE_BY_SIDE
	MultiplierDown    float64 // PERCENT_PRICE
	MultiplierUp      float64 // PERCENT_PRICE
	MinQty            float64 // LOT_SIZE
	StepSize          float64 // LOT_SIZE
	MinNotional       float64 // NOTIONAL, else MIN_NOTIONAL
}

// UsdmContract is a trading USD-M USDT perpetual contract and its order
// filters; a filter value is zero when the contract has no such filter.
type UsdmContract struct {
	Symbol      string
	TickSize    float64 // PRICE_FILTER
	MinQty      float64 // LOT_SIZE
	StepSize    float64 // LOT_SIZE
	MinNotional float64 // MIN_NOTIONAL
}

// CoinmContract is a trading COIN-M USD perpetual contract and its order
// filters; its quantities are in contracts of ContractSize USD each.
type CoinmContract struct {
	Symbol            string
	TickSize          float64 // PRICE_FILTER
	MinQty            float64 // LOT_SIZE
	StepSize          float64 // LOT_SIZE
	ContractSize      float64
	EqualQtyPrecision int
}

// SpotLimits are the current average price of a spot symbol and the order
// filters a spot grid bot derives its limits from. The multipliers are the
// stricter of the two percent price filters, zero when the symbol has
// neither.
type SpotLimits struct {
	AveragePrice      float64
	BidMultiplierDown float64
	AskMultiplierUp   float64
	MinPrice          float64
	MaxPrice          float64
	TickSize          float64
	MinQty            float64
	StepSize          float64
	MinNotional       float64
}

// UsdmLimits are the mark price and filters of a USD-M perpetual contract.
type UsdmLimits struct {
	MarkPrice float64
	UsdmContract
}

// CoinmLimits are the mark price and filters of a COIN-M perpetual contract.
type CoinmLimits struct {
	MarkPrice float64
	CoinmContract
}

// Limits are the grid limits of an instrument. A futures market is nil when
// the instrument's base asset has no trading USDT (USD-M) or USD (COIN-M)
// perpetual contract there or its data is unavailable.
type Limits struct {
	Symbol string
	Spot   SpotLimits
	Usdm   *UsdmLimits
	Coinm  *CoinmLimits
}

// Exchange reads the average price and order filters of a spot symbol.
type Exchange interface {
	AveragePrice(context.Context, string) (float64, error)
	SpotFilters(context.Context, string) (SpotFilters, error)
}

// Futures reads the trading perpetual contracts of a futures market, keyed
// by base asset, and the mark price of a contract symbol.
type Futures[C any] interface {
	Contracts(context.Context) (map[string]C, error)
	MarkPrice(context.Context, string) (float64, error)
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

// cache keeps one value per key for ttl. Concurrent refreshes of a key share
// one request.
type cache[T any] struct {
	ttl   time.Duration
	group singleflight.Group

	mu      sync.Mutex
	entries map[string]cached[T]
}

func newCache[T any](ttl time.Duration) cache[T] {
	return cache[T]{ttl: ttl, entries: make(map[string]cached[T])}
}

// get returns the fresh cached value or fetches a new one. When the fetch
// fails, it also returns the expired value, if any, with expired set.
func (cache *cache[T]) get(ctx context.Context, now func() time.Time, key string, fetch func(context.Context, string) (T, error)) (value T, expired bool, err error) {
	cache.mu.Lock()
	entry, ok := cache.entries[key]
	cache.mu.Unlock()
	if ok && now().Sub(entry.fetchedAt) < cache.ttl {
		return entry.value, false, nil
	}
	result, err, _ := cache.group.Do(key, func() (any, error) {
		// Callers share this request, so one caller's cancellation must not
		// fail the others.
		fetchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), fetchTimeout)
		defer cancel()
		value, err := fetch(fetchCtx, key)
		if err != nil {
			return nil, err
		}
		cache.mu.Lock()
		cache.entries[key] = cached[T]{value: value, fetchedAt: now()}
		cache.mu.Unlock()
		return value, nil
	})
	if err != nil {
		return entry.value, ok, err
	}
	return result.(T), false, nil
}

// futuresMarket caches the contracts of one futures market, all fetched at
// once, and the mark prices of its contracts.
type futuresMarket[C any] struct {
	name     string
	exchange Futures[C]
	// symbol names a contract's symbol.
	symbol     func(C) string
	contracts  cache[map[string]C]
	markPrices cache[float64]
	// failures warns once per base asset while its data keeps failing.
	failures *logging.Failures[string]
}

func newFuturesMarket[C any](name string, exchange Futures[C], symbol func(C) string, logger *slog.Logger) *futuresMarket[C] {
	return &futuresMarket[C]{
		name: name, exchange: exchange, symbol: symbol, failures: logging.NewFailures[string](logger),
		contracts: newCache[map[string]C](filtersTTL), markPrices: newCache[float64](priceTTL),
	}
}

// contract returns the contract of a base asset and its mark price; ok is
// false when the market has no such trading contract or its data is
// unavailable, which is logged once per base asset while it keeps failing.
func (futures *futuresMarket[C]) contract(ctx context.Context, now func() time.Time, baseAsset string) (contract C, markPrice float64, ok bool) {
	contracts, expired, err := futures.contracts.get(ctx, now, "", func(ctx context.Context, _ string) (map[string]C, error) {
		return futures.exchange.Contracts(ctx)
	})
	if err != nil && !expired {
		futures.failures.Warn(ctx, baseAsset, "futures contracts unavailable; serving grid limits without them", "market", futures.name, "base_asset", baseAsset, "error", err)
		return contract, 0, false
	}
	contract, ok = contracts[baseAsset]
	var markErr error
	if ok {
		// An expired mark price is never served: it bounds live orders.
		markPrice, _, markErr = futures.markPrices.get(ctx, now, futures.symbol(contract), futures.exchange.MarkPrice)
	}
	switch {
	case markErr != nil:
		futures.failures.Warn(ctx, baseAsset, "futures mark price unavailable; serving grid limits without it", "market", futures.name, "symbol", futures.symbol(contract), "error", markErr)
		return contract, 0, false
	case err != nil:
		futures.failures.Warn(ctx, baseAsset, "futures contracts refresh failed; using the expired contracts", "market", futures.name, "base_asset", baseAsset, "error", err)
	default:
		futures.failures.Forget(baseAsset)
	}
	return contract, markPrice, ok
}

// Service checks that a symbol is an active instrument and serves its limits
// from the per-symbol caches.
type Service struct {
	exchange    Exchange
	instruments Instruments
	now         func() time.Time
	prices      cache[float64]
	filters     cache[SpotFilters]
	// filterFailures warns once per symbol while its filters refresh keeps
	// failing.
	filterFailures *logging.Failures[string]
	usdm           *futuresMarket[UsdmContract]
	coinm          *futuresMarket[CoinmContract]
}

func New(exchange Exchange, usdm Futures[UsdmContract], coinm Futures[CoinmContract], instruments Instruments, logger *slog.Logger, options Options) *Service {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	logger = logger.With("module", "gridlimits")
	return &Service{
		exchange: exchange, instruments: instruments, now: now, filterFailures: logging.NewFailures[string](logger),
		prices:  newCache[float64](priceTTL),
		filters: newCache[SpotFilters](filtersTTL),
		usdm:    newFuturesMarket("USD-M", usdm, func(contract UsdmContract) string { return contract.Symbol }, logger),
		coinm:   newFuturesMarket("COIN-M", coinm, func(contract CoinmContract) string { return contract.Symbol }, logger),
	}
}

// Limits returns the current grid limits of a normalized symbol. A symbol
// that is not an active instrument is market.ErrInstrumentNotFound; a spot
// failure fails the request, a futures failure only leaves that market nil.
func (service *Service) Limits(ctx context.Context, symbol string) (Limits, error) {
	instrument, err := service.instruments.GetActiveInstrumentBySymbol(ctx, symbol)
	if err != nil {
		return Limits{}, err
	}
	limits := Limits{Symbol: instrument.Symbol}
	var futures sync.WaitGroup
	futures.Go(func() {
		if contract, markPrice, ok := service.usdm.contract(ctx, service.now, instrument.BaseAsset); ok {
			limits.Usdm = &UsdmLimits{MarkPrice: markPrice, UsdmContract: contract}
		}
	})
	futures.Go(func() {
		if contract, markPrice, ok := service.coinm.contract(ctx, service.now, instrument.BaseAsset); ok {
			limits.Coinm = &CoinmLimits{MarkPrice: markPrice, CoinmContract: contract}
		}
	})
	spot, err := service.spot(ctx, instrument.Symbol)
	futures.Wait()
	if err != nil {
		return Limits{}, err
	}
	limits.Spot = spot
	return limits, nil
}

func (service *Service) spot(ctx context.Context, symbol string) (SpotLimits, error) {
	var average float64
	var filters SpotFilters
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() (err error) {
		// An expired average price is never served: it bounds live orders.
		average, _, err = service.prices.get(groupCtx, service.now, symbol, service.exchange.AveragePrice)
		return err
	})
	group.Go(func() error {
		value, expired, err := service.filters.get(groupCtx, service.now, symbol, service.exchange.SpotFilters)
		if err != nil && expired {
			service.filterFailures.Warn(ctx, symbol, "price filters refresh failed; using the expired filters", "symbol", symbol, "error", err)
			err = nil
		} else if err == nil {
			service.filterFailures.Forget(symbol)
		}
		filters = value
		return err
	})
	if err := group.Wait(); err != nil {
		return SpotLimits{}, err
	}
	return SpotLimits{
		AveragePrice:      average,
		BidMultiplierDown: max(filters.BidMultiplierDown, filters.MultiplierDown),
		AskMultiplierUp:   stricterUp(filters.AskMultiplierUp, filters.MultiplierUp),
		MinPrice:          filters.MinPrice,
		MaxPrice:          filters.MaxPrice,
		TickSize:          filters.TickSize,
		MinQty:            filters.MinQty,
		StepSize:          filters.StepSize,
		MinNotional:       filters.MinNotional,
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
