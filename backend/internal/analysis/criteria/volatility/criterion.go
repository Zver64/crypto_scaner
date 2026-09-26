// Package volatility implements the volatility criterion using candle-range percentiles.
package volatility

import (
	"context"
	"math"

	"crypto-scanner/internal/analysis"
	"crypto-scanner/internal/platform/numeric"
)

type Factory struct{}

func New() Factory           { return Factory{} }
func (Factory) Name() string { return "volatility" }

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
		!numeric.Finite(percentile) || percentile < 0 || percentile > 100 || !numeric.Finite(minimum) || minimum < 0 {
		return nil, analysis.ErrInvalidArgument
	}
	return criterion{unit: unit, period: int(period), percentile: percentile, minimum: minimum}, nil
}

type criterion struct {
	unit                analysis.Unit
	period              int
	percentile, minimum float64
}

func (criterion) Name() string { return "volatility" }
func (c criterion) Requirements() []analysis.CandleRequirement {
	return []analysis.CandleRequirement{{Unit: c.unit, Count: c.period}}
}

func (c criterion) Evaluate(_ context.Context, input analysis.Input) (analysis.Evaluation, error) {
	candles := input.Candles[c.unit]
	if len(candles) < c.period {
		return analysis.Evaluation{}, &analysis.InsufficientHistoryError{Criterion: c.Name(), Required: c.period, Available: len(candles)}
	}
	candles = candles[len(candles)-c.period:]
	ranges := make([]float64, len(candles))
	for i, candle := range candles {
		if !(candle.Open > 0) {
			return analysis.Evaluation{}, analysis.ErrInvalidCandleData
		}
		ranges[i] = ((candle.High - candle.Low) / candle.Open) * 100
	}
	value := exceedancePercentile(ranges, c.percentile)
	return analysis.Evaluation{Name: c.Name(), Matched: value >= c.minimum, Metrics: map[string]float64{"range_percent": value}, CandleCount: len(candles), From: candles[0].OpenTime.UTC(), To: candles[len(candles)-1].OpenTime.UTC()}, nil
}

func number(value any) (float64, bool) { number, ok := value.(float64); return number, ok }
func maxPeriod(unit analysis.Unit) int {
	if unit == analysis.UnitHours {
		return 3650 * 24
	}
	return 3650
}
