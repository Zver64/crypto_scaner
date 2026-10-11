package strategy

import (
	"testing"
	"time"

	"crypto-scanner/internal/market"
)

func TestSignalExitTimeKeepsFirstDecisionWhileMoveContinues(t *testing.T) {
	for _, direction := range []Direction{DirectionLong, DirectionShort} {
		for _, test := range []struct {
			name  string
			first market.Candle
			want  SignalOutcome
		}{
			{"target", ohlc(100, 103, 99.5, 102), SignalSuccess},
			{"stop", ohlc(100, 100.5, 98, 99), SignalFailure},
			{"both", ohlc(100, 103, 98, 100), SignalFailure},
			{"open at target", ohlc(103, 104, 98, 100), SignalSuccess},
			{"open at stop", ohlc(98, 104, 97, 100), SignalFailure},
		} {
			t.Run(string(direction)+"/"+test.name, func(t *testing.T) {
				future := []market.Candle{test.first, ohlc(102, 104, 100, 103), ohlc(103, 104, 98, 100)}
				start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
				for i := range future {
					future[i].OpenTime = start.Add(time.Duration(i) * time.Hour)
					if direction == DirectionShort {
						c := &future[i]
						c.Open, c.High, c.Low, c.Close = 200-c.Open, 200-c.Low, 200-c.High, 200-c.Close
					}
				}
				outcome, move, exit := directionalOutcome(direction, 100, .01, .02, future)
				if outcome != test.want || !exit.Equal(start) {
					t.Fatalf("got %s exit %v, want %s exit %v", outcome, exit, test.want, start)
				}
				if test.want == SignalSuccess && test.name == "target" && move != .04 {
					t.Fatalf("move stopped at target: %v", move)
				}
			})
		}
	}
}
