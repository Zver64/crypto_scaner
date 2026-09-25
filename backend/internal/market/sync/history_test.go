package sync_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"testing"
	"testing/synctest"
	"time"

	"crypto-scanner/internal/market"
	marketsync "crypto-scanner/internal/market/sync"
)

func TestSyncEnsuresHistoryDepthForEverySupportedInterval(t *testing.T) {
	for _, interval := range market.CandleIntervals() {
		interval := interval
		for _, scenario := range []string{"empty", "short", "sufficient", "longer", "internal gap", "stale", "young listing", "no closed history", "paginated forward"} {
			t.Run(string(interval)+"/"+scenario, func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					instrument := market.Instrument{ID: 1, Symbol: "BTCUSDT", Active: true}
					all := intervalCandles(interval, 1202)
					var stored []market.Candle
					want := 1000
					switch scenario {
					case "empty":
					case "short":
						stored = cloneCandles(all[len(all)-51 : len(all)-1])
					case "sufficient":
						stored = cloneCandles(all[len(all)-1001 : len(all)-1])
					case "longer":
						stored = cloneCandles(all[len(all)-1101 : len(all)-1])
						want = 1100
					case "internal gap":
						stored = cloneCandles(all[len(all)-1001 : len(all)-1])
						stored = append(stored[:400], stored[401:]...)
					case "stale":
						stored = cloneCandles(all[len(all)-61 : len(all)-11])
					case "young listing":
						all = intervalCandles(interval, 74)
						stored = cloneCandles(all[23:73])
						want = 73
					case "no closed history":
						all = intervalCandles(interval, 1)
						want = 0
					case "paginated forward":
						all = intervalCandles(interval, 2202)
						stored = cloneCandles(all[:1])
						want = 2201
					}

					exchange := &historyExchange{instrument: instrument, candles: all}
					store := &historyStore{fakeMarketStore: fakeMarketStore{active: []market.Instrument{instrument}}, candles: stored}
					synchronizer := marketsync.NewWithProfile(exchange, store, nil, 1, marketsync.Profile(interval))
					if err := synchronizer.Sync(t.Context()); err != nil {
						t.Fatal(err)
					}
					assertContinuousHistory(t, store, interval, want)
					requestsAfterRepair := len(exchange.requests)

					restarted := marketsync.NewWithProfile(exchange, store, nil, 1, marketsync.Profile(interval))
					if err := restarted.Sync(t.Context()); err != nil {
						t.Fatal(err)
					}
					assertContinuousHistory(t, store, interval, want)
					wantRecheck := 1
					if scenario == "no closed history" {
						wantRecheck = 0
					}
					if len(exchange.requests)-requestsAfterRepair != wantRecheck {
						t.Fatalf("completed repair made %d requests, want %d latest-close recheck", len(exchange.requests)-requestsAfterRepair, wantRecheck)
					}
					if scenario == "short" {
						last := exchange.requests[requestsAfterRepair-1]
						if !last.HistoryRepair || last.AfterOpenTime != nil || !last.ClosedBefore.Equal(stored[0].OpenTime) || last.Limit != 950 {
							t.Fatalf("prefix request = %#v, want 950 candles exclusively before oldest row", last)
						}
					}
				})
			})
		}
	}
}

func TestForwardPaginationResumesFromPersistedProgressAfterFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		instrument := market.Instrument{ID: 1, Symbol: "BTCUSDT", Active: true}
		all := intervalCandles(market.IntervalHour, 2202)
		store := &historyStore{
			fakeMarketStore: fakeMarketStore{active: []market.Instrument{instrument}},
			candles:         cloneCandles(all[:1]),
		}
		exchangeErr := errors.New("interrupted after first page")
		exchange := &historyExchange{instrument: instrument, candles: all, failAtRequest: 2, requestErr: exchangeErr}
		synchronizer := marketsync.NewWithProfile(exchange, store, nil, 1, marketsync.HourlyProfile())

		if err := synchronizer.Sync(t.Context()); !errors.Is(err, exchangeErr) {
			t.Fatalf("Sync() error = %v, want pagination failure", err)
		}
		if len(store.candles) != 1000 {
			t.Fatalf("persisted progress = %d candles, want first page retained including overlap", len(store.candles))
		}
		exchange.failAtRequest = 0
		if err := synchronizer.Sync(t.Context()); err != nil {
			t.Fatal(err)
		}
		assertContinuousHistory(t, store, market.IntervalHour, 2201)
	})
}

