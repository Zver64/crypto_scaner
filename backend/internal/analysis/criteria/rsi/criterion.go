// Package rsi implements a maximum RSI criterion over closed indicator values.
package rsi

import (
	"context"

	"crypto-scanner/internal/analysis"
	"crypto-scanner/internal/closedindicator"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/platform/numeric"
)

const outputName = "rsi"

// Factory builds criteria that read RSI values from the closed indicator
// tracker; selection must be one of its tracked targets.
type Factory struct{ selection indicator.Selection }

func New(selection indicator.Selection) Factory { return Factory{selection: selection} }
func (Factory) Name() string                    { return "rsi" }
func (factory Factory) Build(parameters map[string]any) (analysis.Criterion, error) {
	interval, intervalOK := parameters["interval"].(string)
	maximum, maximumOK := parameters["max_rsi"].(float64)
	candleInterval := market.CandleInterval(interval)
	if len(parameters) != 2 || !intervalOK || (candleInterval != market.IntervalDay && candleInterval != market.IntervalWeek) ||
		!maximumOK || !numeric.Finite(maximum) || maximum < 0 || maximum > 100 {
		return nil, analysis.ErrInvalidArgument
	}
	return criterion{target: closedindicator.Target{Interval: candleInterval, Selection: factory.selection}, maximum: maximum}, nil
}

type criterion struct {
	target  closedindicator.Target
	maximum float64
}

func (criterion) Name() string                               { return "rsi" }
func (criterion) Requirements() []analysis.CandleRequirement { return nil }
func (criterion) UsesClosedIndicators() bool                 { return true }

// Evaluate never matches an unavailable RSI: an unknown value cannot be shown
// to stay below the maximum.
func (c criterion) Evaluate(_ context.Context, input analysis.Input) (analysis.Evaluation, error) {
	for _, value := range input.ClosedIndicators {
		if !value.Target.Equal(c.target) {
			continue
		}
		for _, output := range value.Outputs {
			if output.Name == outputName {
				openTime := value.OpenTime.UTC()
				return analysis.Evaluation{Matched: output.Value <= c.maximum, Metrics: map[string]float64{"rsi": output.Value}, CandleCount: 1, From: openTime, To: openTime}, nil
			}
		}
	}
	return analysis.Evaluation{Matched: false}, nil
}
