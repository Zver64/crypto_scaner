package marketsync_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"testing/synctest"

	"crypto-scanner/internal/closedindicator"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/indicator/candle"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/market/marketsync"
)

// A load extends the history backwards to the requested depth or to the
// oldest candle the exchange has, repeats no request once done, and leaves
// Sync, which keeps a shallower depth, to its latest-close recheck.
func TestLoadHistoryStopsAtDepthOrExhaustionAndSyncLeavesDeepRowsAlone(t *testing.T) {
	for _, test := range []struct {
		name      string
		listed    int
		stored    int
		want      int
		exhausted bool
	}{
		{name: "depth across live boundary", listed: 3002, stored: 1500, want: 2500},
		{name: "exhaustion", listed: 1502, stored: 1000, want: 1501, exhausted: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				instrument := market.Instrument{ID: 1, Symbol: "BTCUSDT", Active: true}
				all := intervalCandles(market.IntervalHour, test.listed)
				exchange := &historyExchange{instrument: instrument, candles: all}
				store := &historyStore{fakeMarketStore: fakeMarketStore{active: []market.Instrument{instrument}}, candles: cloneCandles(all[len(all)-1-test.stored : len(all)-1])}
				synchronizer := marketsync.New(exchange, store, slog.New(slog.DiscardHandler), 1, 1000, market.BinanceSpotSyncProfile(market.IntervalHour))

				notified := 0
				for attempt := range 2 {
					load, err := synchronizer.LoadHistory(t.Context(), instrument, 2500, func(candles []market.Candle) {
						if attempt != 0 {
							t.Error("unchanged history sent live notifications")
						}
						for _, candle := range candles {
							if candle.OpenTime.Before(all[len(all)-1-min(test.want, market.SyncDepth)].OpenTime) {
								t.Errorf("deep candle %s sent a live notification", candle.OpenTime)
							}
						}
						notified += len(candles)
					})
					if err != nil {
						t.Fatal(err)
					}
					if load.Count != test.want || !load.Oldest.Equal(all[len(all)-1-test.want].OpenTime) || load.Exhausted != test.exhausted {
						t.Fatalf("LoadHistory() = %+v, want %d candles, exhausted %v", load, test.want, test.exhausted)
					}
				}
				if want := min(test.want, market.SyncDepth) - test.stored; notified != want {
					t.Fatalf("notified %d candles, want only %d filling the live history", notified, want)
				}
				assertContinuousHistory(t, store, market.IntervalHour, test.want)
				// A shallower load reports the whole stored history.
				if load, err := synchronizer.LoadHistory(t.Context(), instrument, 1200, nil); err != nil || load.Count != test.want || load.Exhausted != test.exhausted {
					t.Fatalf("shallower LoadHistory() = %+v, %v", load, err)
				}
				for _, request := range exchange.requests {
					if !request.HistoryRepair {
						t.Fatalf("load request %#v is not a history repair", request)
					}
				}
				requests := len(exchange.requests)
				livePages := (min(test.want, market.SyncDepth) - test.stored + 999) / 1000
				deepPages := (max(0, test.want-market.SyncDepth) + 999) / 1000
				if want := livePages + deepPages; requests != want {
					t.Fatalf("loads made %d requests, want %d", requests, want)
				}

				if err := synchronizer.Sync(t.Context()); err != nil {
					t.Fatal(err)
				}
				if made := exchange.requests[requests:]; len(made) != 1 || made[0].HistoryRepair {
					t.Fatalf("Sync after the load made requests %#v, want only the latest-close recheck", made)
				}
				assertContinuousHistory(t, store, market.IntervalHour, test.want)
			})
		})
	}
}