func TestHistoryRepairDatabaseFailureDoesNotAdvanceCoverage(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		instrument := market.Instrument{ID: 1, Symbol: "BTCUSDT", Active: true}
		all := intervalCandles(market.IntervalDay, 1202)
		storeErr := errors.New("temporary database failure")
		store := &historyStore{
			fakeMarketStore: fakeMarketStore{active: []market.Instrument{instrument}},
			candles:         cloneCandles(all[len(all)-51 : len(all)-1]),
			upsertErrOnce:   storeErr,
		}
		exchange := &historyExchange{instrument: instrument, candles: all}
		synchronizer := marketsync.NewWithProfile(exchange, store, nil, 1, marketsync.MVPProfile())

		if err := synchronizer.Sync(t.Context()); !errors.Is(err, storeErr) {
			t.Fatalf("Sync() error = %v, want storage failure", err)
		}
		if len(store.candles) != 50 || len(store.coverage) != 0 {
			t.Fatalf("failed write advanced history=%d or coverage=%#v", len(store.candles), store.coverage)
		}
		if err := synchronizer.Sync(t.Context()); err != nil {
			t.Fatal(err)
		}
		assertContinuousHistory(t, store, market.IntervalDay, 1000)
	})
}

func TestHistoryRepairFailureKeepsDurableProgressAndDoesNotMarkExhausted(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		instrument := market.Instrument{ID: 1, Symbol: "BTCUSDT", Active: true}
		all := intervalCandles(market.IntervalDay, 1202)
		stored := cloneCandles(all[len(all)-51 : len(all)-1])
		exchangeErr := errors.New("temporary Binance failure")
		exchange := &historyExchange{instrument: instrument, candles: all, repairErr: exchangeErr}
		store := &historyStore{fakeMarketStore: fakeMarketStore{active: []market.Instrument{instrument}}, candles: stored}
		synchronizer := marketsync.NewWithProfile(exchange, store, nil, 1, marketsync.MVPProfile())

		if err := synchronizer.Sync(t.Context()); !errors.Is(err, exchangeErr) {
			t.Fatalf("Sync() error = %v, want repair failure", err)
		}
		if len(store.coverage) != 0 {
			t.Fatalf("API failure persisted false exhaustion marker: %#v", store.coverage)
		}
		exchange.repairErr = nil
		if err := synchronizer.Sync(t.Context()); err != nil {
			t.Fatal(err)
		}
		assertContinuousHistory(t, store, market.IntervalDay, 1000)
	})
}

func assertContinuousHistory(t *testing.T, store *historyStore, interval market.CandleInterval, want int) {
	t.Helper()
	got, err := store.ListLatestCandlesByInterval(t.Context(), 1, string(interval), 3000)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != want {
		t.Fatalf("got %d candles, want %d", len(got), want)
	}
	seen := make(map[time.Time]struct{}, len(got))
	for index, candle := range got {
		if _, duplicate := seen[candle.OpenTime]; duplicate {
			t.Fatalf("duplicate candle at %s", candle.OpenTime)
		}
		seen[candle.OpenTime] = struct{}{}
		if index > 0 && !interval.NextOpenTime(candle.OpenTime).Equal(got[index-1].OpenTime) {
			t.Fatalf("history gap between %s and %s", candle.OpenTime, got[index-1].OpenTime)
		}
	}
}

// A deterministic exchange fixture honors both exclusive boundaries and the
// Binance page limit. Its source also includes the currently open candle.
type historyExchange struct {
	instrument    market.Instrument
	candles       []market.Candle
	requests      []market.CandleRequest
	repairErr     error
	failAtRequest int
	requestErr    error
}

func (e *historyExchange) ListInstruments(context.Context) ([]market.Instrument, error) {
	return []market.Instrument{e.instrument}, nil
}

