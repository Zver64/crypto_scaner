// Package chart coordinates candle pages with transport-independent indicator calculations.
package chart

import (
	"context"
	"errors"
	"fmt"
	"sort"
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

type IndicatorConfig struct {
	Type       indicator.Type
	Parameters indicator.Parameters
}

type Request struct {
	Symbol     string
	Interval   market.CandleInterval
	Before     *time.Time
	Limit      int
	Indicators []IndicatorConfig
}

type Point struct {
	Time  time.Time
	Value float64
}

type Series struct {
	Name   string
	Points []Point
}

type IndicatorResult struct {
	Type       indicator.Type
	Parameters indicator.Parameters
	Series     []Series
}

type Page struct {
	Symbol     string
	Candles    []market.Candle
	Indicators []IndicatorResult
	HasMore    bool
	NextBefore *time.Time
}

type Service struct {
	store      Store
	calculator indicator.Calculator
}

func NewService(store Store, calculator indicator.Calculator) (*Service, error) {
	if store == nil || calculator == nil {
		return nil, fmt.Errorf("%w: store and calculator are required", ErrInvalidRequest)
	}
	return &Service{store: store, calculator: calculator}, nil
}

// Build loads candles exactly once, using the largest requested indicator
// lookback as hidden context, and returns only the visible page.
func (service *Service) Build(ctx context.Context, request Request) (Page, error) {
	symbol := strings.ToUpper(strings.TrimSpace(request.Symbol))
	if service == nil || symbol == "" || !request.Interval.Valid() || request.Limit <= 0 || len(request.Indicators) == 0 {
		return Page{}, fmt.Errorf("%w: symbol, interval, limit, and indicators are required", ErrInvalidRequest)
	}
	if len(request.Indicators) > maxIndicatorConfigs {
		return Page{}, fmt.Errorf("%w: at most %d indicators are allowed", ErrInvalidRequest, maxIndicatorConfigs)
	}

	lookback := 0
	for _, config := range request.Indicators {
		value, err := service.calculator.Lookback(config.Type, config.Parameters)
		if err != nil {
			return Page{}, fmt.Errorf("%w: lookback for %q: %v", ErrInvalidRequest, config.Type, err)
		}
		if value > maxChartLookback {
			return Page{}, fmt.Errorf("%w: lookback for %q exceeds %d candles", ErrInvalidRequest, config.Type, maxChartLookback)
		}
		if value > lookback {
			lookback = value
		}
	}
	if lookback > int(^uint(0)>>1)-request.Limit {
		return Page{}, fmt.Errorf("%w: candle limit is too large", ErrInvalidRequest)
	}

	instrument, err := service.store.GetActiveInstrumentBySymbol(ctx, symbol)
	if err != nil {
		return Page{}, fmt.Errorf("resolve chart instrument: %w", err)
	}
	stored, err := service.store.ListCandlePage(ctx, instrument.ID, request.Interval, request.Before, request.Limit+lookback)
	if err != nil {
		return Page{}, fmt.Errorf("list chart candles: %w", err)
	}

	hidden := len(stored.Candles) - request.Limit
	if hidden < 0 {
		hidden = 0
	}
	visible := append([]market.Candle(nil), stored.Candles[hidden:]...)
	results := make([]IndicatorResult, 0, len(request.Indicators))
	closeValues := make([]float64, len(stored.Candles))
	for index, candle := range stored.Candles {
		closeValues[index] = candle.Close
	}
	for _, config := range request.Indicators {
		calculated, calculateErr := service.calculator.Calculate(indicator.Request{
			Type: config.Type, Parameters: config.Parameters, Inputs: indicator.Inputs{"close": closeValues},
		})
		if calculateErr != nil {
			return Page{}, fmt.Errorf("%w: calculate %q: %v", ErrInvalidRequest, config.Type, calculateErr)
		}
		names := make([]string, 0, len(calculated.Outputs))
		for name := range calculated.Outputs {
			names = append(names, name)
		}
		sort.Strings(names)
		series := make([]Series, 0, len(names))
		for _, name := range names {
			output := calculated.Outputs[name]
			if len(output.Values) > 0 && (output.Offset >= len(stored.Candles) || len(output.Values) > len(stored.Candles)-output.Offset) {
				return Page{}, fmt.Errorf("indicator %q output %q exceeds candle input", config.Type, name)
			}
			points := make([]Point, 0, len(output.Values))
			for valueIndex, value := range output.Values {
				candleIndex := output.Offset + valueIndex
				if candleIndex < hidden {
					continue
				}
				points = append(points, Point{Time: stored.Candles[candleIndex].OpenTime.UTC(), Value: value})
			}
			series = append(series, Series{Name: name, Points: points})
		}
		results = append(results, IndicatorResult{Type: config.Type, Parameters: config.Parameters, Series: series})
	}

	hasMore := stored.HasMore || hidden > 0
	var nextBefore *time.Time
	if hasMore && len(visible) > 0 {
		value := visible[0].OpenTime.UTC()
		nextBefore = &value
	}
	return Page{Symbol: instrument.Symbol, Candles: visible, Indicators: results, HasMore: hasMore, NextBefore: nextBefore}, nil
}