// Committed pages survive a later failure, and the job reports the change
// even though no instrument/interval finished. Retrying resumes from storage.
func TestHistoryLoaderReportsPartialChangesAndResumesAfterFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		instrument := market.Instrument{ID: 1, Symbol: "BTCUSDT", Active: true}
		all := intervalCandles(market.IntervalHour, 3002)
		store := &historyStore{fakeMarketStore: fakeMarketStore{active: []market.Instrument{instrument}}, candles: cloneCandles(all[len(all)-1001 : len(all)-1])}
		exchange := &historyExchange{instrument: instrument, candles: all, failAtRequest: 2, requestErr: errors.New("second page failed")}
		synchronizer := marketsync.New(exchange, store, slog.New(slog.DiscardHandler), 1, 1000, market.BinanceSpotSyncProfile(market.IntervalHour))
		registry, err := indicator.NewRegistry(candle.New())
		if err != nil {
			t.Fatal(err)
		}
		subscriptions := historySubscriptions{{InstrumentID: 1, Target: closedindicator.Target{Interval: market.IntervalHour, Selection: indicator.Selection{Type: candle.Type}}, Points: 1500}}
		tracker, err := closedindicator.New(store, registry, subscriptions, nil, slog.New(slog.DiscardHandler))
		if err != nil {
			t.Fatal(err)
		}
		notified, rounds := 0, 0
		loader := marketsync.NewHistoryLoader(knownInstruments{}, map[market.CandleInterval]marketsync.HistoryExtender{market.IntervalHour: synchronizer}, slog.New(slog.DiscardHandler), marketsync.HistoryLoaderOptions{
			Changed: func(candles []market.Candle) {
				notified += len(candles)
				tracker.HistoryChanged(candles)
			},
			Synced: func() {
				rounds++
				tracker.HistorySynced()
			},
		})
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		done, tracked := make(chan error, 1), make(chan error, 1)
		go func() { tracked <- tracker.Run(ctx) }()
		synctest.Wait()
		assertHistoryShift(t, tracker, subscriptions, false)
		go func() { done <- loader.Run(ctx) }()
		intervals := []market.CandleInterval{market.IntervalHour}
		if _, err := loader.Start(t.Context(), []string{"BTCUSDT"}, intervals, 2500); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		job, _ := loader.Job()
		if job.Status != marketsync.HistoryJobFailed || !job.HistoryChanged || len(job.Items) != 0 {
			t.Fatalf("partial job = %+v", job)
		}
		assertContinuousHistory(t, store, market.IntervalHour, 2000)
		assertHistoryShift(t, tracker, subscriptions, true)
		if notified != 1000 || rounds != 1 {
			t.Fatalf("partial load notified %d candles and %d rounds", notified, rounds)
		}

		exchange.failAtRequest = 0
		if _, err := loader.Start(t.Context(), []string{"BTCUSDT"}, intervals, 2500); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		job, _ = loader.Job()
		if job.Status != marketsync.HistoryJobDone || !job.HistoryChanged || len(job.Items) != 1 || job.Items[0].Count != 2500 {
			t.Fatalf("retried job = %+v", job)
		}
		if len(exchange.requests) != 3 || exchange.requests[2].Limit != 500 {
			t.Fatalf("retry did not resume from committed pages: %+v", exchange.requests)
		}
		assertContinuousHistory(t, store, market.IntervalHour, 2500)
		if notified != 1000 || rounds != 1 {
			t.Fatalf("deep resume sent extra notifications: %d candles and %d rounds", notified, rounds)
		}
		cancel()
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		if err := <-tracked; err != nil {
			t.Fatal(err)
		}
	})
}

type historySubscriptions []closedindicator.Subscription

func (subscriptions historySubscriptions) Subscriptions(context.Context) ([]closedindicator.Subscription, error) {
	return subscriptions, nil
}

func assertHistoryShift(t *testing.T, tracker *closedindicator.Tracker, subscriptions historySubscriptions, wantKnown bool) {
	t.Helper()
	values, missing := tracker.Snapshot(subscriptions)
	if len(missing) != 0 || len(values) != 1 {
		t.Fatalf("tracker snapshot = %+v, missing = %+v", values, missing)
	}
	for _, output := range values[0].Outputs {
		if output.Name == "close" {
			if _, known := output.At(1499); known != wantKnown {
				t.Fatalf("close shift 1499 known = %v, want %v", known, wantKnown)
			}
			return
		}
	}
	t.Fatal("tracker has no close output")
}

func TestLoadHistoryRejectsMissingInitialHistory(t *testing.T) {
	instrument := market.Instrument{ID: 1, Symbol: "BTCUSDT", Active: true}
	exchange := &historyExchange{instrument: instrument}
	store := &historyStore{fakeMarketStore: fakeMarketStore{active: []market.Instrument{instrument}}}
	synchronizer := marketsync.New(exchange, store, slog.New(slog.DiscardHandler), 1, 1000, market.BinanceSpotSyncProfile(market.IntervalHour))
	load, err := synchronizer.LoadHistory(t.Context(), instrument, 2500, nil)
	if !errors.Is(err, marketsync.ErrHistoryNotReady) || load.Changed || len(exchange.requests) != 0 {
		t.Fatalf("empty LoadHistory = %+v, %v; requests = %d", load, err, len(exchange.requests))
	}
}

