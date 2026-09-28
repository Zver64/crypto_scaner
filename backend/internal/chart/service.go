// Package chart coordinates candle pages with the shared indicator registry.
package chart

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
)

var ErrInvalidRequest = errors.New("invalid chart request")

const (
	// DefaultRange is the initial chart range; larger ranges come from scroll-back.
	DefaultRange = 200
	// MaxRange bounds the number of closed candles in one chart.
	MaxRange = 5000
	// maxIndicators bounds the indicator selection of one chart.
	maxIndicators        = 8
	maxIndicatorLookback = 5000
)

type Store interface {
	GetActiveInstrumentBySymbol(context.Context, string) (market.Instrument, error)
	ListCandlePage(context.Context, int64, market.CandleInterval, *time.Time, int) (market.CandlePage, error)
}
type Request struct {
	Symbol     string
	Interval   market.CandleInterval
	Limit      int
	Indicators []indicator.Selection
}
type Page struct {
	Symbol     string
	Candles    []market.Candle
	Indicators []indicator.Calculation
	HasMore    bool
	NextBefore *time.Time
	// warmup holds the closed candles before Candles that indicators need to
	// settle. They are calculated over but never returned to clients.
	warmup []market.Candle
}
type Service struct {
	store      Store
	indicators *indicator.Registry
	catalog    []CatalogIndicator
	logger     *slog.Logger
}

// NewService validates the indicator catalog against the registry, so an
// inconsistent catalog fails at startup instead of on the first chart.
func NewService(store Store, indicators *indicator.Registry, catalog []CatalogIndicator, logger *slog.Logger) (*Service, error) {
	if store == nil || indicators == nil || logger == nil {
		return nil, errors.New("chart store, indicator registry, and logger are required")
	}
	if err := validateCatalog(indicators, catalog); err != nil {
		return nil, fmt.Errorf("invalid chart indicator catalog: %w", err)
	}
	return &Service{store: store, indicators: indicators, catalog: slices.Clone(catalog), logger: logger.With("module", "chart")}, nil
}

// Catalog returns the indicators clients should request and how to draw them.
func (service *Service) Catalog() []CatalogIndicator {
	return slices.Clone(service.catalog)
}
func (service *Service) Build(ctx context.Context, request Request) (Page, error) {
	symbol := market.NormalizeSymbol(request.Symbol)
	if service == nil || symbol == "" || !request.Interval.Valid() || request.Limit <= 0 || len(request.Indicators) == 0 {
		return Page{}, fmt.Errorf("%w: symbol, interval, limit, and indicators are required", ErrInvalidRequest)
	}
	if request.Limit > MaxRange {
		return Page{}, fmt.Errorf("%w: chart range exceeds limit", ErrInvalidRequest)
	}
	warmup, err := service.warmup(request.Indicators)
	if err != nil {
		return Page{}, err
	}
	instrument, err := service.store.GetActiveInstrumentBySymbol(ctx, symbol)
	if err != nil {
		return Page{}, fmt.Errorf("resolve chart instrument: %w", err)
	}
	stored, err := service.store.ListCandlePage(ctx, instrument.ID, request.Interval, nil, request.Limit+warmup)
	if err != nil {
		return Page{}, fmt.Errorf("list chart candles: %w", err)
	}
	split := max(0, len(stored.Candles)-request.Limit)
	page := Page{
		Symbol:  instrument.Symbol,
		Candles: stored.Candles[split:],
		HasMore: stored.HasMore || split > 0,
		warmup:  stored.Candles[:split],
	}
	page.NextBefore = market.CandlePage{Candles: page.Candles, HasMore: page.HasMore}.NextBefore()
	if page.Indicators, err = service.calculate(request.Interval, page.warmup, page.Candles, request.Indicators); err != nil {
		return Page{}, fmt.Errorf("%w: calculate: %w", ErrInvalidRequest, err)
	}
	return page, nil
}

// Validate checks an indicator selection before any history is loaded.
func (service *Service) Validate(configs []indicator.Selection) error {
	if len(configs) == 0 || len(configs) > maxIndicators {
		return fmt.Errorf("%w: indicator count must be between 1 and %d", ErrInvalidRequest, maxIndicators)
	}
	for _, config := range configs {
		value, err := service.indicators.Lookback(config.Type, config.Parameters)
		if err != nil {
			return fmt.Errorf("%w: lookback for %q: %w", ErrInvalidRequest, config.Type, err)
		}
		if value > maxIndicatorLookback {
			return fmt.Errorf("%w: lookback for %q exceeds %d", ErrInvalidRequest, config.Type, maxIndicatorLookback)
		}
	}
	return nil
}

// warmup validates the selection and returns the largest lookback, the number
// of closed candles loaded before the visible range.
func (service *Service) warmup(configs []indicator.Selection) (int, error) {
	if err := service.Validate(configs); err != nil {
		return 0, err
	}
	largest := 0
	for _, config := range configs {
		value, err := service.indicators.Lookback(config.Type, config.Parameters)
		if err != nil {
			return 0, err
		}
		largest = max(largest, value)
	}
	return largest, nil
}

// calculate runs indicators over the warm-up and visible candles and keeps only
// the points of visible candles.
func (service *Service) calculate(interval market.CandleInterval, warmup, candles []market.Candle, configs []indicator.Selection) ([]indicator.Calculation, error) {
	all := append(append([]market.Candle(nil), warmup...), candles...)
	results, err := service.indicators.CalculateCandles(interval, all, configs)
	if err != nil || len(warmup) == 0 {
		return results, err
	}
	var from time.Time
	if len(candles) > 0 {
		from = candles[0].OpenTime
	}
	for i := range results {
		for j := range results[i].Series {
			points := results[i].Series[j].Points
			first, _ := slices.BinarySearchFunc(points, from, func(point indicator.Point, target time.Time) int {
				return point.Time.Compare(target)
			})
			if len(candles) == 0 {
				first = len(points)
			}
			results[i].Series[j].Points = points[first:]
		}
	}
	return results, nil
}

// extend calculates a private live context. It never mutates closed history.
func (service *Service) extend(page Page, interval market.CandleInterval, live *market.Candle, configs []indicator.Selection) (Page, error) {
	candles := append([]market.Candle(nil), page.Candles...)
	if live != nil {
		if len(candles) > 0 && candles[len(candles)-1].OpenTime.Equal(live.OpenTime) {
			candles[len(candles)-1] = *live
		} else if len(candles) == 0 || interval.NextOpenTime(candles[len(candles)-1].OpenTime).Equal(live.OpenTime) {
			candles = append(candles, *live)
		}
	}
	results, err := service.calculate(interval, page.warmup, candles, configs)
	if err != nil {
		return Page{}, err
	}
	page.Candles = candles
	page.Indicators = results
	return page, nil
}
