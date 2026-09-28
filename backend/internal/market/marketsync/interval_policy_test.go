package marketsync

import (
	"testing"

	"crypto-scanner/internal/market"
)

func TestPolicyForSupportedIntervals(t *testing.T) {
	for _, interval := range market.CandleIntervals() {
		policy := policyForInterval(interval, 1000)
		if policy.interval != interval || policy.initialLimit != 1000 || !policy.repairGaps {
			t.Fatalf("policyForInterval(%q) = %+v", interval, policy)
		}
	}
}

func TestPolicyRejectsUnknownIntervalForGapRepair(t *testing.T) {
	policy := policyForInterval("4h", 1000)
	if policy.repairGaps {
		t.Fatalf("unexpected gap repair policy: %+v", policy)
	}
}
