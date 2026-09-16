package sync

import (
	"testing"
	"time"

	"crypto-scanner/internal/market"
)

func TestPolicyForSupportedIntervals(t *testing.T) {
	for _, interval := range market.CandleIntervals() {
		policy := policyForInterval(interval)
		if policy.interval != interval || policy.inspectionLimit != 1000 || policy.initialLimit != 1000 || !policy.repairGaps {
			t.Fatalf("policyForInterval(%q) = %+v", interval, policy)
		}
	}
}

func TestPolicyRejectsUnknownIntervalForGapRepair(t *testing.T) {
	policy := policyForInterval("4h")
	if policy.repairGaps {
		t.Fatalf("unexpected gap repair policy: %+v", policy)
	}
}

func TestMissingRangesGroupsCalendarLengthMonthlyGap(t *testing.T) {
	candles := []market.Candle{
		{OpenTime: time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)},
		{OpenTime: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)},
		{OpenTime: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
	}
	got := missingRanges(candles, market.IntervalMonth)
	if len(got) != 1 || !got[0].from.Equal(time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)) || !got[0].to.Equal(time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("missingRanges() = %#v", got)
	}
}