func (e *historyExchange) ListClosedCandles(_ context.Context, request market.CandleRequest) ([]market.Candle, error) {
	e.requests = append(e.requests, request)
	if e.failAtRequest > 0 && len(e.requests) == e.failAtRequest {
		return nil, e.requestErr
	}
	if request.HistoryRepair && e.repairErr != nil {
		return nil, e.repairErr
	}
	var result []market.Candle
	for _, candle := range e.candles {
		if candle.CloseTime.Before(request.ClosedBefore) && (request.AfterOpenTime == nil || candle.OpenTime.After(*request.AfterOpenTime)) {
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
	candles       []market.Candle
	coverage      map[string]market.HistoryCoverage
	upsertErrOnce error
}

func (s *historyStore) ListLatestCandlesByInterval(_ context.Context, _ int64, _ string, limit int) ([]market.Candle, error) {
	result := cloneCandles(s.candles)
	sort.Slice(result, func(i, j int) bool { return result[i].OpenTime.After(result[j].OpenTime) })
	return result[:min(len(result), limit)], nil
}

func (s *historyStore) UpsertCandlesWithChanges(ctx context.Context, candles []market.Candle) ([]market.Candle, error) {
	previous := cloneCandles(s.candles)
	if err := s.UpsertCandles(ctx, candles); err != nil {
		return nil, err
	}
	changed := make([]market.Candle, 0, len(candles))
	for _, candle := range candles {
		found := false
		for _, old := range previous {
			if old.Interval == candle.Interval && old.OpenTime.Equal(candle.OpenTime) {
				found = true
				if !reflect.DeepEqual(old, candle) {
					changed = append(changed, candle)
				}
				break
			}
		}
		if !found {
			changed = append(changed, candle)
		}
	}
	return changed, nil
}

func (s *historyStore) UpsertCandles(_ context.Context, candles []market.Candle) error {
	if len(candles) > 0 && s.upsertErrOnce != nil {
		err := s.upsertErrOnce
		s.upsertErrOnce = nil
		return err
	}
	for _, candle := range candles {
		found := false
		for i := range s.candles {
			if s.candles[i].Interval == candle.Interval && s.candles[i].OpenTime.Equal(candle.OpenTime) {
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

func (s *historyStore) GetCandleHistoryCoverage(_ context.Context, instrumentID int64, interval market.CandleInterval) (market.HistoryCoverage, bool, error) {
	coverage, found := s.coverage[coverageKey(instrumentID, interval)]
	return coverage, found, nil
}

func (s *historyStore) SaveCandleHistoryCoverage(_ context.Context, coverage market.HistoryCoverage) error {
	if s.coverage == nil {
		s.coverage = map[string]market.HistoryCoverage{}
	}
	s.coverage[coverageKey(coverage.InstrumentID, coverage.Interval)] = coverage
	return nil
}

func coverageKey(instrumentID int64, interval market.CandleInterval) string {
	return fmt.Sprintf("%d:%s", instrumentID, interval)
}

func cloneCandles(candles []market.Candle) []market.Candle {
	return append([]market.Candle(nil), candles...)
}

func intervalCandles(interval market.CandleInterval, count int) []market.Candle {
	open := interval.OpenTime(time.Now().UTC())
	for range count - 1 {
		open = previousOpenTime(interval, open)
	}
	candles := make([]market.Candle, count)
	for index := range candles {
		closeTime := interval.NextOpenTime(open).Add(-time.Millisecond)
		candles[index] = market.Candle{
			InstrumentID: 1, Interval: interval, OpenTime: open, CloseTime: closeTime,
			Open: 1, High: 2, Low: 0.5, Close: 1.5, Volume: 1,
		}
		open = interval.NextOpenTime(open)
	}
	return candles
}

func previousOpenTime(interval market.CandleInterval, open time.Time) time.Time {
	switch interval {
	case market.IntervalHour:
		return open.Add(-time.Hour)
	case market.IntervalDay:
		return open.AddDate(0, 0, -1)
	case market.IntervalWeek:
		return open.AddDate(0, 0, -7)
	case market.IntervalMonth:
		return open.AddDate(0, -1, 0)
	default:
		panic("unsupported candle interval")
	}
}
