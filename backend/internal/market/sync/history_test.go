package sync_test

import (
	"context"
	"sort"
	"testing"
	"testing/synctest"
	"time"

	"crypto-scanner/internal/market"
	marketsync "crypto-scanner/internal/market/sync"
)

func TestHourlySyncLoadsSevenDaysOfClosedPrices(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		instrument := market.Instrument{ID: 1, Symbol: "BTCUSDT", Active: true}
		exchange := &historyExchange{instrument: instrument, candles: hourlyCandles(170)}
		store := &historyStore{fakeMarketStore: fakeMarketStore{active: []market.Instrument{instrument}}}
		if err := marketsync.NewWithProfile(exchange, store, nil, 1, marketsync.HourlyProfile()).Sync(t.Context()); err != nil {
			t.Fatal(err)
		}
		got, _ := store.ListLatestCandlesByInterval(t.Context(), 1, "1h", 1000)
		if len(got) != 169 || !got[0].OpenTime.Equal(time.Now().Add(-time.Hour)) || !got[168].OpenTime.Equal(time.Now().Add(-169*time.Hour)) {
			t.Fatalf("wanted 169 closed prices spanning 168 hours, got %d candles", len(got))
		}
	})
}

func TestHourlySyncRepairsExistingShortAndGappedHistory(t *testing.T) {
	for _, scenario := range []string{"short", "internal gap", "stale", "new listing", "paginated outage"} {
		t.Run(scenario, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				instrument := market.Instrument{ID: 1, Symbol: "BTCUSDT", Active: true}
				all := hourlyCandles(170)
				stored := append([]market.Candle(nil), all[109:169]...)
				want := 169
				switch scenario {
				case "internal gap":
					stored = append(append([]market.Candle(nil), all[:80]...), all[81:169]...)
				case "stale":
					stored = append([]market.Candle(nil), all[10:50]...)
				case "new listing":
					all = all[120:]
					stored = nil
					want = 49
				case "paginated outage":
					all = hourlyCandles(1202)
					stored = append([]market.Candle(nil), all[:1]...)
					want = 1201
				}
				exchange := &historyExchange{instrument: instrument, candles: all}
				store := &historyStore{fakeMarketStore: fakeMarketStore{active: []market.Instrument{instrument}}, candles: stored}
				synchronizer := marketsync.NewWithProfile(exchange, store, nil, 1, marketsync.HourlyProfile())
				for run := range 2 {
					if err := synchronizer.Sync(t.Context()); err != nil {
						t.Fatal(err)
					}
					got, _ := store.ListLatestCandlesByInterval(t.Context(), 1, "1h", 2000)
					if len(got) != want {
						t.Fatalf("run %d: got %d candles, want %d", run, len(got), want)
					}
					for i, candle := range got {
						if !candle.OpenTime.Equal(time.Now().Add(-time.Duration(i+1) * time.Hour)) {
							t.Fatalf("gap/open candle at %s", candle.OpenTime)
						}
					}
				}
			})
		})
	}
}

// A deterministic exchange fixture honors range and page limits and includes
// the open candle so the synchronization boundary must exclude it.
type historyExchange struct {
	instrument market.Instrument
	candles    []market.Candle
}

func (e *historyExchange) ListInstruments(context.Context) ([]market.Instrument, error) {
	return []market.Instrument{e.instrument}, nil
}

func (e *historyExchange) ListClosedCandles(_ context.Context, request market.CandleRequest) ([]market.Candle, error) {
	var result []market.Candle
	for _, candle := range e.candles {
		if request.AfterOpenTime == nil || candle.OpenTime.After(*request.AfterOpenTime) {
			result = append(result, candle)
		}
	}
	if len(result) > request.Limit {
		if request.AfterOpenTime == nil {
			result = result[len(result)-request.Limit:]
		} else {
			result = result[:request.Limit]
		}
	}
	return result, nil
}

type historyStore struct {
	fakeMarketStore
	candles []market.Candle
}

func (s *historyStore) ListLatestCandlesByInterval(_ context.Context, _ int64, _ string, limit int) ([]market.Candle, error) {
	result := append([]market.Candle(nil), s.candles...)
	sort.Slice(result, func(i, j int) bool { return result[i].OpenTime.After(result[j].OpenTime) })
	return result[:min(len(result), limit)], nil
}

func (s *historyStore) UpsertCandles(_ context.Context, candles []market.Candle) error {
	for _, candle := range candles {
		found := false
		for i := range s.candles {
			if s.candles[i].OpenTime.Equal(candle.OpenTime) {
				s.candles[i] = candle
				found = true
				break
			}
		}
		if !found {
			s.candles = append(s.candles, candle)
		}
	}
	return nil
}

func hourlyCandles(count int) []market.Candle {
	candles := make([]market.Candle, count)
	for i := range candles {
		open := time.Now().UTC().Truncate(time.Hour).Add(time.Duration(i-count+1) * time.Hour)
		candles[i] = market.Candle{InstrumentID: 1, Interval: "1h", OpenTime: open, CloseTime: open.Add(time.Hour - time.Millisecond), Close: float64(i + 1)}
	}
	return candles
}
