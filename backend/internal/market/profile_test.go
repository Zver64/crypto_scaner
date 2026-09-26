package market

import "testing"

func TestSyncProfilesPreserveFieldsAndKeys(t *testing.T) {
	tests := []struct {
		name     string
		profile  SyncProfile
		wantKey  string
		interval CandleInterval
	}{
		{name: "hourly", profile: BinanceSpotSyncProfile(IntervalHour), wantKey: "binance:spot:USDT:1h:UTC", interval: IntervalHour},
		{name: "daily", profile: BinanceSpotSyncProfile(IntervalDay), wantKey: "binance:spot:USDT:1d:UTC", interval: IntervalDay},
		{name: "weekly", profile: BinanceSpotSyncProfile(IntervalWeek), wantKey: "binance:spot:USDT:1w:UTC", interval: IntervalWeek},
		{name: "monthly", profile: BinanceSpotSyncProfile(IntervalMonth), wantKey: "binance:spot:USDT:1M:UTC", interval: IntervalMonth},
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

func TestCoinMetadataSyncProfile(t *testing.T) {
	profile := CoinMetadataSyncProfile()
	if profile.Key() != "coingecko:coin_metadata:USD:1h:UTC" {
		t.Fatalf("profile = %+v", profile)
	}
}
