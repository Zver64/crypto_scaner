// Package percentile implements candle-range percentile filtering.
package percentile

import (
	"context"
	"math"
	"sort"

	"crypto-scanner/internal/analysis"
	"crypto-scanner/internal/market"
)

type Factory struct{}

func New() Factory           { return Factory{} }
func (Factory) Name() string { return "percentile" }

func (Factory) Build(parameters map[string]any) (analysis.Criterion, error) {
	if len(parameters) != 4 {
		return nil, analysis.ErrInvalidArgument
	}
	unitValue, unitOK := parameters["unit"].(string)
	period, periodOK := number(parameters["period"])
	percentile, percentileOK := number(parameters["percentile"])
	minimum, minimumOK := number(parameters["minimum_range_percent"])
	unit := analysis.Unit(unitValue)
	if !unitOK || !periodOK || period != math.Trunc(period) || !percentileOK || !minimumOK ||
		(unit != analysis.UnitDays && unit != analysis.UnitHours) || period < 1 || period > float64(maxPeriod(unit)) ||
		!finite(percentile) || percentile < 0 || percentile > 100 || !finite(minimum) || minimum < 0 {
		return nil, analysis.ErrInvalidArgument
	}
	return criterion{unit: unit, period: int(period), percentile: percentile, minimum: minimum}, nil
}

type criterion struct {
	unit                analysis.Unit
	period              int
	percentile, minimum float64
}

func (criterion) Name() string { return "percentile" }
func (c criterion) Requirements() []analysis.CandleRequirement {
	return []analysis.CandleRequirement{{Unit: c.unit, Count: c.period}}
}

func (c criterion) Evaluate(_ context.Context, data map[analysis.Unit][]market.Candle) (analysis.Evaluation, error) {
	candles := data[c.unit]
	if len(candles) == 0 {
		return analysis.Evaluation{}, &analysis.InsufficientHistoryError{Criterion: c.Name(), Required: c.period, Available: len(candles)}
	}
	candles = append([]market.Candle(nil), candles...)
	sort.Slice(candles, func(i, j int) bool { return candles[i].OpenTime.After(candles[j].OpenTime) })
	candles = candles[:min(c.period, len(candles))]
	ranges := make([]float64, len(candles))
	for i, candle := range candles {
		if !(candle.Open > 0) {
			return analysis.Evaluation{}, analysis.ErrInvalidCandleData
		}
		ranges[i] = ((candle.High - candle.Low) / candle.Open) * 100
	}
	sort.Float64s(ranges)
	rank := (c.percentile / 100) * float64(len(ranges)-1)
	lower, upper := int(math.Floor(rank)), int(math.Ceil(rank))
	value := ranges[lower] + (rank-float64(lower))*(ranges[upper]-ranges[lower])
	return analysis.Evaluation{Name: c.Name(), Matched: value >= c.minimum, Metrics: map[string]float64{"range_percent": value}, OrderingScore: &value, CandleCount: len(candles), From: candles[len(candles)-1].OpenTime.UTC(), To: candles[0].OpenTime.UTC()}, nil
}

func number(value any) (float64, bool) { number, ok := value.(float64); return number, ok }
func finite(value float64) bool        { return !math.IsNaN(value) && !math.IsInf(value, 0) }
func maxPeriod(unit analysis.Unit) int {
	if unit == analysis.UnitHours {
		return 3650 * 24
	}
	return 3650
}
