package strategy

import (
	"math"
	"testing"

	"crypto-scanner/internal/market"
)

func TestSignalFinalCloseOutcome(t *testing.T) {
	for _, direction := range []Direction{DirectionLong, DirectionShort} {
		sign := 1.0
		if direction == DirectionShort {
			sign = -1
		}
		for _, test := range []struct {
			name string
			move float64
			want SignalOutcome
		}{
			{"against entry", -0.5, SignalFailure},
			{"at entry", 0, SignalNeutral},
			{"below one ATR", 0.99, SignalNeutral},
			{"at one ATR", 1, SignalSuccess},
			{"above one ATR", 1.5, SignalSuccess},
		} {
			t.Run(string(direction)+"/"+test.name, func(t *testing.T) {
				quiet := ohlc(100, 100.1, 99.9, 100)
				close := 100 + sign*test.move
				last := ohlc(100, max(100.1, close), min(99.9, close), close)
				history := signalHistory(quiet, quiet, quiet, quiet, quiet, last)
				got := evaluateTestSignal(t, direction, market.IntervalHour, history, signalHistoryCandles, 6, 2)
				if got == nil || got.Outcome != test.want || !got.ExitTime.Equal(history[len(history)-1].OpenTime) {
					t.Fatalf("got %+v, want %s", got, test.want)
				}
			})
		}
	}
}

// Compare the final close to a price level, not a divided return: a close
// exactly one ATR forward must not fall below the boundary through rounding.
func TestSignalFinalCloseFractionalATRBoundary(t *testing.T) {
	for _, direction := range []Direction{DirectionLong, DirectionShort} {
		sign := 1.0
		if direction == DirectionShort {
			sign = -1
		}
		level := 100 + sign*2.3
		for _, test := range []struct {
			close float64
			want  SignalOutcome
		}{
			{level, SignalSuccess}, {math.Nextafter(level, 100), SignalNeutral},
		} {
			future := []market.Candle{ohlc(100, max(100.1, test.close), min(99.9, test.close), test.close)}
			got, _, _ := directionalOutcome(direction, 100, 0.023, 0.046, future)
			if got != test.want {
				t.Fatalf("%s close %.17g: got %s, want %s", direction, test.close, got, test.want)
			}
		}
	}
}

func TestSignalFinalCloseNotMaximumExcursion(t *testing.T) {
	history := signalHistory(ohlc(100, 101.5, 99.9, 101), ohlc(101, 101.2, 100.4, 100.5))
	got := evaluateTestSignal(t, DirectionLong, market.IntervalHour, history, signalHistoryCandles, 2, 2)
	if got == nil || got.Outcome != SignalNeutral {
		t.Fatalf("got %+v, want neutral", got)
	}
}

func TestSignalStatsExcludeNeutrals(t *testing.T) {
	var stats SignalStats
	if stats.SuccessRate() != nil {
		t.Fatal("empty stats have a rate")
	}
	stats.record(&SignalEvaluation{Outcome: SignalNeutral})
	if stats.SuccessRate() != nil {
		t.Fatal("only neutral stats have a rate")
	}
	stats.record(&SignalEvaluation{Outcome: SignalSuccess})
	stats.record(&SignalEvaluation{Outcome: SignalFailure})
	if stats.Count != 3 || stats.Neutrals != 1 || stats.Successes != 1 || stats.SuccessRate() == nil || *stats.SuccessRate() != 0.5 {
		t.Fatalf("unexpected stats %+v", stats)
	}
}

func TestJudgeSignalCountsNeutralsAndSpacing(t *testing.T) {
	quiet := ohlc(100, 100.1, 99.9, 100)
	history := signalHistory(quiet, quiet, quiet, quiet, quiet, quiet)
	at := signalHistoryCandles
	occurrences := []SignalOccurrence{{}, {}, {}}
	report, err := judgeSignal(testRegistry(t), Strategy{Direction: DirectionLong, Window: 3, TargetRatio: 2}, market.IntervalHour, history,
		occurrences, []int{at, at + 1, at + 3}, at, at+3)
	if err != nil {
		t.Fatal(err)
	}
	if report.Evaluated != 3 || report.Signals.Count != 2 || report.Signals.Neutrals != 2 || report.Signals.SuccessRate() != nil ||
		report.All.Count != 4 || report.All.Neutrals != 4 || report.All.SuccessRate() != nil ||
		occurrences[1].Evaluation.Counted {
		t.Fatalf("unexpected report %+v", report)
	}
}
