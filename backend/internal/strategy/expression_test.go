package strategy

import (
	"errors"
	"slices"
	"testing"
)

func TestCompileListsEveryProblem(t *testing.T) {
	variables := Variables(nil)
	tests := []struct {
		name     string
		source   string
		problems []string
	}{
		{
			name:   "unknown names in several comparisons",
			source: "h_rsi > 70 && (h_close > 1 || d_ema > h_rsi) && h_close < 2",
			problems: []string{
				"h_rsi is not a configured indicator or candle field",
				"d_ema is not a configured indicator or candle field",
			},
		},
		{
			name:     "unknown name in a crossing",
			source:   "crosses_above(h_foo, 1) && h_foo < 5",
			problems: []string{"h_foo is not a configured indicator or candle field"},
		},
		{
			name:     "unknown function",
			source:   "foo(h_close) > 1",
			problems: []string{"1:4: undeclared reference to 'foo' (in container '')"},
		},
		{
			name:     "comparison of numbers only",
			source:   "1 > 0 && h_close > 1",
			problems: []string{"each comparison reads an indicator or a candle field"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Compile(test.source, variables)
			var invalid *InvalidExpressionError
			if !errors.As(err, &invalid) || !errors.Is(err, ErrInvalidArgument) {
				t.Fatalf("Compile() error = %v, want an invalid expression", err)
			}
			if !slices.Equal(invalid.Problems, test.problems) {
				t.Fatalf("problems = %q, want %q", invalid.Problems, test.problems)
			}
		})
	}
}

func TestCompileAcceptsValidExpression(t *testing.T) {
	source := `crosses_above(h_close, prev(h_close, 2)) && of("BTCUSDT", h_close) > 1`
	if _, err := Compile(source, Variables(nil)); err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
}