// A job starts in the background, rejects another one while it runs, shows
// its progress, and ends done or failed.
func TestHistoryLoaderRunsOneJobInTheBackground(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		extender := &blockingExtender{results: make(chan error)}
		loader := marketsync.NewHistoryLoader(knownInstruments{}, map[market.CandleInterval]marketsync.HistoryExtender{market.IntervalHour: extender, market.IntervalDay: extender}, slog.New(slog.DiscardHandler), marketsync.HistoryLoaderOptions{})
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error)
		go func() { done <- loader.Run(ctx) }()
		if _, ok := loader.Job(); ok {
			t.Fatal("Job() reports a job before any started")
		}

		intervals := []market.CandleInterval{market.IntervalHour, market.IntervalDay}
		started, err := loader.Start(t.Context(), []string{"btcusdt"}, intervals, 2500)
		if err != nil || started.Status != marketsync.HistoryJobRunning || !slices.Equal(started.Symbols, []string{"BTCUSDT"}) || started.Depth != 2500 {
			t.Fatalf("Start() = %+v, %v", started, err)
		}
		if _, err := loader.Start(t.Context(), []string{"ETHUSDT"}, intervals, 2500); !errors.Is(err, marketsync.ErrHistoryLoadRunning) {
			t.Fatalf("second Start() error = %v, want ErrHistoryLoadRunning", err)
		}
		// A history synchronization has not filled yet is skipped, and the
		// job goes on.
		extender.results <- fmt.Errorf("%w: BTCUSDT 1h", marketsync.ErrHistoryNotReady)
		synctest.Wait()
		if job, _ := loader.Job(); job.Status != marketsync.HistoryJobRunning || len(job.Items) != 1 || job.Items[0] != (marketsync.HistoryLoad{Symbol: "BTCUSDT", Interval: market.IntervalHour, NotReady: true}) {
			t.Fatalf("job in progress = %+v", job)
		}
		extender.results <- nil
		synctest.Wait()
		if job, _ := loader.Job(); job.Status != marketsync.HistoryJobDone || len(job.Items) != 2 || job.Items[1].NotReady || job.FinishedAt.IsZero() || job.Error != "" {
			t.Fatalf("finished job = %+v", job)
		}

		if _, err := loader.Start(t.Context(), []string{"ETHUSDT"}, intervals[:1], 2500); err != nil {
			t.Fatal(err)
		}
		extender.results <- errors.New("Binance is down")
		synctest.Wait()
		if job, _ := loader.Job(); job.Status != marketsync.HistoryJobFailed || job.Error == "" || strings.Contains(job.Error, "Binance") || len(job.Items) != 0 || !slices.Equal(job.Symbols, []string{"ETHUSDT"}) {
			t.Fatalf("failed job = %+v", job)
		}
		cancel()
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	})
}

func TestHistoryLoaderRejectsInvalidLoadsBeforeStarting(t *testing.T) {
	loader := marketsync.NewHistoryLoader(knownInstruments{}, map[market.CandleInterval]marketsync.HistoryExtender{market.IntervalHour: &blockingExtender{}}, slog.New(slog.DiscardHandler), marketsync.HistoryLoaderOptions{})
	for _, test := range []struct {
		symbols   []string
		intervals []market.CandleInterval
		depth     int
		want      error
	}{
		{symbols: []string{"BTCUSDT"}, depth: market.SyncDepth, want: marketsync.ErrInvalidHistoryLoad},
		{symbols: []string{"BTCUSDT"}, depth: market.RetentionDepth + 1, want: marketsync.ErrInvalidHistoryLoad},
		{symbols: []string{"btcusdt", "BTCUSDT"}, depth: 2500, want: marketsync.ErrInvalidHistoryLoad},
		{symbols: []string{"NOPEUSDT"}, depth: 2500, want: market.ErrInstrumentNotFound},
		{symbols: []string{"BTCUSDT"}, intervals: []market.CandleInterval{}, depth: 2500, want: marketsync.ErrInvalidHistoryLoad},
		{symbols: []string{"BTCUSDT"}, intervals: []market.CandleInterval{market.IntervalHour, market.IntervalHour}, depth: 2500, want: marketsync.ErrInvalidHistoryLoad},
		{symbols: []string{"BTCUSDT"}, intervals: []market.CandleInterval{market.IntervalDay}, depth: 2500, want: marketsync.ErrInvalidHistoryLoad},
	} {
		if test.intervals == nil {
			test.intervals = []market.CandleInterval{market.IntervalHour}
		}
		if _, err := loader.Start(t.Context(), test.symbols, test.intervals, test.depth); !errors.Is(err, test.want) {
			t.Fatalf("Start(%v, %d) error = %v, want %v", test.symbols, test.depth, err, test.want)
		}
	}
	if _, ok := loader.Job(); ok {
		t.Fatal("a rejected load left a job")
	}
}

// knownInstruments knows BTCUSDT and ETHUSDT.
type knownInstruments struct{}

func (knownInstruments) GetActiveInstrumentBySymbol(_ context.Context, symbol string) (market.Instrument, error) {
	id, ok := map[string]int64{"BTCUSDT": 1, "ETHUSDT": 2}[symbol]
	if !ok {
		return market.Instrument{}, market.ErrInstrumentNotFound
	}
	return market.Instrument{ID: id, Symbol: symbol, Active: true}, nil
}

// blockingExtender finishes each load with the next result it receives.
type blockingExtender struct{ results chan error }

func (extender *blockingExtender) LoadHistory(ctx context.Context, instrument market.Instrument, depth int, _ func([]market.Candle)) (marketsync.HistoryLoad, error) {
	select {
	case err := <-extender.results:
		if err != nil {
			return marketsync.HistoryLoad{}, err
		}
	case <-ctx.Done():
		return marketsync.HistoryLoad{}, ctx.Err()
	}
	return marketsync.HistoryLoad{Symbol: instrument.Symbol, Interval: market.IntervalHour, Count: depth}, nil
}
