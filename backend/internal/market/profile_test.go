package market

import "testing"

func TestSyncProfilesPreserveFieldsAndKeys(t *testing.T) {
	tests := []struct {
		name     string
		profile  SyncProfile
		wantKey  string
		interval string
	}{
		{name: "daily", profile: DailySyncProfile(), wantKey: "binance:spot:USDT:1d:UTC", interval: "1d"},
		{name: "hourly", profile: HourlySyncProfile(), wantKey: "binance:spot:USDT:1h:UTC", interval: "1h"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.profile.Exchange != "binance" || test.profile.Market != "spot" || test.profile.QuoteAsset != "USDT" || test.profile.Interval != test.interval || test.profile.TimeZone != "UTC" {
				t.Fatalf("profile = %+v", test.profile)
			}
			if got := test.profile.Key(); got != test.wantKey {
				t.Fatalf("Key() = %q, want %q", got, test.wantKey)
			}
		})
	}
}

func TestSyncProfilesReturnIndependentValues(t *testing.T) {
	daily := DailySyncProfile()
	daily.Interval = "mutated"
	if got := DailySyncProfile().Interval; got != "1d" {
		t.Fatalf("DailySyncProfile().Interval = %q, want 1d", got)
	}

	hourly := HourlySyncProfile()
	hourly.Interval = "mutated"
	if got := HourlySyncProfile().Interval; got != "1h" {
		t.Fatalf("HourlySyncProfile().Interval = %q, want 1h", got)
	}
}
