package strategy

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/indicator/pivot"
	"crypto-scanner/internal/market"
)

// The divergence indicator signals on exactly the candles of the RSI
// divergence rules of docs/strategy-language.md, with the RSI at the later
// pivot, and its markers sit on its own RSI line.
func TestDivergenceMatchesItsRules(t *testing.T) {
	// A wave of changing amplitude and period with noise, whose highs and
	// lows stand apart from the closes.
	hourly := make([]market.Candle, 1500)
	seed := uint32(7)
	noise := func() float64 {
		seed = seed*1664525 + 1013904223
		return float64(seed)/math.MaxUint32 - 0.5
	}
	for index := range hourly {
		x := float64(index)
		close := 100 + 10*math.Sin(x/7)*math.Sin(x/90) + 3*math.Sin(x/3.1) + 4*noise()
		hourly[index] = testCandle(1, market.IntervalHour, backtestHour(index), close)
		hourly[index].High, hourly[index].Low = close+1+2*noise(), close-1-2*noise()
	}
	service := snapshotTestService(t, newBacktestStore(market.IntervalHour, hourly))

	occurrences := func(expression string) map[time.Time]float64 {
		t.Helper()
		result, err := service.BacktestDraft(context.Background(), Strategy{Signal: true, Direction: DirectionLong, Window: 6, TargetRatio: 2, Expression: expression}, "BTCUSDT", time.Time{}, time.Time{}, false)
		if err != nil {
			t.Fatalf("BacktestDraft(%s) error = %v", expression, err)
		}
		times := map[time.Time]float64{}
		for _, occurrence := range result.Signal.Occurrences {
			times[occurrence.Time] = math.NaN()
			for name, value := range occurrence.IndicatorValues {
				if strings.HasPrefix(name, "pivot_") || strings.HasPrefix(name, "divergence_bull(") || strings.HasPrefix(name, "divergence_bear(") {
					times[occurrence.Time] = value
				}
			}
		}
		return times
	}
	for _, sides := range [][2]int{{5, 5}, {3, 2}} {
		left, right := sides[0], sides[1]
		for _, test := range []struct{ output, pivot, price, rsi, before string }{
			{"bull", "pivot_low", "low", ">", "<"},
			{"bear", "pivot_high", "high", "<", ">"},
		} {
			pivot := fmt.Sprintf(`%s("1h", %d, %d, rsi("1h"))`, test.pivot, left, right)
			price := fmt.Sprintf(`Prev(%s("1h"), %d)`, test.price, right)
			rule := fmt.Sprintf(`%[1]s > 0 && %[1]s %[3]s ValueWhen(%[1]s, %[1]s, 1) && %[2]s %[4]s ValueWhen(%[1]s, %[2]s, 1) && Prev(BarsSince(%[1]s), 1) >= 5 && Prev(BarsSince(%[1]s), 1) <= 60`,
				pivot, price, test.rsi, test.before)
			want := occurrences(rule)
			got := occurrences(fmt.Sprintf(`divergence_%s("1h", %d, %d) > 0`, test.output, left, right))
			if len(want) < 3 || len(got) != len(want) {
				t.Fatalf("%s %d/%d: %d signals, the rule %d", test.output, left, right, len(got), len(want))
			}
			// The rule reads earlier candles, which deepens its warm-up and
			// changes the RSI by the remainder of the seed.
			for at, value := range want {
				if math.Abs(got[at]-value) > 1e-4 {
					t.Errorf("%s %d/%d at %v = %v, the rule %v", test.output, left, right, at, got[at], value)
				}
			}
		}
	}

	result, err := service.BacktestDraft(context.Background(), Strategy{Signal: true, Direction: DirectionLong, Window: 6, TargetRatio: 2, Expression: `divergence_bull("1h") > 0`}, "BTCUSDT", time.Time{}, time.Time{}, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, definition := range result.Charts[0].Definitions {
		if definition.Selection.Type != pivot.DivergenceType {
			continue
		}
		want := chart.LineAnchor{IndicatorID: definition.ID, Output: "rsi"}
		if len(definition.Lines) != 3 || definition.Lines[0].Anchor != want || definition.Lines[1].Anchor != want || definition.Lines[2].Output != "rsi" {
			t.Fatalf("divergence lines = %+v, want markers anchored to %+v", definition.Lines, want)
		}
		return
	}
	t.Fatal("divergence is not drawn")
}
