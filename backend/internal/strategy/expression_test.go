package strategy

import (
	"errors"
	"maps"
	"slices"
	"strings"
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
	price, known, _ := compiled.Price(func(read Read) (float64, bool) {
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

// A calculated prev shift is read at the latest candle and picks the operand
// that many candles back, reading only that candle. A shift outside 1 to 500
// is a rule error where it decides the result.
func TestCompileCalculatedPrevShift(t *testing.T) {
	exit := append(Variables(nil), PositionVariables()...)
	for _, test := range []struct {
		name, source string
		barsHeld     float64
		missing      int
		result       bool
		known        bool
		shiftError   bool
	}{
		{name: "picks the shift", source: "prev(h_close, bars_held - 1) > h_open", barsHeld: 4, missing: 7, result: true, known: true},
		{name: "picked value missing", source: "prev(h_close, bars_held - 1) > h_open", barsHeld: 4, missing: 3},
		{name: "zero shift", source: "prev(h_close, bars_held - 1) > h_open", barsHeld: 1, shiftError: true},
		{name: "beyond 500", source: "prev(h_close, bars_held - 1) > h_open", barsHeld: 502, shiftError: true},
		{name: "decided without it", source: "h_open < 0 && prev(h_close, bars_held - 1) > h_open", barsHeld: 1, known: true},
		{name: "unknown before it", source: "h_low > 0 && prev(h_close, bars_held - 1) > h_open", barsHeld: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			compiled, err := Compile(test.source, exit)
			if err != nil {
				t.Fatal(err)
			}
			// Closes fall by one per candle back; the open lies between the
			// closes three and four candles back.
			var shifts []int
			value := func(read Read) (float64, bool) {
				switch read.Variable.Name {
				case "bars_held":
					return test.barsHeld, true
				case "h_open":
					return 96.5, true
				case "h_low":
					return 0, false
				}
				shifts = append(shifts, read.Shift)
				return 100 - float64(read.Shift), read.Shift != test.missing
			}
			result, known, err := compiled.Evaluate(value)
			var shiftError *ShiftError
			if result != test.result || known != test.known || errors.As(err, &shiftError) != test.shiftError {
				t.Fatalf("Evaluate() = %v (%v), %v", result, known, err)
			}
			if test.name == "picks the shift" {
				if !slices.Equal(shifts, []int{3}) {
					t.Fatalf("read h_close at shifts %v, want only the picked one", shifts)
				}
				got := compiled.Values(value)
				if want := map[string]float64{"bars_held": 4, "h_open": 96.5, "prev(h_close, 3)": 97}; !maps.Equal(got, want) {
					t.Fatalf("Values() = %v, want %v", got, want)
				}
				if !slices.ContainsFunc(compiled.Reads(), func(read Read) bool { return read.Variable.Name == "h_close" && read.Shift == maxShift }) {
					t.Fatalf("Reads() = %v, want h_close at shift %d", compiled.Reads(), maxShift)
				}
			}
		})
	}

	if _, err := Compile("prev(h_close, 500) > 1", Variables(nil)); err != nil {
		t.Fatalf("Compile(prev 500) error = %v", err)
	}
	for source, problem := range map[string]string{
		"prev(prev(h_close, bars_held), bars_held) > 1":   "prev with a calculated shift cannot contain prev with a calculated shift",
		"prev(h_close, prev(bars_held)) > 1":              shiftProblem,
		"prev(h_close, bars_held * 2) > 1":                shiftProblem,
		"prev(pnl, bars_held) > 1":                        "pnl cannot be read through prev, percentile, crossings, or of",
		"prev(h_close, h_open) > 1":                       "h_open does not count candles, so it cannot be a prev shift",
		"prev(percentile(h_close, 5, 50), bars_held) > 1": "prev with a calculated shift cannot contain percentile",
		"percentile(prev(h_close, bars_held), 5, 50) > 1": "percentile cannot contain prev with a calculated shift",
		"prev(h_close, 2.5) > 1":                          "the prev shift must be a whole number from 1 to 500",
		"prev(h_close, 501) > 1":                          "the prev shift must be a whole number from 1 to 500",
	} {
		_, err := Compile(source, exit)
		var invalid *InvalidExpressionError
		if !errors.As(err, &invalid) || len(invalid.Problems) != 1 || !strings.HasSuffix(invalid.Problems[0], problem) {
			t.Errorf("Compile(%q) error = %v, want %q", source, err, problem)
		}
	}
}
