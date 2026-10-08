package strategy

import (
	"errors"
	"maps"
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

// Position variables exist only in exit rules and only at the latest
// candle; an exit rule may read nothing else.
func TestCompileReadsPositionVariablesOnlyInExitRules(t *testing.T) {
	exit := append(Variables(nil), PositionVariables()...)
	if _, err := Compile("pnl > 5 || bars_held >= 24", exit); err != nil {
		t.Fatalf("Compile(exit) error = %v", err)
	}
	for _, test := range []struct {
		source    string
		variables []Variable
		problem   string
	}{
		{source: "h_close > entry_price", variables: Variables(nil), problem: "entry_price is available only in the exit rule"},
		{source: "prev(pnl) > 0", variables: exit, problem: "pnl cannot be read through prev, percentile, crossings, or of"},
		{source: `of("BTCUSDT", pnl) > 0 && h_close > 1`, variables: exit, problem: "pnl cannot be read through prev, percentile, crossings, or of"},
	} {
		_, err := Compile(test.source, test.variables)
		var invalid *InvalidExpressionError
		if !errors.As(err, &invalid) || !slices.Equal(invalid.Problems, []string{test.problem}) {
			t.Fatalf("Compile(%q) error = %v, want %q", test.source, err, test.problem)
		}
	}
}

// A price is arithmetic over values that reads the evaluated coin, and its
// values are reported as the source writes them, outside percentile windows.
func TestCompilePrice(t *testing.T) {
	variables := Variables(nil)
	for source, problem := range map[string]string{
		"h_close > 1":                  "the price must be calculated from indicators, candle fields, and numbers",
		"5":                            "the price reads an indicator or a candle field",
		`of("BTCUSDT", h_close) * 0.9`: "the expression must also read the evaluated coin, not only coins read through of",
		"pnl * 2":                      "pnl is available only in the exit rule",
	} {
		_, err := CompilePrice(source, variables)
		var invalid *InvalidExpressionError
		if !errors.As(err, &invalid) || !slices.Equal(invalid.Problems, []string{problem}) {
			t.Fatalf("CompilePrice(%q) error = %v, want %q", source, err, problem)
		}
	}
	compiled, err := CompilePrice(`min(h_low, prev(h_low, 2), prev(h_low)) - percentile(h_high - h_low, 3, 50) + of("ETHUSDT", h_close) * 0`, variables)
	if err != nil {
		t.Fatal(err)
	}
	// Lows fall by one per candle back and highs stay 2 above them.
	price, known := compiled.Price(func(read Read) (float64, bool) {
		value := 10 - float64(read.Shift)
		if read.Variable.Name == "h_high" {
			value += 2
		}
		return value, true
	})
	if !known || price != 8-2 {
		t.Fatalf("Price() = %v (%v), want 6", price, known)
	}
	got := compiled.Values(func(read Read) (float64, bool) { return float64(read.Shift), true })
	want := map[string]float64{"h_low": 0, "prev(h_low)": 1, "prev(h_low, 2)": 2, `of("ETHUSDT", h_close)`: 0}
	if !maps.Equal(got, want) {
		t.Fatalf("Values() = %v, want %v", got, want)
	}
}
