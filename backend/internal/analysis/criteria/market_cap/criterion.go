package market_cap

import (
	"context"
	"crypto-scanner/internal/analysis"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/marketcap"
	"errors"
	"math"
)

type Factory struct{ resolver *marketcap.Resolver }

func New(resolver *marketcap.Resolver) Factory { return Factory{resolver} }
func (Factory) Name() string                   { return "market_cap" }
func (f Factory) Build(parameters map[string]any) (analysis.Criterion, error) {
	value, ok := parameters["min_market_cap_usd"].(float64)
	if len(parameters) != 1 || !ok || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || f.resolver == nil {
		return nil, analysis.ErrInvalidArgument
	}
	return &criterion{resolver: f.resolver, minimum: value}, nil
}

type criterion struct {
	resolver *marketcap.Resolver
	minimum  float64
	facts    map[string]marketcap.Fact
}

func (criterion) Name() string                               { return "market_cap" }
func (criterion) Requirements() []analysis.CandleRequirement { return nil }
func (c *criterion) Prepare(ctx context.Context, candidates []market.Instrument) ([]analysis.Warning, error) {
	batch, err := c.resolver.ResolveBatch(ctx, candidates)
	if err != nil {
		if errors.Is(err, marketcap.ErrBootstrapIncomplete) {
			return nil, analysis.ErrMarketCapUnavailable
		}
		return nil, err
	}
	c.facts = batch.Facts
	if batch.ProviderWarning {
		return []analysis.Warning{{Code: "market_cap_provider_unavailable", Message: "Market capitalization provider is temporarily unavailable; cached values were used where possible"}}, nil
	}
	return nil, nil
}
func (c *criterion) Evaluate(_ context.Context, input analysis.Input) (analysis.Evaluation, error) {
	fact, ok := c.facts[input.Instrument.BaseAsset]
	if !ok || fact.USD == nil {
		reason := "market_cap_missing"
		if ok && fact.Reason != "" {
			reason = fact.Reason
		}
		return analysis.Evaluation{}, &analysis.UnresolvedError{Code: reason, Message: "Market capitalization could not be resolved"}
	}
	cap := *fact.USD
	return analysis.Evaluation{Matched: cap >= c.minimum, Metrics: map[string]float64{"market_cap_usd": cap}}, nil
}
