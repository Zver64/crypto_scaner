package strategy

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func TestMarketCapRange(t *testing.T) {
	usd := func(value float64) *float64 { return &value }
	small := MarketCapRange{MinUSD: usd(10e6), MaxUSD: usd(1.5e9)}
	for _, test := range []struct {
		bounds MarketCapRange
		cap    *float64
		want   bool
	}{
		{MarketCapRange{}, nil, true},
		{MarketCapRange{}, usd(1), true},
		{small, nil, false},
		{small, usd(10e6), true},
		{small, usd(1.5e9), true},
		{small, usd(9e6), false},
		{small, usd(2e9), false},
		{MarketCapRange{MinUSD: usd(1e9)}, usd(5e12), true},
		{MarketCapRange{MaxUSD: usd(1e9)}, usd(1), true},
	} {
		if got := test.bounds.Contains(test.cap); got != test.want {
			t.Errorf("%v.Contains(%v) = %v, want %v", test.bounds, test.cap, got, test.want)
		}
	}

	for bounds, want := range map[MarketCapRange]string{
		{}:                       "",
		small:                    "$10M – $1.5B",
		{MinUSD: usd(1e9)}:       "≥ $1B",
		{MaxUSD: usd(123456789)}: "≤ $123.46M",
		{MaxUSD: usd(500)}:       "≤ $500",
		{MaxUSD: usd(999995000)}: "≤ $1B",
	} {
		if got := bounds.String(); got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	}

	if err := small.validate(); err != nil {
		t.Errorf("validate() = %v", err)
	}
	for _, bounds := range []MarketCapRange{
		{MinUSD: usd(0)}, {MaxUSD: usd(-1)}, {MinUSD: usd(math.NaN())}, {MaxUSD: usd(math.Inf(1))},
		{MinUSD: usd(2e9), MaxUSD: usd(1e9)},
	} {
		if err := bounds.validate(); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("validate(%v) = %v, want ErrInvalidArgument", bounds, err)
		}
	}
}

func TestSummaryTextNamesTheMarketCapRange(t *testing.T) {
	minimum := 50e6
	entry := Entry{Strategy: Strategy{Name: "Small caps", Expression: "h_close > 1", MarketCap: MarketCapRange{MinUSD: &minimum}}, Compiled: &Expression{}}
	if got := summaryText(entry, nil, snapshot{}); !strings.Contains(got, "\nMarket cap: ≥ $50M\n") {
		t.Fatalf("summaryText() = %q", got)
	}
}
