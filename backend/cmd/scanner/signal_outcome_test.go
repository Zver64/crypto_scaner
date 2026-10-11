package main

import (
	"crypto-scanner/internal/apiclient"
	"strings"
	"testing"
)

func TestSignalOutcomeRendering(t *testing.T) {
	for _, test := range []struct {
		outcome *apiclient.BacktestSignalOutcome
		text    string
	}{
		{nil, ""}, {new(apiclient.Success), "Yes"}, {new(apiclient.Failure), "No"}, {new(apiclient.Neutral), "Neutral"},
	} {
		if got := signalOutcome(test.outcome); got != test.text {
			t.Fatalf("got %q, want %q", got, test.text)
		}
	}
	stats := apiclient.BacktestSignalStats{Count: 7, Successes: 2, Neutrals: 4, SuccessRate: new(2.0 / 3)}
	if got := successful(stats); !strings.HasPrefix(got, "2/3 (") {
		t.Fatalf("neutral denominator: %q", got)
	}
	if got := successful(apiclient.BacktestSignalStats{Count: 4, Neutrals: 4}); got != "-" {
		t.Fatalf("neutral-only percentage: %q", got)
	}
}
