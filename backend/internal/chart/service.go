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

// Build calculates indicators over exactly the requested closed-candle range.
// A larger limit extends the range and replaces all previous indicator points.
func (service *Service) Build(ctx context.Context, request Request) (Page, error) {
	symbol := strings.ToUpper(strings.TrimSpace(request.Symbol))
	if service == nil || symbol == "" || !request.Interval.Valid() || request.Limit <= 0 || len(request.Indicators) == 0 {
		return Page{}, fmt.Errorf("%w: symbol, interval, limit, and indicators are required", ErrInvalidRequest)
	}
	if len(request.Indicators) > maxIndicatorConfigs {
		return Page{}, fmt.Errorf("%w: at most %d indicators are allowed", ErrInvalidRequest, maxIndicatorConfigs)
	}

	if request.Limit > maxChartLookback {
		return Page{}, fmt.Errorf("%w: candle limit exceeds %d", ErrInvalidRequest, maxChartLookback)
	}
	for _, config := range request.Indicators {
		value, err := service.calculator.Lookback(config.Type, config.Parameters)
		if err != nil {
			return Page{}, fmt.Errorf("%w: lookback for %q: %v", ErrInvalidRequest, config.Type, err)
		}
		if value > maxChartLookback {
			return Page{}, fmt.Errorf("%w: lookback for %q exceeds %d candles", ErrInvalidRequest, config.Type, maxChartLookback)
		}
	}

	instrument, err := service.store.GetActiveInstrumentBySymbol(ctx, symbol)
	if err != nil {
		return Page{}, fmt.Errorf("resolve chart instrument: %w", err)
	}
	stored, err := service.store.ListCandlePage(ctx, instrument.ID, request.Interval, request.Before, request.Limit)
	if err != nil {
		return Page{}, fmt.Errorf("list chart candles: %w", err)
	}

	results := make([]IndicatorResult, 0, len(request.Indicators))
	// Calculate each continuous run independently. A missing candle cannot be
	// treated as an adjacent close when warming up an indicator.
	starts := []int{0}
	for index := 1; index < len(stored.Candles); index++ {
		if !request.Interval.NextOpenTime(stored.Candles[index-1].OpenTime).Equal(stored.Candles[index].OpenTime) {
			starts = append(starts, index)
		}
	}
	starts = append(starts, len(stored.Candles))
	for _, config := range request.Indicators {
		byName := make(map[string][]Point)
		for run := 0; run+1 < len(starts); run++ {
			start, end := starts[run], starts[run+1]
			closes := make([]float64, end-start)
			for index := start; index < end; index++ {
				closes[index-start] = stored.Candles[index].Close
			}
			calculated, calculateErr := service.calculator.Calculate(indicator.Request{
				Type: config.Type, Parameters: config.Parameters, Inputs: indicator.Inputs{"close": closes},
			})
			if calculateErr != nil {
				return Page{}, fmt.Errorf("%w: calculate %q: %v", ErrInvalidRequest, config.Type, calculateErr)
			}
			for name, output := range calculated.Outputs {
				if _, exists := byName[name]; !exists {
					byName[name] = []Point{}
				}
				if len(output.Values) > 0 && (output.Offset >= end-start || len(output.Values) > end-start-output.Offset) {
					return Page{}, fmt.Errorf("indicator %q output %q exceeds candle input", config.Type, name)
				}
				for index, value := range output.Values {
					byName[name] = append(byName[name], Point{Time: stored.Candles[start+output.Offset+index].OpenTime.UTC(), Value: value})
				}
			}
		}
		names := make([]string, 0, len(byName))
		for name := range byName {
			names = append(names, name)
		}
		sort.Strings(names)
		series := make([]Series, 0, len(names))
		for _, name := range names {
			series = append(series, Series{Name: name, Points: byName[name]})
		}
		results = append(results, IndicatorResult{Type: config.Type, Parameters: config.Parameters, Series: series})
	}

	hasMore := stored.HasMore
	var nextBefore *time.Time
	if hasMore && len(stored.Candles) > 0 {
		value := stored.Candles[0].OpenTime.UTC()
		nextBefore = &value
	}
	return Page{Symbol: instrument.Symbol, Candles: stored.Candles, Indicators: results, HasMore: hasMore, NextBefore: nextBefore}, nil
}
