// Package chart coordinates candle pages with the shared indicator registry.
package chart

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
}
type Service struct {
	store      Store
	indicators *indicator.Registry
	logger     *slog.Logger
}

func NewService(store Store, indicators *indicator.Registry, logger *slog.Logger) (*Service, error) {
	if store == nil || indicators == nil || logger == nil {
		return nil, errors.New("chart store, indicator registry, and logger are required")
	}
	return &Service{store: store, indicators: indicators, logger: logger.With("module", "chart")}, nil
}
func (service *Service) Build(ctx context.Context, request Request) (Page, error) {
	symbol := market.NormalizeSymbol(request.Symbol)
	if service == nil || symbol == "" || !request.Interval.Valid() || request.Limit <= 0 || len(request.Indicators) == 0 {
		return Page{}, fmt.Errorf("%w: symbol, interval, limit, and indicators are required", ErrInvalidRequest)
	}
	if request.Limit > MaxRange {
		return Page{}, fmt.Errorf("%w: chart range exceeds limit", ErrInvalidRequest)
	}
	if err := service.Validate(request.Indicators); err != nil {
		return Page{}, err
	}
	instrument, err := service.store.GetActiveInstrumentBySymbol(ctx, symbol)
	if err != nil {
		return Page{}, fmt.Errorf("resolve chart instrument: %w", err)
	}
	stored, err := service.store.ListCandlePage(ctx, instrument.ID, request.Interval, nil, request.Limit)
	if err != nil {
		return Page{}, fmt.Errorf("list chart candles: %w", err)
	}
	results, err := service.indicators.CalculateCandles(request.Interval, stored.Candles, request.Indicators)
	if err != nil {
		return Page{}, fmt.Errorf("%w: calculate: %w", ErrInvalidRequest, err)
	}
	return Page{Symbol: instrument.Symbol, Candles: stored.Candles, Indicators: results, HasMore: stored.HasMore, NextBefore: stored.NextBefore()}, nil
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
	results, err := service.indicators.CalculateCandles(interval, candles, configs)
	if err != nil {
		return Page{}, err
	}
	page.Candles = candles
	page.Indicators = results
	return page, nil
}
