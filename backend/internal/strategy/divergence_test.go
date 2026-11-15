package strategy

import (
	"context"
	"fmt"
	"math"
	"slices"
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
			// The pivots of the RSI, the price read at them, and the
			// pivots of the price, the RSI read at them.
			rsiPivot := fmt.Sprintf(`%s("1h", %d, %d, rsi("1h"))`, test.pivot, left, right)
			pricePivot := fmt.Sprintf(`%s("1h", %d, %d)`, test.pivot, left, right)
			// The RSI the rule reads for pivots of the price is rsi("1h"),
			// which warms up over fewer candles than the divergence, so it
			// differs by more of the remainder of the seed.
			for _, in := range []struct {
				name, pivot, rsi, price string
				tolerance               float64
			}{
				{"rsi", rsiPivot, rsiPivot, fmt.Sprintf(`Prev(%s("1h"), %d)`, test.price, right), 1e-4},
				{"price", pricePivot, fmt.Sprintf(`Prev(rsi("1h"), %d)`, right), pricePivot, 1e-3},
			} {
				name := fmt.Sprintf("%s %d/%d in %s", test.output, left, right, in.name)
				rule := fmt.Sprintf(`%[1]s > 0 && %[2]s %[4]s ValueWhen(%[1]s, %[2]s, 1) && %[3]s %[5]s ValueWhen(%[1]s, %[3]s, 1) && Prev(BarsSince(%[1]s), 1) >= 5 && Prev(BarsSince(%[1]s), 1) <= 60`,
					in.pivot, in.rsi, in.price, test.rsi, test.before)
				call := fmt.Sprintf(`divergence_%s("1h", %d, %d, %s)`, test.output, left, right, in.name)
				bars := fmt.Sprintf(`divergence_%s_bars("1h", %d, %d, %s)`, test.output, left, right, in.name)
				signal := call + " > 0"
				want := occurrences(rule)
				got := occurrences(signal)
				if len(want) < 3 || len(got) != len(want) {
					t.Fatalf("%s: %d signals, the rule %d", name, len(got), len(want))
				}
				for at := range want {
					if _, found := got[at]; !found {
						t.Errorf("%s: no signal at %v, where the rule has one", name, at)
					}
				}
				// The value is the RSI at the later pivot. The rule reads
				// earlier candles, which deepens its warm-up and changes the
				// RSI by the remainder of the seed.
				rsiAt := occurrences(fmt.Sprintf(`%s && Abs(%s - %s) < %g`, signal, call, in.rsi, in.tolerance))
				if len(rsiAt) != len(got) {
					t.Errorf("%s: the RSI at the pivot matches on %d of %d signals", name, len(rsiAt), len(got))
				}
				// The candles from the earlier pivot to the later one, where
				// charts start the line to the marker, are those the rule
				// counts between the confirmations and the later one.
				between := fmt.Sprintf(`Prev(BarsSince(%s), 1)`, in.pivot)
				all := occurrences(fmt.Sprintf(`%s && %s >= 0`, signal, between))
				matching := occurrences(fmt.Sprintf(`%s && Abs(%s - %s - 1) < 0.5`, signal, bars, between))
				if len(all) < 3 || len(matching) != len(all) {
					t.Errorf("%s: bars match the rule on %d of %d signals", name, len(matching), len(all))
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
		// The bars are delivered for the segments of the markers, not drawn.
		var names []string
		for _, calculation := range result.Charts[0].Indicators {
			if calculation.Type == pivot.DivergenceType {
				for _, series := range calculation.Series {
					names = append(names, series.Name)
				}
			}
		}
		if !slices.Equal(names, []string{"bull", "bull_bars", "bear", "bear_bars", "rsi"}) {
			t.Fatalf("divergence series = %v, want the lines and their segments", names)
		}
		return
	}
	t.Fatal("divergence is not drawn")
}
