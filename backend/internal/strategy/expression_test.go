package strategy

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"math"
	"slices"
	"strings"
	"testing"

	"crypto-scanner/internal/closedindicator"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/scannerindicator"

	"github.com/google/uuid"
)

func validationTestService(t *testing.T) *Service {
	t.Helper()
	registry := testRegistry(t)
	indicators, err := scannerindicator.New(&struct{ scannerindicator.Store }{}, registry, []string{"yellow.5"}, slog.New(slog.DiscardHandler), func() {})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(newBacktestStore(market.IntervalHour, nil), indicators, registry, noTracker{}, slog.New(slog.DiscardHandler), 0, func([]uuid.UUID) {})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func TestCompileListsEveryProblem(t *testing.T) {
	registry := testRegistry(t)
	tests := []struct {
		name     string
		source   string
		problems []string
	}{
		{
			name:   "names of the old syntax",
			source: `h_rsi > 70 && (close("1h") > 1 || d_ema > h_rsi) && close("1h") < 2`,
			problems: []string{
				`unknown name h_rsi; read indicators and candle fields through calls, such as close("1h")`,
				`unknown name d_ema; read indicators and candle fields through calls, such as close("1h")`,
			},
		},
		{
			name:     "unknown name in a crossing",
			source:   `CrossesAbove(foo, 1) && foo < 5`,
			problems: []string{`unknown name foo; read indicators and candle fields through calls, such as close("1h")`},
		},
		{
			name:     "functions of the old syntax and unknown ones",
			source:   `prev(close("1h")) > 1 && crosses_above(close("1h"), 1) && foo(close("1h")) > 1`,
			problems: []string{"unknown function prev; did you mean Prev?", "unknown function crosses_above; did you mean CrossesAbove?", "unknown function or indicator foo"},
		},
		{
			name:     "an indicator with several outputs called without one",
			source:   `bbands("1h", 20) > 1`,
			problems: []string{"1:7: bbands has several outputs; call one of bbands_upperband, bbands_middleband, bbands_lowerband"},
		},
		{
			name:   "unknown outputs and fields",
			source: `bbands_upper("1h") > 1 && rsi_rsi("1h") > 1 && bbands_upperband("1h").upperband > 1 && pnl.x > 1`,
			problems: []string{
				"bbands has no output upper; call one of bbands_upperband, bbands_middleband, bbands_lowerband",
				"unknown function or indicator rsi_rsi",
				`unknown field upperband; an indicator output is part of the function name, such as bbands_upperband("1h")`,
				`unknown field x; an indicator output is part of the function name, such as bbands_upperband("1h")`,
			},
		},
		{
			// Each window stays under the limit; together they expand past it.
			name:     "expanded past the node limit",
			source:   strings.Repeat(`Percentile(close("1h") + open("1h"), 500, 50) > 1 && `, 15) + `close("1h") > 1`,
			problems: []string{"the expression is too large"},
		},
		{
			name:     "comparison of numbers only",
			source:   `1 > 0 && close("1h") > 1`,
			problems: []string{"each comparison reads an indicator or a candle field"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Compile(test.source, registry, EntryRule)
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

// A call that does not resolve fails parsing at the call.
func TestCompileRejectsInvalidCalls(t *testing.T) {
	registry := testRegistry(t)
	for source, problem := range map[string]string{
		`rsi() > 1`:                   `1:4: rsi takes the period first, such as rsi("1h")`,
		`close("1h") > rsi("2h")`:     `1:18: the period of rsi is "1h", "1d", "1w", or "1M"`,
		`close("1h", 2) > 1`:          "1:6: close takes only the period",
		`rsi("1h", 14, close, 2) > 1`: "1:4: rsi takes the period and at most 2 parameters",
		`rsi("1h", 1) > 1`:            "1:4: rsi: invalid indicator request: period must be an integer between 2 and 2000",
		`rsi("1h", close("1h")) > 1`:  "1:4: the period of rsi is a number",
		`sma("1h", 20, closing) > 1`:  "1:4: the source of sma is a candle field (open, high, low, close, volume, quote_volume, trades) or an indicator call of the same period",
		`sma("1h", 20, 4) > 1`:        "1:4: the source of sma is a candle field (open, high, low, close, volume, quote_volume, trades) or an indicator call of the same period",
		// A source call is one indicator call of the period of the outer
		// call, with a value on every candle and candle fields beneath.
		`sma("1h", 9, rsi("1d")) > 1`:               "1:4: the source of sma is a candle field (open, high, low, close, volume, quote_volume, trades) or an indicator call of the same period",
		`sma("1h", 9, close("1h")) > 1`:             "1:4: the source of sma is a candle field (open, high, low, close, volume, quote_volume, trades) or an indicator call of the same period",
		`sma("1h", 9, Prev(rsi("1h"))) > 1`:         "1:4: the source of sma is a candle field (open, high, low, close, volume, quote_volume, trades) or an indicator call of the same period",
		`sma("1h", 9, rsi("1h") + 1) > 1`:           "1:4: the source of sma is a candle field (open, high, low, close, volume, quote_volume, trades) or an indicator call of the same period",
		`sma("1h", 9, sma("1h", 9, rsi("1h"))) > 1`: "1:4: sma: invalid indicator request: source reads sma, whose own source must be a candle field",
		`sma("1h", 9, pivot_low("1h")) > 1`:         `1:4: sma: invalid indicator request: source reads "low" of pivot, which is not an output with a value on every candle`,
	} {
		_, err := Compile(source, registry, EntryRule)
		var invalid *InvalidExpressionError
		if !errors.As(err, &invalid) || !slices.Equal(invalid.Problems, []string{problem}) {
			t.Errorf("Compile(%q) error = %v, want %q", source, err, problem)
		}
	}
}

// Calls read the same values however they spell their parameters, and
// labels and Canonical spell them canonically: defaults dropped from the
// end, numbers without trailing zeros, and choices by name.
func TestCallsAreSpelledCanonically(t *testing.T) {
	registry := testRegistry(t)
	source := `rsi( "1h", 14 ) > 30 && sma("1d", 30, volume) > 0 && bbands_upperband("1h", 20, 2.50) > close("1h") && rsi("1h") < 70`
	compiled, err := Compile(source, registry, EntryRule)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, read := range compiled.Reads() {
		names = append(names, read.Variable.Name)
	}
	if want := []string{`bbands_upperband("1h", 20, 2.5)`, `close("1h")`, `rsi("1h")`, `rsi("1h")`, `sma("1d", 30, volume)`}; !slices.Equal(names, want) {
		t.Fatalf("reads = %q, want %q", names, want)
	}
	canonical, targets := Canonical(source, registry)
	if want := `rsi("1h") > 30 && sma("1d", 30, volume) > 0 && bbands_upperband("1h", 20, 2.5) > close("1h") && rsi("1h") < 70`; canonical != want {
		t.Fatalf("Canonical() = %s, want %s", canonical, want)
	}
	var types []string
	for _, target := range targets {
		types = append(types, string(target.Interval)+" "+string(target.Selection.Type))
	}
	if want := []string{"1h rsi", "1d sma", "1h bbands", "1h rsi"}; !slices.Equal(types, want) {
		t.Fatalf("Canonical() targets = %q, want %q", types, want)
	}
}

// A call written as the source of another is part of that call: the rule
// reads one value, the outer indicator calculated over the inner one, spelled
// with the inner call in its own canonical form.
func TestSourceCallsAreSpelledCanonically(t *testing.T) {
	registry := testRegistry(t)
	source := `sma( "1h", 9.0, rsi( "1h", 14 ) ) > 50 && Prev(pivot_low("1d", 5, 5, rsi("1d", 21)), 2) > ema("1h", 20, macd_macdhist("1h", 12)) && correl("1h", 30, rsi("1h"), volume) > 0`
	compiled, err := Compile(source, registry, EntryRule)
	if err != nil {
		t.Fatal(err)
	}
	var labels []string
	for _, read := range compiled.Reads() {
		labels = append(labels, read.label())
	}
	if want := []string{`correl("1h", 30, rsi("1h"), volume)`, `ema("1h", 20, macd_macdhist("1h"))`, `Prev(pivot_low("1d", 5, 5, rsi("1d", 21)), 2)`, `sma("1h", 9, rsi("1h"))`}; !slices.Equal(labels, want) {
		t.Fatalf("reads = %q, want %q", labels, want)
	}
	canonical, targets := Canonical(source, registry)
	if want := `sma("1h", 9, rsi("1h")) > 50 && Prev(pivot_low("1d", 5, 5, rsi("1d", 21)), 2) > ema("1h", 20, macd_macdhist("1h")) && correl("1h", 30, rsi("1h"), volume) > 0`; canonical != want {
		t.Fatalf("Canonical() = %s, want %s", canonical, want)
	}
	nested, err := registry.Normalize(indicator.Selection{Type: "sma", Parameters: indicator.Parameters{
		"period": 9, "source": indicator.Source{Selection: indicator.Selection{Type: "rsi"}, Output: "rsi"}.Value(),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 4 || !targets[0].Equal(closedindicator.Target{Interval: market.IntervalHour, Selection: nested}) {
		t.Fatalf("Canonical() targets = %+v, want 4 starting with the SMA of the RSI", targets)
	}
}

func TestCompileAcceptsValidExpression(t *testing.T) {
	source := `CrossesAbove(close("1h"), Prev(close("1h"), 2)) && Of("BTCUSDT", bbands_lowerband("1d")) > 1 && Abs(Max(rsi("1w"), 1, 2)) > Mod(trades("1M"), 2)`
	if _, err := Compile(source, testRegistry(t), EntryRule); err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
}

// Position variables exist only in exit rules and only at the latest
// candle; an exit rule may read nothing else.
func TestCompileReadsPositionVariablesOnlyInExitRules(t *testing.T) {
	registry := testRegistry(t)
	if _, err := Compile("pnl > 5 || bars_held >= 24", registry, ExitRule); err != nil {
		t.Fatalf("Compile(exit) error = %v", err)
	}
	for _, test := range []struct {
		source  string
		kind    RuleKind
		problem string
	}{
		{source: `close("1h") > entry_price`, kind: EntryRule, problem: "entry_price is available only in the exit rule"},
		{source: "Prev(pnl) > 0", kind: ExitRule, problem: "pnl cannot be read through Prev, Percentile, ValueWhen, BarsSince, crossings, or Of"},
		{source: `Of("BTCUSDT", pnl) > 0 && close("1h") > 1`, kind: ExitRule, problem: "pnl cannot be read through Prev, Percentile, ValueWhen, BarsSince, crossings, or Of"},
	} {
		_, err := Compile(test.source, registry, test.kind)
		var invalid *InvalidExpressionError
		if !errors.As(err, &invalid) || !slices.Equal(invalid.Problems, []string{test.problem}) {
			t.Fatalf("Compile(%q) error = %v, want %q", test.source, err, test.problem)
		}
	}
}

// A price is arithmetic over values that reads the evaluated coin, and its
// values are reported as the source writes them, outside percentile windows.
func TestCompilePrice(t *testing.T) {
	registry := testRegistry(t)
	for source, problem := range map[string]string{
		`close("1h") > 1`:                  "the price must be calculated from indicators, candle fields, and numbers",
		"5":                                "the price reads an indicator or a candle field",
		`Of("BTCUSDT", close("1h")) * 0.9`: "the expression must also read the evaluated coin, not only coins read through Of",
		"pnl * 2":                          "pnl is available only in the exit rule",
	} {
		_, err := Compile(source, registry, PriceRule)
		var invalid *InvalidExpressionError
		if !errors.As(err, &invalid) || !slices.Equal(invalid.Problems, []string{problem}) {
			t.Fatalf("CompilePrice(%q) error = %v, want %q", source, err, problem)
		}
	}
	compiled, err := Compile(`Min(low("1h"), Prev(low("1h"), 2), Prev(low("1h"))) - Percentile(high("1h") - low("1h"), 3, 50) + Of("ETHUSDT", close("1h")) * 0`, registry, PriceRule)
	if err != nil {
		t.Fatal(err)
	}
	// Lows fall by one per candle back and highs stay 2 above them.
	price, known, _ := compiled.Price(func(read Read) (float64, bool) {
		value := 10 - float64(read.Shift)
		if read.Variable.Name == `high("1h")` {
			value += 2
		}
		return value, true
	})
	if !known || price != 8-2 {
		t.Fatalf("Price() = %v (%v), want 6", price, known)
	}
	got := compiled.Values(func(read Read) (float64, bool) { return float64(read.Shift), true })
	want := map[string]float64{`low("1h")`: 0, `Prev(low("1h"))`: 1, `Prev(low("1h"), 2)`: 2, `Of("ETHUSDT", close("1h"))`: 0}
	if !maps.Equal(got, want) {
		t.Fatalf("Values() = %v, want %v", got, want)
	}
}

// A calculated Prev shift is read at the latest candle and picks the operand
// that many candles back, reading only that candle. A shift outside 0 to 500
// is a rule error where it decides the result.
func TestCompileCalculatedPrevShift(t *testing.T) {
	registry := testRegistry(t)
	for _, test := range []struct {
		name, source string
		barsHeld     float64
		missing      int
		result       bool
		known        bool
		shiftError   bool
	}{
		{name: "picks the shift", source: `Prev(close("1h"), bars_held - 1) > open("1h")`, barsHeld: 4, missing: 7, result: true, known: true},
		{name: "picked value missing", source: `Prev(close("1h"), bars_held - 1) > open("1h")`, barsHeld: 4, missing: 3},
		{name: "zero shift", source: `Prev(close("1h"), bars_held - 1) > open("1h")`, barsHeld: 1, missing: 7, result: true, known: true},
		{name: "below zero", source: `Prev(close("1h"), bars_held - 1) > open("1h")`, barsHeld: 0, shiftError: true},
		{name: "beyond 500", source: `Prev(close("1h"), bars_held - 1) > open("1h")`, barsHeld: 502, shiftError: true},
		{name: "decided without it", source: `open("1h") < 0 && Prev(close("1h"), bars_held - 1) > open("1h")`, barsHeld: 0, known: true},
		{name: "unknown before it", source: `low("1h") > 0 && Prev(close("1h"), bars_held - 1) > open("1h")`, barsHeld: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			compiled, err := Compile(test.source, registry, ExitRule)
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
				case `open("1h")`:
					return 96.5, true
				case `low("1h")`:
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
					t.Fatalf(`read close("1h") at shifts %v, want only the picked one`, shifts)
				}
				got := compiled.Values(value)
				if want := map[string]float64{"bars_held": 4, `open("1h")`: 96.5, `Prev(close("1h"), 3)`: 97}; !maps.Equal(got, want) {
					t.Fatalf("Values() = %v, want %v", got, want)
				}
				if !slices.ContainsFunc(compiled.Reads(), func(read Read) bool { return read.Variable.Name == `close("1h")` && read.Shift == maxShift }) {
					t.Fatalf(`Reads() = %v, want close("1h") at shift %d`, compiled.Reads(), maxShift)
				}
			}
		})
	}

	if _, err := Compile(`Prev(close("1h"), 500) > 1`, registry, EntryRule); err != nil {
		t.Fatalf("Compile(prev 500) error = %v", err)
	}
	for source, problem := range map[string]string{
		`Prev(Prev(close("1h"), bars_held), bars_held) > 1`:   "Prev with a calculated shift cannot contain Prev with a calculated shift",
		`Prev(close("1h"), Prev(bars_held)) > 1`:              shiftProblem,
		`Prev(close("1h"), bars_held * 2) > 1`:                shiftProblem,
		"Prev(pnl, bars_held) > 1":                            "pnl cannot be read through Prev, Percentile, ValueWhen, BarsSince, crossings, or Of",
		`Prev(close("1h"), open("1h")) > 1`:                   `open("1h") does not count candles, so it cannot be a Prev shift`,
		`Prev(Percentile(close("1h"), 5, 50), bars_held) > 1`: "Prev with a calculated shift cannot contain Percentile",
		`Percentile(Prev(close("1h"), bars_held), 5, 50) > 1`: "Percentile cannot contain Prev with a calculated shift",
		`Prev(close("1h"), 2.5) > 1`:                          "the Prev shift must be a whole number from 1 to 500",
		`Prev(close("1h"), 501) > 1`:                          "the Prev shift must be a whole number from 1 to 500",
	} {
		_, err := Compile(source, registry, ExitRule)
		if problem == "" {
			if err != nil {
				t.Errorf("Compile(%q) error = %v", source, err)
			}
			continue
		}
		var invalid *InvalidExpressionError
		if !errors.As(err, &invalid) || len(invalid.Problems) != 1 || !strings.HasSuffix(invalid.Problems[0], problem) {
			t.Errorf("Compile(%q) error = %v, want %q", source, err, problem)
		}
	}
}

// An empty value is known: whatever compares it is false, and arithmetic
// with it is empty too.
func TestEmptyValuesCompareAsFalse(t *testing.T) {
	registry := testRegistry(t)
	value := func(read Read) (float64, bool) {
		if read.Variable.Name == `pivot_high("1h")` {
			return math.NaN(), true
		}
		return 100, true
	}
	for source, want := range map[string]bool{
		`pivot_high("1h") > 0`:                              false,
		`pivot_high("1h") <= 0`:                             false,
		`!(pivot_high("1h") > 0)`:                           true,
		`close("1h") - pivot_high("1h") / 2 < 1000`:         false,
		`Max(pivot_high("1h"), close("1h")) >= close("1h")`: false,
		`pivot_high("1h") > 0 || close("1h") > 0`:           true,
	} {
		compiled, err := Compile(source, registry, EntryRule)
		if err != nil {
			t.Fatal(err)
		}
		if result, known, err := compiled.Evaluate(value); result != want || !known || err != nil {
			t.Errorf("Evaluate(%s) = %v (%v), %v, want %v", source, result, known, err, want)
		}
		if _, shown := compiled.Values(value)[`pivot_high("1h")`]; shown {
			t.Errorf("Values(%s) holds the empty value", source)
		}
	}
	price, err := Compile(`pivot_high("1h") + 1`, registry, PriceRule)
	if err != nil {
		t.Fatal(err)
	}
	if _, known, err := price.Price(value); known || err != nil {
		t.Fatalf("Price() of an empty value is known (%v), %v", known, err)
	}
}

// ValueWhen and BarsSince scan the latest 200 closed candles from the latest
// one: an occurrence not among them is empty, and a candle not known on the
// way makes the result unknown. Percentile ignores empty values.
func TestValueWhenAndBarsSince(t *testing.T) {
	registry := testRegistry(t)
	// Pivot highs of 50 and 60 are confirmed 3 and 10 candles back, the RSI
	// is above 70 2 and 7 candles back, and prices fall by one per candle
	// back. Nothing is known missing candles back, when set.
	values := func(missing int) func(Read) (float64, bool) {
		return func(read Read) (float64, bool) {
			if missing > 0 && read.Shift == missing {
				return 0, false
			}
			switch read.Variable.Name {
			case `pivot_high("1h")`:
				if high, confirmed := map[int]float64{3: 50, 10: 60}[read.Shift]; confirmed {
					return high, true
				}
				return math.NaN(), true
			case `rsi("1h")`:
				return map[bool]float64{true: 80, false: 40}[read.Shift == 2 || read.Shift == 7], true
			}
			return 100 - float64(read.Shift), true
		}
	}
	for _, test := range []struct {
		source  string
		missing int
		want    float64
		empty   bool
		unknown bool
	}{
		{source: `ValueWhen(pivot_high("1h"))`, want: 50},
		{source: `ValueWhen(pivot_high("1h"), 1)`, want: 60},
		{source: `ValueWhen(pivot_high("1h"), 2)`, empty: true},
		{source: `ValueWhen(rsi("1h") > 70, close("1h"), 1)`, want: 93},
		{source: `ValueWhen(rsi("1h") > 70 && close("1h") < 95, close("1h"), 0)`, want: 93},
		{source: `ValueWhen(rsi("1h") > 90, close("1h"), 0)`, empty: true},
		{source: `ValueWhen(pivot_high("1h") > 0, Prev(low("1h"), 5), 1)`, want: 85},
		{source: `BarsSince(pivot_high("1h"))`, want: 3},
		{source: `BarsSince(rsi("1h") > 70)`, want: 2},
		{source: `BarsSince(rsi("1h") > 30)`, want: 0},
		{source: `BarsSince(rsi("1h") > 90)`, empty: true},
		{source: `Prev(BarsSince(pivot_high("1h")), 1)`, want: 2},
		{source: `Of("ETHUSDT", BarsSince(pivot_high("1h"))) + close("1h") * 0`, want: 3},
		{source: `Prev(close("1h"), BarsSince(pivot_high("1h")) + 5)`, want: 92},
		{source: `Prev(close("1h"), BarsSince(rsi("1h") > 90) + 5)`, empty: true},
		// The condition is true at the latest candle: a shift of 0.
		{source: `Prev(close("1h"), BarsSince(rsi("1h") > 30))`, want: 100},
		// The occurrence is found before the unknown candle, or behind it.
		{source: `ValueWhen(pivot_high("1h"))`, missing: 5, want: 50},
		{source: `ValueWhen(pivot_high("1h"), 1)`, missing: 5, unknown: true},
		{source: `ValueWhen(rsi("1h") > 70, close("1h"), 1)`, missing: 5, unknown: true},
		{source: `BarsSince(rsi("1h") > 70)`, missing: 1, unknown: true},
		{source: `Percentile(pivot_high("1h"), 12, 50)`, want: 50},
		{source: `Percentile(pivot_high("1h"), 12, 51)`, want: 60},
		{source: `Percentile(pivot_high("1h"), 2, 50)`, empty: true},
		{source: `Percentile(pivot_high("1h"), 12, 50)`, missing: 5, unknown: true},
	} {
		price, err := Compile(test.source, registry, PriceRule)
		if err != nil {
			t.Fatalf("Compile(%s) error = %v", test.source, err)
		}
		// An empty price is not known, so a comparison tells empty, which is
		// false, from unknown.
		condition, err := Compile("!("+test.source+" >= 0)", registry, EntryRule)
		if err != nil {
			t.Fatalf("Compile(%s) error = %v", test.source, err)
		}
		got, known, err := price.Price(values(test.missing))
		empty, decided, conditionErr := condition.Evaluate(values(test.missing))
		if err != nil || conditionErr != nil || known != (!test.empty && !test.unknown) || known && got != test.want || decided == test.unknown || empty != test.empty {
			t.Errorf("%s with candle %d missing = %v (%v), empty %v (%v), %v, %v", test.source, test.missing, got, known, empty, decided, err, conditionErr)
		}
	}

	for source, problem := range map[string]string{
		`ValueWhen(BarsSince(rsi("1h") > 70), 1) > 0`:                                                          "ValueWhen cannot contain BarsSince",
		`BarsSince(ValueWhen(pivot_high("1h")) > 1) > 0`:                                                       "BarsSince cannot contain ValueWhen",
		`BarsSince(Percentile(close("1h"), 5, 50) > 1) > 0`:                                                    "BarsSince cannot contain Percentile",
		`BarsSince(Prev(close("1h"), bars_held) > 1) > 0`:                                                      "BarsSince cannot contain Prev with a calculated shift",
		`Percentile(ValueWhen(pivot_high("1h")), 5, 50) > 1`:                                                   "Percentile cannot contain ValueWhen",
		`Prev(ValueWhen(pivot_high("1h")), bars_held) > 1`:                                                     "Prev with a calculated shift cannot contain ValueWhen",
		`Prev(close("1h"), BarsSince(pivot_low("1h")) + 5) > 1`:                                                "",
		`Prev(close("1h"), BarsSince(pivot_low("1h")) * 2) > 1`:                                                shiftProblem,
		`Prev(close("1h"), ValueWhen(pivot_low("1h"))) > 1`:                                                    shiftProblem,
		`BarsSince(pnl > 1) > 3`:                                                                               "pnl cannot be read through Prev, Percentile, ValueWhen, BarsSince, crossings, or Of",
		`ValueWhen(pivot_high("1h"), 200) > 1`:                                                                 "the ValueWhen occurrence must be a whole number from 0 to 199",
		`ValueWhen(pivot_high("1h"), close("1h"), 0) > 1`:                                                      "ValueWhen takes a condition, a value, and an occurrence, or a value and an optional occurrence",
		`ValueWhen(rsi("1h") > 70) > 1`:                                                                        "ValueWhen takes a condition, a value, and an occurrence, or a value and an optional occurrence",
		`bars_since(pivot_high("1h")) > value_when(close("1h"))`:                                               "unknown function bars_since; did you mean BarsSince?; unknown function value_when; did you mean ValueWhen?",
		strings.Repeat(`BarsSince(rsi("1h") > 70) < 5 && `, 9) + `BarsSince(CrossesAbove(rsi("1h"), 70)) < 5`:  "",
		strings.Repeat(`BarsSince(rsi("1h") > 70) < 5 && `, 10) + `BarsSince(CrossesAbove(rsi("1h"), 70)) < 5`: "the expression has too many comparisons",
	} {
		_, err := Compile(source, registry, ExitRule)
		if problem == "" {
			if err != nil {
				t.Errorf("Compile(%q) error = %v", source, err)
			}
			continue
		}
		if err == nil || !strings.HasSuffix(err.Error(), problem) {
			t.Errorf("Compile(%q) error = %v, want %q", source, err, problem)
		}
	}

	canonical, _ := Canonical(`ValueWhen(rsi("1h", 14) > 70, pivot_high("1h", 5, 5), 0) > BarsSince(pivot_low("1h", 5))`, registry)
	if want := `ValueWhen(rsi("1h") > 70, pivot_high("1h"), 0) > BarsSince(pivot_low("1h"))`; canonical != want {
		t.Fatalf("Canonical() = %s, want %s", canonical, want)
	}
}

// An indicator call the indicator service cannot describe, such as one past
// the chart limit, is a problem of the expression, not a failed validation.
func TestValidateListsProblemsOfIndicatorsItCannotDescribe(t *testing.T) {
	service := validationTestService(t)
	validation, err := service.Validate(context.Background(), `t3("1h", 1000) > 1 && rsi("1h") > 1`, EntryRule)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if len(validation.Problems) == 0 || len(validation.Indicators) != 1 || validation.Indicators[0].Selection.Type != "rsi" {
		t.Fatalf("Validate() = %+v, want problems and only rsi described", validation)
	}
}
