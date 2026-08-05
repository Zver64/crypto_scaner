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
	if input.PeriodDays < 1 || input.PeriodDays > 3650 ||
		math.IsNaN(input.Percentile) || input.Percentile < 0 || input.Percentile > 100 {
		return analysis.AnalysisResult{}, analysis.ErrInvalidArgument
	}
	if len(input.Candles) < input.PeriodDays {
		return analysis.AnalysisResult{}, &analysis.InsufficientHistoryError{
			Required:  input.PeriodDays,
			Available: len(input.Candles),
		}
	}

	candles := append([]market.Candle(nil), input.Candles...)
	sort.Slice(candles, func(i, j int) bool {
		return candles[i].OpenTime.After(candles[j].OpenTime)
	})
	candles = candles[:input.PeriodDays]

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
