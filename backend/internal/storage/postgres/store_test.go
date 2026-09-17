package postgres

import (
	"testing"

	"crypto-scanner/internal/analysis"
)

func TestSelectionParamsAcceptsScientificMarketCapValues(t *testing.T) {
	const minimumMarketCapUSD = 500_000_000.0
	params, err := selectionParams(analysis.Selection{Constraints: []analysis.SelectionConstraint{{
		Fact:     analysis.SelectionFactMarketCapUSD,
		Operator: analysis.SelectionAtLeast,
		Number:   minimumMarketCapUSD,
	}}})
	if err != nil {
		t.Fatal(err)
	}

	value, err := params.MinimumMarketCapUsd.Float64Value()
	if err != nil {
		t.Fatal(err)
	}
	if !value.Valid || value.Float64 != minimumMarketCapUSD {
		t.Fatalf("minimum market cap = %+v", value)
	}
}
