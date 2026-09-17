package market_cap

import (
	"context"
	"math"

	"crypto-scanner/internal/analysis"
	"crypto-scanner/internal/market"
)

// Factory builds a criterion backed by the market-cap fact selected from
// PostgreSQL. Refreshing that fact belongs to the marketcap synchronizer.
type Factory struct{}

func New() Factory           { return Factory{} }
func (Factory) Name() string { return "market_cap" }
func (Factory) Build(parameters map[string]any) (analysis.Criterion, error) {
	value, ok := parameters["min_market_cap_usd"].(float64)
	if len(parameters) != 1 || !ok || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return nil, analysis.ErrInvalidArgument
	}
	return &criterion{minimum: value}, nil
}

type criterion struct{ minimum float64 }

func (criterion) Name() string                               { return "market_cap" }
func (criterion) Requirements() []analysis.CandleRequirement { return nil }
func (*criterion) Prepare(context.Context, []market.Instrument) ([]analysis.Warning, error) {
	return nil, nil
}

// MinimumMarketCapUSD identifies this criterion as a SQL selection constraint.
func (c *criterion) MinimumMarketCapUSD() float64 { return c.minimum }
func (c *criterion) Evaluate(_ context.Context, input analysis.Input) (analysis.Evaluation, error) {
	if input.Instrument.MarketCapUSD == nil {
		return analysis.Evaluation{}, &analysis.UnresolvedError{Code: "market_cap_missing", Message: "Market capitalization could not be resolved"}
	}
	cap := *input.Instrument.MarketCapUSD
	return analysis.Evaluation{Matched: cap >= c.minimum, Metrics: map[string]float64{"market_cap_usd": cap}}, nil
}
