package strategy

import (
	"testing"
	"time"

	"crypto-scanner/internal/market"
)

// A range needs exactly the latest 24 hourly candles, continuous and each
// with a positive open.
func TestSignalRangeNeedsTheLatestDay(t *testing.T) {
	latest := backtestHour(30)
	day := func() []market.Candle {
		candles := make([]market.Candle, signalRangeHours)
		for i := range candles {
			// Ranges of 1% to 24%.
			candles[i] = market.Candle{OpenTime: latest.Add(time.Duration(i-signalRangeHours+1) * time.Hour), Open: 100, High: 100 + float64(i+1), Low: 100}
		}
		return candles
	}
	if got, ok := signalRange(day(), latest); !ok || got != 5 {
		t.Fatalf("signalRange() = %v, %v, want 5, true", got, ok)
	}
	gap := day()
	gap[0].OpenTime = gap[0].OpenTime.Add(-time.Hour)
	zeroOpen := day()
	zeroOpen[3].Open = 0
	for name, candles := range map[string][]market.Candle{"short": day()[1:], "stale": day()[:signalRangeHours], "gap": gap, "zero open": zeroOpen} {
		at := latest
		if name == "stale" {
			at = latest.Add(time.Hour)
		}
		if _, ok := signalRange(candles, at); ok {
			t.Fatalf("signalRange() of %s candles is known", name)
		}
	}
}

// Every signal message describes a coin by its links, labeled price, and
// range: n/a without one, left out when unloaded.
func TestSignalCoinShowsPriceAndRange(t *testing.T) {
	btc, eth := Instrument{ID: 1, Symbol: "BTCUSDT"}, Instrument{ID: 2, Symbol: "ETHUSDT"}
	links := testCoins.Symbol("BTCUSDT")
	ranges := coinRanges{loaded: true, values: map[int64]float64{1: 1.844}}
	for _, test := range []struct {
		got, want string
	}{
		{signalCoin(testCoins, btc, 65000, true, ranges), links + " · price 65000 · range 1.84%"},
		{signalCoin(testCoins, eth, 3400, true, ranges), testCoins.Symbol("ETHUSDT") + " · price 3400 · range n/a"},
		{signalCoin(testCoins, btc, 0, false, coinRanges{}), links},
	} {
		if test.got != test.want {
			t.Fatalf("signalCoin() = %q, want %q", test.got, test.want)
		}
	}
}
