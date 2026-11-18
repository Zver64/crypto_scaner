package gridlimits_test

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

	"crypto-scanner/internal/market"
	"crypto-scanner/internal/market/gridlimits"
)

var errUnavailable = errors.New("unavailable")

type spotExchange struct{ err error }

func (exchange spotExchange) AveragePrice(context.Context, string) (float64, error) {
	return 100, exchange.err
}

func (exchange spotExchange) SpotFilters(context.Context, string) (gridlimits.SpotFilters, error) {
	return gridlimits.SpotFilters{TickSize: 0.01, MinQty: 0.001, StepSize: 0.001, MinNotional: 5}, exchange.err
}

type futures[C any] struct {
	mu             sync.Mutex
	markSymbols    []string
	contracts      map[string]C
	contractsErr   error
	markPriceErr   error
	contractCalls  int
	markPriceCalls int
}

func (futures *futures[C]) Contracts(context.Context) (map[string]C, error) {
	futures.mu.Lock()
	defer futures.mu.Unlock()
	futures.contractCalls++
	if futures.contractsErr != nil {
		return nil, futures.contractsErr
	}
	return futures.contracts, nil
}

func (futures *futures[C]) MarkPrice(_ context.Context, symbol string) (float64, error) {
	futures.mu.Lock()
	defer futures.mu.Unlock()
	futures.markPriceCalls++
	futures.markSymbols = append(futures.markSymbols, symbol)
	return 99, futures.markPriceErr
}

type instruments struct{}

func (instruments) GetActiveInstrumentBySymbol(_ context.Context, symbol string) (market.Instrument, error) {
	base, ok := market.SpotBaseAsset(symbol)
	if !ok {
		return market.Instrument{}, market.ErrInstrumentNotFound
	}
	return market.Instrument{Symbol: symbol, BaseAsset: base}, nil
}

var (
	usdmBTC  = gridlimits.UsdmContract{Symbol: "BTCUSDT", TickSize: 0.1, MinQty: 0.001, StepSize: 0.001, MinNotional: 100}
	coinmBTC = gridlimits.CoinmContract{Symbol: "BTCUSD_PERP", TickSize: 0.1, MinQty: 1, StepSize: 1, ContractSize: 100, EqualQtyPrecision: 4}
)

func newService(spot spotExchange, usdm *futures[gridlimits.UsdmContract], coinm *futures[gridlimits.CoinmContract], now *time.Time) *gridlimits.Service {
	return gridlimits.New(spot, usdm, coinm, instruments{}, slog.New(slog.DiscardHandler), gridlimits.Options{Now: func() time.Time { return *now }})
}

func btcFutures() (*futures[gridlimits.UsdmContract], *futures[gridlimits.CoinmContract]) {
	return &futures[gridlimits.UsdmContract]{contracts: map[string]gridlimits.UsdmContract{"BTC": usdmBTC}},
		&futures[gridlimits.CoinmContract]{contracts: map[string]gridlimits.CoinmContract{"BTC": coinmBTC}}
}

func TestLimitsServesThePerpetualContractsOfTheBaseAsset(t *testing.T) {
	now := time.Unix(0, 0)
	usdm, coinm := btcFutures()
	service := newService(spotExchange{}, usdm, coinm, &now)

	limits, err := service.Limits(context.Background(), "BTCUSDT")
	if err != nil {
		t.Fatal(err)
	}
	if want := (gridlimits.UsdmLimits{MarkPrice: 99, UsdmContract: usdmBTC}); limits.Usdm == nil || *limits.Usdm != want {
		t.Fatalf("usdm = %+v, want %+v", limits.Usdm, want)
	}
	if want := (gridlimits.CoinmLimits{MarkPrice: 99, CoinmContract: coinmBTC}); limits.Coinm == nil || *limits.Coinm != want {
		t.Fatalf("coinm = %+v, want %+v", limits.Coinm, want)
	}
	if !slices.Equal(usdm.markSymbols, []string{"BTCUSDT"}) || !slices.Equal(coinm.markSymbols, []string{"BTCUSD_PERP"}) {
		t.Fatalf("mark price symbols = %v, %v; want the contract symbols", usdm.markSymbols, coinm.markSymbols)
	}
	if limits.Spot.MinNotional != 5 || limits.Spot.AveragePrice != 100 {
		t.Fatalf("spot = %+v", limits.Spot)
	}

	limits, err = service.Limits(context.Background(), "ETHUSDT")
	if err != nil {
		t.Fatal(err)
	}
	if limits.Usdm != nil || limits.Coinm != nil {
		t.Fatalf("futures without contracts = %+v, %+v, want nil", limits.Usdm, limits.Coinm)
	}
}

func TestLimitsLeavesAFailingFuturesMarketNull(t *testing.T) {
	now := time.Unix(0, 0)
	usdm, coinm := btcFutures()
	usdm.contractsErr = errUnavailable
	coinm.markPriceErr = errUnavailable
	service := newService(spotExchange{}, usdm, coinm, &now)

	limits, err := service.Limits(context.Background(), "BTCUSDT")
	if err != nil {
		t.Fatal(err)
	}
	if limits.Usdm != nil || limits.Coinm != nil {
		t.Fatalf("failing futures = %+v, %+v, want nil", limits.Usdm, limits.Coinm)
	}

	failingSpot := newService(spotExchange{err: errUnavailable}, usdm, coinm, &now)
	if _, err := failingSpot.Limits(context.Background(), "BTCUSDT"); !errors.Is(err, errUnavailable) {
		t.Fatalf("spot failure = %v, want %v", err, errUnavailable)
	}
}

func TestLimitsCachesContractsForADayAndMarkPricesForSeconds(t *testing.T) {
	now := time.Unix(0, 0)
	usdm, coinm := btcFutures()
	service := newService(spotExchange{}, usdm, coinm, &now)
	request := func() gridlimits.Limits {
		t.Helper()
		limits, err := service.Limits(context.Background(), "BTCUSDT")
		if err != nil {
			t.Fatal(err)
		}
		return limits
	}

	request()
	request()
	if usdm.contractCalls != 1 || usdm.markPriceCalls != 1 {
		t.Fatalf("calls within the price TTL = %d contracts, %d mark prices; want 1, 1", usdm.contractCalls, usdm.markPriceCalls)
	}

	now = now.Add(6 * time.Second)
	request()
	if usdm.contractCalls != 1 || usdm.markPriceCalls != 2 {
		t.Fatalf("calls after the price TTL = %d contracts, %d mark prices; want 1, 2", usdm.contractCalls, usdm.markPriceCalls)
	}

	now = now.Add(25 * time.Hour)
	usdm.contractsErr = errUnavailable
	if limits := request(); limits.Usdm == nil || limits.Usdm.UsdmContract != usdmBTC {
		t.Fatalf("usdm after a failed refresh = %+v, want the expired contract", limits.Usdm)
	}
	if usdm.contractCalls != 2 {
		t.Fatalf("contract calls after the filters TTL = %d, want 2", usdm.contractCalls)
	}

	usdm.markPriceErr = errUnavailable
	now = now.Add(6 * time.Second)
	if limits := request(); limits.Usdm != nil {
		t.Fatalf("usdm with a failed mark price = %+v, want nil: an expired mark price is never served", limits.Usdm)
	}
}
