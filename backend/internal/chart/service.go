// Package chart coordinates candle pages with the shared indicator engine.
package chart

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
)

var ErrInvalidRequest = errors.New("invalid chart request")

const (
	maxIndicatorConfigs = 8
	maxChartLookback    = 5000
)

type Store interface {
	GetActiveInstrumentBySymbol(context.Context, string) (market.Instrument, error)
	ListCandlePage(context.Context, int64, market.CandleInterval, *time.Time, int) (market.CandlePage, error)
}
type IndicatorConfig = indicator.Selection
type Request struct {
	Symbol     string
	Interval   market.CandleInterval
	Limit      int
	Indicators []IndicatorConfig
}
type Point = indicator.Point
type Series = indicator.NamedSeries
type IndicatorResult = indicator.Calculation
type Page struct {
	Symbol     string
	Candles    []market.Candle
	Indicators []IndicatorResult
	HasMore    bool
	NextBefore *time.Time
}
type Service struct {
	store  Store
	engine *indicator.Engine
}

func NewService(store Store, calculator indicator.Calculator) (*Service, error) {
	if store == nil || calculator == nil {
		return nil, fmt.Errorf("%w: store and calculator are required", ErrInvalidRequest)
	}
	return &Service{store: store, engine: indicator.NewEngine(calculator)}, nil
}
func (service *Service) Build(ctx context.Context, request Request) (Page, error) {
	symbol := strings.ToUpper(strings.TrimSpace(request.Symbol))
	if service == nil || symbol == "" || !request.Interval.Valid() || request.Limit <= 0 || len(request.Indicators) == 0 {
		return Page{}, fmt.Errorf("%w: symbol, interval, limit, and indicators are required", ErrInvalidRequest)
	}
	if request.Limit > maxChartLookback {
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
	results, err := service.engine.Calculate(request.Interval, stored.Candles, request.Indicators)
	if err != nil {
		return Page{}, fmt.Errorf("%w: calculate: %v", ErrInvalidRequest, err)
	}
	var nextBefore *time.Time
	if stored.HasMore && len(stored.Candles) > 0 {
		value := stored.Candles[0].OpenTime.UTC()
		nextBefore = &value
	}
	return Page{Symbol: instrument.Symbol, Candles: stored.Candles, Indicators: results, HasMore: stored.HasMore, NextBefore: nextBefore}, nil
}

// Validate checks an indicator selection before any history is loaded.
func (service *Service) Validate(configs []IndicatorConfig) error {
	if len(configs) == 0 || len(configs) > maxIndicatorConfigs {
		return fmt.Errorf("%w: indicator count must be between 1 and %d", ErrInvalidRequest, maxIndicatorConfigs)
	}
	for _, config := range configs {
		value, err := service.engine.Lookback(config.Type, config.Parameters)
		if err != nil || value > maxChartLookback {
			return fmt.Errorf("%w: lookback for %q: %v", ErrInvalidRequest, config.Type, err)
		}
	}
	return nil
}

// Extend calculates a private live context. It never mutates closed history.
func (service *Service) Extend(page Page, interval market.CandleInterval, live *market.Candle, configs []IndicatorConfig) (Page, error) {
	candles := append([]market.Candle(nil), page.Candles...)
	if live != nil {
		if len(candles) > 0 && candles[len(candles)-1].OpenTime.Equal(live.OpenTime) {
			candles[len(candles)-1] = *live
		} else if len(candles) == 0 || interval.NextOpenTime(candles[len(candles)-1].OpenTime).Equal(live.OpenTime) {
			candles = append(candles, *live)
		}
	}
	results, err := service.engine.Calculate(interval, candles, configs)
	if err != nil {
		return Page{}, err
	}
	page.Candles = candles
	page.Indicators = results
	return page, nil
}
