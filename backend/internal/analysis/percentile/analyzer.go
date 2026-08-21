package percentile

import (
	"context"
	"math"
	"sort"

	"crypto-scanner/internal/analysis"
	"crypto-scanner/internal/market"
)

// Analyzer calculates candle-range percentiles.
type Analyzer struct{}

var _ analysis.Analyzer = Analyzer{}

// New constructs a candle-range percentile analyzer.
func New() Analyzer {
	return Analyzer{}
}

// Name identifies the analyzer in application composition.
func (Analyzer) Name() string {
	return "percentile"
}

// Analyze calculates the requested percentile without rounding its result.
func (Analyzer) Analyze(_ context.Context, input analysis.AnalysisInput) (analysis.AnalysisResult, error) {
	unit, period := input.Unit, input.Period
	maxPeriod := 3650
	if unit == analysis.UnitHours {
		maxPeriod *= 24
	}
	if (unit != analysis.UnitDays && unit != analysis.UnitHours) || period < 1 || period > maxPeriod ||
		math.IsNaN(input.Percentile) || math.IsInf(input.Percentile, 0) || input.Percentile < 0 || input.Percentile > 100 {
		return analysis.AnalysisResult{}, analysis.ErrInvalidArgument
	}
	if len(input.Candles) < period {
		return analysis.AnalysisResult{}, &analysis.InsufficientHistoryError{
			Required:  period,
			Available: len(input.Candles),
		}
	}

	candles := append([]market.Candle(nil), input.Candles...)
	sort.Slice(candles, func(i, j int) bool {
		return candles[i].OpenTime.After(candles[j].OpenTime)
	})
	candles = candles[:period]

	ranges := make([]float64, len(candles))
	for i, candle := range candles {
		if !(candle.Open > 0) {
			return analysis.AnalysisResult{}, analysis.ErrInvalidCandleData
		}
		ranges[i] = ((candle.High - candle.Low) / candle.Open) * 100
	}
	sort.Float64s(ranges)

	rank := (input.Percentile / 100) * float64(len(ranges)-1)
	lower := int(math.Floor(rank))
	upper := int(math.Ceil(rank))
	value := ranges[lower] + (rank-float64(lower))*(ranges[upper]-ranges[lower])

	return analysis.AnalysisResult{
		RangePercent: value,
		CandleCount:  len(candles),
		From:         candles[len(candles)-1].OpenTime,
		To:           candles[0].OpenTime,
	}, nil
}
