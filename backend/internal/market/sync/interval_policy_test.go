package sync

import (
	"testing"

	"crypto-scanner/internal/market"
)

func TestPolicyForInterval(t *testing.T) {
	tests := []struct {
		name     string
		interval string
		want     intervalPolicy
	}{
		{
			name:     "hourly",
			interval: "1h",
			want: intervalPolicy{
				inspectionLimit: market.SevenDayPriceSlots,
				initialLimit:    market.SevenDayPriceSlots,
				repairGaps:      true,
			},
		},
		{name: "daily", interval: "1d", want: intervalPolicy{inspectionLimit: 1, initialLimit: 30}},
		{name: "other interval falls back", interval: "4h", want: intervalPolicy{inspectionLimit: 1, initialLimit: 30}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := policyForInterval(test.interval); got != test.want {
				t.Fatalf("policyForInterval(%q) = %+v, want %+v", test.interval, got, test.want)
			}
		})
	}
}
