package marketsync_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"crypto-scanner/internal/market"
	"crypto-scanner/internal/market/marketsync"
)

func TestSynchronizerAppliesCompleteSnapshotAndRecordsSuccess(t *testing.T) {
	previousSuccess := time.Date(2026, time.August, 4, 0, 1, 0, 0, time.UTC)
	previousClosed := time.Date(2026, time.August, 3, 0, 0, 0, 0, time.UTC)
	profile := market.BinanceSpotSyncProfile(market.IntervalDay)
	store := &fakeMarketStore{state: market.SyncState{
		Profile: profile, Status: market.SyncStatusRunning,
		LastSucceededAt: &previousSuccess, LastClosedOpenTime: &previousClosed,
	}}
	wantSnapshot := []market.Instrument{
		{Symbol: "BTCUSDT", BaseAsset: "BTC", QuoteAsset: "USDT", Status: "TRADING", Active: true},
		{Symbol: "ETHUSDT", BaseAsset: "ETH", QuoteAsset: "USDT", Status: "BREAK", Active: false},
	}
	synchronizer := marketsync.New(&fakeExchange{items: wantSnapshot}, store, slog.New(slog.DiscardHandler), 4, 1000, market.BinanceSpotSyncProfile(market.IntervalDay))

	if err := synchronizer.Sync(context.Background()); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if !reflect.DeepEqual(store.applied, wantSnapshot) {
		t.Fatalf("applied snapshot = %#v, want %#v", store.applied, wantSnapshot)
	}
	if len(store.saved) != 2 {
		t.Fatalf("saved states = %#v, want running then succeeded", store.saved)
	}
	running, succeeded := store.saved[0], store.saved[1]
	if running.Status != market.SyncStatusRunning || running.LastStartedAt == nil || running.LastSucceededAt == nil || !running.LastSucceededAt.Equal(previousSuccess) {
		t.Fatalf("running state = %#v", running)
	}
	if succeeded.Status != market.SyncStatusSucceeded || succeeded.LastSucceededAt == nil || succeeded.LastSucceededAt.Before(*running.LastStartedAt) || succeeded.ErrorMessage != "" {
		t.Fatalf("succeeded state = %#v", succeeded)
	}
	if succeeded.LastClosedOpenTime == nil || !succeeded.LastClosedOpenTime.Equal(previousClosed) {
		t.Fatalf("success discarded candle progress: %#v", succeeded)
	}
}

func TestSynchronizerBackfillsLatestClosedCandlesForInstrumentWithoutHistory(t *testing.T) {
	instrument := market.Instrument{ID: 41, Symbol: "BTCUSDT", BaseAsset: "BTC", QuoteAsset: "USDT", Status: "TRADING", Active: true}
	closed := market.Candle{
		Interval: "1d", OpenTime: time.Date(2026, time.August, 3, 0, 0, 0, 0, time.UTC),
		CloseTime: time.Date(2026, time.August, 3, 23, 59, 59, 999000000, time.UTC), Open: 100, High: 110, Low: 90, Close: 105,
	}
	forming := closed
	forming.OpenTime = time.Date(2099, time.January, 1, 0, 0, 0, 0, time.UTC)
	forming.CloseTime = time.Date(2099, time.January, 1, 23, 59, 59, 999000000, time.UTC)
	exchange := &fakeExchange{items: []market.Instrument{instrument}, candles: map[string][]market.Candle{"BTCUSDT": {closed, forming}}}
	store := &fakeMarketStore{
		state:  market.SyncState{Profile: market.BinanceSpotSyncProfile(market.IntervalDay), Status: market.SyncStatusNeverRun},
		active: []market.Instrument{instrument}, latest: map[int64][]market.Candle{},
	}
	var logs bytes.Buffer

	if err := marketsync.New(exchange, store, slog.New(slog.NewJSONHandler(&logs, nil)), 4, 1000, market.BinanceSpotSyncProfile(market.IntervalDay)).Sync(context.Background()); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if len(exchange.candleRequests) < 1 {
		t.Fatal("no candle request was made")
	}
	request := exchange.candleRequests[0]
	if request.Symbol != "BTCUSDT" || request.Interval != "1d" || request.Limit != 1000 || request.ClosedBefore.IsZero() {
		t.Fatalf("candle request = %#v, want latest 1000 daily candles at synchronization cutoff", request)
	}
	written := flattenedCandles(store.upserted)
	if len(written) != 1 {
		t.Fatalf("upserted batches = %#v, want one unique closed candle", store.upserted)
	}
	got := written[0]
	if got.InstrumentID != instrument.ID || got.OpenTime != closed.OpenTime || !got.CloseTime.Before(request.ClosedBefore) {
		t.Fatalf("upserted candle = %#v, want instrument identity and closed history", got)
	}
	if len(store.saved) != 2 || store.saved[1].Status != market.SyncStatusSucceeded || store.saved[1].LastClosedOpenTime == nil || !store.saved[1].LastClosedOpenTime.Equal(closed.OpenTime) {
		t.Fatalf("saved states = %#v, want successful candle progress", store.saved)
	}
	for _, field := range []string{`"outcome":"succeeded"`, `"instruments_total":1`, `"instruments_succeeded":1`, `"instruments_failed":0`, `"exchange_requests":2`, `"candle_rows_requested":4`, `"candle_rows_written":1`, `"gap_ranges_repaired":0`, `"lag_intervals":`, `"retry_count":0`} {
		if !strings.Contains(logs.String(), field) {
			t.Fatalf("structured log %s missing %s", logs.String(), field)
		}
	}
}

func TestSynchronizerUsesPolicyForInitialRequests(t *testing.T) {
	instrument := market.Instrument{ID: 41, Symbol: "BTCUSDT", QuoteAsset: "USDT", Status: "TRADING", Active: true}
	tests := []struct {
		name    string
		profile market.SyncProfile
	}{
		{name: "daily", profile: market.BinanceSpotSyncProfile(market.IntervalDay)},
		{name: "hourly", profile: market.BinanceSpotSyncProfile(market.IntervalHour)},
		{name: "weekly", profile: market.BinanceSpotSyncProfile(market.IntervalWeek)},
		{name: "monthly", profile: market.BinanceSpotSyncProfile(market.IntervalMonth)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			exchange := &fakeExchange{items: []market.Instrument{instrument}, candles: map[string][]market.Candle{}}
			store := &fakeMarketStore{
				state:  market.SyncState{Profile: test.profile, Status: market.SyncStatusNeverRun},
				active: []market.Instrument{instrument}, latest: map[int64][]market.Candle{},
			}

			if err := marketsync.New(exchange, store, slog.New(slog.DiscardHandler), 1, 1000, test.profile).Sync(context.Background()); err != nil {
				t.Fatalf("Sync() error = %v", err)
			}
			if len(exchange.candleRequests) != 1 {
				t.Fatalf("candle requests = %#v, want one", exchange.candleRequests)
			}
			request := exchange.candleRequests[0]
			if request.Limit != 1000 {
				t.Fatalf("request limit = %d, want 1000", request.Limit)
			}
			if request.AfterOpenTime != nil {
				t.Fatalf("initial request = %#v, want no after-open-time", request)
			}
		})
	}
}

func TestSynchronizerRepairsDepthWhenLatestClosedIntervalIsStored(t *testing.T) {
	instrument := market.Instrument{ID: 41, Symbol: "BTCUSDT", Active: true}
	latest := market.Candle{InstrumentID: instrument.ID, Interval: market.IntervalHour, OpenTime: market.IntervalHour.LastClosedOpenTime(time.Now())}
	exchange := &fakeExchange{items: []market.Instrument{instrument}}
	store := &fakeMarketStore{
		state:  market.SyncState{Profile: market.BinanceSpotSyncProfile(market.IntervalHour), Status: market.SyncStatusSucceeded},
		active: []market.Instrument{instrument}, latest: map[int64][]market.Candle{instrument.ID: {latest}},
	}
	if err := marketsync.New(exchange, store, slog.New(slog.DiscardHandler), 1, 1000, market.BinanceSpotSyncProfile(market.IntervalHour)).Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(exchange.candleRequests) != 2 || exchange.candleRequests[0].AfterOpenTime == nil ||
		!exchange.candleRequests[0].AfterOpenTime.Equal(market.BinanceSpotSyncProfile(market.IntervalHour).Interval.PreviousOpenTime(latest.OpenTime)) ||
		!exchange.candleRequests[1].HistoryRepair || exchange.candleRequests[1].AfterOpenTime != nil {
		t.Fatalf("candle requests = %#v, want latest-close recheck and backward depth repair", exchange.candleRequests)
	}
}

func TestSynchronizerPersistsCorrectedLatestClose(t *testing.T) {
	instrument := market.Instrument{ID: 41, Symbol: "BTCUSDT", Active: true}
	open := market.IntervalDay.LastClosedOpenTime(time.Now())
	stored := market.Candle{InstrumentID: instrument.ID, Interval: market.IntervalDay, OpenTime: open, CloseTime: market.IntervalDay.NextOpenTime(open).Add(-time.Millisecond), Close: 10}
	corrected := stored
	corrected.Close = 12
	exchange := &fakeExchange{items: []market.Instrument{instrument}, candles: map[string][]market.Candle{instrument.Symbol: {corrected}}}
	store := &fakeMarketStore{active: []market.Instrument{instrument}, latest: map[int64][]market.Candle{instrument.ID: {stored}}}
	if err := marketsync.New(exchange, store, slog.New(slog.DiscardHandler), 4, 1000, market.BinanceSpotSyncProfile(market.IntervalDay)).Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.latest[instrument.ID]) == 0 || store.latest[instrument.ID][0].Close != 12 {
		t.Fatalf("corrected close not persisted: %#v", store.latest[instrument.ID])
	}
}

func TestSynchronizerRechecksLatestStoredOpenTime(t *testing.T) {
	instrument := market.Instrument{ID: 41, Symbol: "BTCUSDT", QuoteAsset: "USDT", Status: "TRADING", Active: true}
	latest := market.Candle{InstrumentID: instrument.ID, Interval: "1d", OpenTime: time.Date(2026, time.August, 2, 0, 0, 0, 0, time.UTC)}
	missing := market.Candle{
		OpenTime:  time.Date(2026, time.August, 3, 0, 0, 0, 0, time.UTC),
		CloseTime: time.Date(2026, time.August, 3, 23, 59, 59, 999000000, time.UTC),
		Open:      100, High: 110, Low: 90, Close: 105,
	}
	exchange := &fakeExchange{items: []market.Instrument{instrument}, candles: map[string][]market.Candle{instrument.Symbol: {missing}}}
	store := &fakeMarketStore{
		state:  market.SyncState{Profile: market.BinanceSpotSyncProfile(market.IntervalDay), Status: market.SyncStatusSucceeded},
		active: []market.Instrument{instrument}, latest: map[int64][]market.Candle{instrument.ID: {latest}},
	}

	if err := marketsync.New(exchange, store, slog.New(slog.DiscardHandler), 4, 1000, market.BinanceSpotSyncProfile(market.IntervalDay)).Sync(context.Background()); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if len(exchange.candleRequests) != 2 {
		t.Fatalf("candle requests = %#v, want incremental and depth-repair requests", exchange.candleRequests)
	}
	request := exchange.candleRequests[0]
	if request.AfterOpenTime == nil || !request.AfterOpenTime.Equal(market.BinanceSpotSyncProfile(market.IntervalDay).Interval.PreviousOpenTime(latest.OpenTime)) || request.Limit != 1000 {
		t.Fatalf("incremental request = %#v, want overlap of latest close %s with page limit 1000", request, latest.OpenTime)
	}
	written := flattenedCandles(store.upserted)
	if len(written) != 1 || written[0].OpenTime != missing.OpenTime {
		t.Fatalf("upserted = %#v, want only the missing candle written", store.upserted)
	}
}

func TestSynchronizerRejectsOverlappingRunWithoutWaiting(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	exchange := &fakeExchange{items: []market.Instrument{{Symbol: "BTCUSDT", QuoteAsset: "USDT", Status: "TRADING", Active: true}}, started: started, release: release}
	synchronizer := marketsync.New(exchange, &fakeMarketStore{state: market.SyncState{Profile: market.BinanceSpotSyncProfile(market.IntervalDay)}}, slog.New(slog.DiscardHandler), 4, 1000, market.BinanceSpotSyncProfile(market.IntervalDay))
	firstResult := make(chan error, 1)
	go func() { firstResult <- synchronizer.Sync(context.Background()) }()
	<-started

	if err := synchronizer.Sync(context.Background()); !errors.Is(err, marketsync.ErrSyncInProgress) {
		t.Fatalf("overlapping Sync() error = %v, want ErrSyncInProgress", err)
	}
	close(release)
	if err := <-firstResult; err != nil {
		t.Fatalf("first Sync() error = %v", err)
	}
}

func TestSynchronizerBoundsInstrumentConcurrency(t *testing.T) {
	instruments := make([]market.Instrument, 5)
	for index := range instruments {
		instruments[index] = market.Instrument{ID: int64(index + 1), Symbol: string(rune('A'+index)) + "USDT", QuoteAsset: "USDT", Status: "TRADING", Active: true}
	}
	started := make(chan struct{}, len(instruments))
	release := make(chan struct{})
	exchange := &fakeExchange{items: instruments, candles: map[string][]market.Candle{}, workerStarted: started, workerRelease: release}
	store := &fakeMarketStore{state: market.SyncState{Profile: market.BinanceSpotSyncProfile(market.IntervalDay)}, active: instruments, latest: map[int64][]market.Candle{}}
	result := make(chan error, 1)
	go func() {
		result <- marketsync.New(exchange, store, slog.New(slog.DiscardHandler), 2, 1000, market.BinanceSpotSyncProfile(market.IntervalDay)).Sync(context.Background())
	}()

	<-started
	<-started
	select {
	case <-started:
		t.Fatal("more than two instrument requests ran concurrently")
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	if err := <-result; err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if exchange.maxActive != 2 {
		t.Fatalf("maximum active requests = %d, want 2", exchange.maxActive)
	}
}

func TestSynchronizerContinuesAfterInstrumentFailureAndReportsRunTotals(t *testing.T) {
	btc := market.Instrument{ID: 41, Symbol: "BTCUSDT", BaseAsset: "BTC", QuoteAsset: "USDT", Status: "TRADING", Active: true}
	eth := market.Instrument{ID: 42, Symbol: "ETHUSDT", BaseAsset: "ETH", QuoteAsset: "USDT", Status: "TRADING", Active: true}
	closed := market.Candle{
		Interval: "1d", OpenTime: time.Date(2026, time.August, 3, 0, 0, 0, 0, time.UTC),
		CloseTime: time.Date(2026, time.August, 3, 23, 59, 59, 999000000, time.UTC), Open: 100, High: 110, Low: 90, Close: 105,
	}
	permanentErr := errors.New("invalid symbol")
	exchange := &fakeExchange{
		items: []market.Instrument{btc, eth}, candles: map[string][]market.Candle{"ETHUSDT": {closed}},
		candleErrors: map[string]error{"BTCUSDT": permanentErr},
	}
	store := &fakeMarketStore{
		state:  market.SyncState{Profile: market.BinanceSpotSyncProfile(market.IntervalDay), Status: market.SyncStatusNeverRun},
		active: []market.Instrument{btc, eth}, latest: map[int64][]market.Candle{},
	}
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))

	synchronizer := marketsync.New(exchange, store, logger, 4, 1000, market.BinanceSpotSyncProfile(market.IntervalDay))
	err := synchronizer.Sync(context.Background())
	if !errors.Is(err, permanentErr) || !errors.Is(err, marketsync.ErrInstrumentsFailed) {
		t.Fatalf("Sync() error = %v, want partial instrument failure", err)
	}
	requested := map[string]bool{}
	for _, request := range exchange.candleRequests {
		requested[request.Symbol] = true
	}
	if !requested["BTCUSDT"] || !requested["ETHUSDT"] {
		t.Fatalf("candle requests = %#v, want both instruments attempted", exchange.candleRequests)
	}
	written := flattenedCandles(store.upserted)
	if len(written) == 0 || written[0].InstrumentID != eth.ID {
		t.Fatalf("upserted batches = %#v, want successful ETH history retained", store.upserted)
	}
	if len(store.saved) != 2 || store.saved[1].Status != market.SyncStatusSucceeded || store.saved[1].LastSucceededAt == nil || store.saved[1].ErrorMessage == "" {
		t.Fatalf("saved states = %#v, want running then succeeded with the instrument failure", store.saved)
	}
	for _, field := range []string{`"outcome":"partial"`, `"instruments_total":2`, `"instruments_succeeded":1`, `"instruments_failed":1`, `"candle_rows_written":1`} {
		if !strings.Contains(logs.String(), field) {
			t.Fatalf("structured log %s missing %s", logs.String(), field)
		}
	}

	// Before another candle closes, a retry synchronizes only the failed instrument.
	exchange.candleRequests = nil
	if err := synchronizer.Sync(context.Background()); !errors.Is(err, permanentErr) {
		t.Fatalf("retry Sync() error = %v, want the instrument failure again", err)
	}
	for _, request := range exchange.candleRequests {
		if request.Symbol != "BTCUSDT" {
			t.Fatalf("retry requested %s, want only the failed instrument", request.Symbol)
		}
	}
	if len(exchange.candleRequests) == 0 {
		t.Fatal("retry did not request the failed instrument")
	}
}

func TestSynchronizerRecordsDiscoveryFailureWithoutApplyingSnapshot(t *testing.T) {
	previousSuccess := time.Date(2026, time.August, 4, 0, 1, 0, 0, time.UTC)
	store := &fakeMarketStore{state: market.SyncState{
		Profile: market.BinanceSpotSyncProfile(market.IntervalDay), Status: market.SyncStatusSucceeded, LastSucceededAt: &previousSuccess,
	}}
	discoveryErr := errors.New("exchange unavailable")
	synchronizer := marketsync.New(&fakeExchange{err: discoveryErr}, store, slog.New(slog.DiscardHandler), 4, 1000, market.BinanceSpotSyncProfile(market.IntervalDay))

	err := synchronizer.Sync(context.Background())
	if !errors.Is(err, discoveryErr) {
		t.Fatalf("Sync() error = %v, want discovery failure", err)
	}
	if store.applied != nil {
		t.Fatalf("failed discovery applied snapshot %#v", store.applied)
	}
	if len(store.saved) != 2 || store.saved[0].Status != market.SyncStatusRunning || store.saved[1].Status != market.SyncStatusFailed {
		t.Fatalf("saved states = %#v, want running then failed", store.saved)
	}
	failed := store.saved[1]
	if failed.LastSucceededAt == nil || !failed.LastSucceededAt.Equal(previousSuccess) || failed.ErrorMessage == "" {
		t.Fatalf("failed state lost useful success metadata: %#v", failed)
	}
}

func TestSynchronizerRejectsEmptyDiscoveryWithoutDeactivatingCatalog(t *testing.T) {
	store := &fakeMarketStore{state: market.SyncState{Profile: market.BinanceSpotSyncProfile(market.IntervalDay), Status: market.SyncStatusNeverRun}}
	synchronizer := marketsync.New(&fakeExchange{items: []market.Instrument{}}, store, slog.New(slog.DiscardHandler), 4, 1000, market.BinanceSpotSyncProfile(market.IntervalDay))

	if err := synchronizer.Sync(context.Background()); err == nil {
		t.Fatal("Sync() accepted an empty discovery snapshot")
	}
	if store.applied != nil {
		t.Fatalf("empty discovery applied snapshot %#v", store.applied)
	}
	if len(store.saved) != 2 || store.saved[1].Status != market.SyncStatusFailed {
		t.Fatalf("saved states = %#v, want running then failed", store.saved)
	}
}

func TestSynchronizerRecordsTransactionalApplyFailure(t *testing.T) {
	applyErr := errors.New("transaction rolled back")
	items := []market.Instrument{{Symbol: "BTCUSDT", BaseAsset: "BTC", QuoteAsset: "USDT", Status: "TRADING", Active: true}}
	store := &fakeMarketStore{
		state: market.SyncState{Profile: market.BinanceSpotSyncProfile(market.IntervalDay), Status: market.SyncStatusNeverRun}, applyErr: applyErr,
	}
	synchronizer := marketsync.New(&fakeExchange{items: items}, store, slog.New(slog.DiscardHandler), 4, 1000, market.BinanceSpotSyncProfile(market.IntervalDay))

	err := synchronizer.Sync(context.Background())
	if !errors.Is(err, applyErr) {
		t.Fatalf("Sync() error = %v, want apply failure", err)
	}
	if !reflect.DeepEqual(store.applied, items) {
		t.Fatalf("attempted snapshot = %#v, want %#v", store.applied, items)
	}
	if len(store.saved) != 2 || store.saved[1].Status != market.SyncStatusFailed {
		t.Fatalf("saved states = %#v, want running then failed", store.saved)
	}
}

func TestSynchronizerRecordsFailureWhenSuccessfulOutcomeCannotBeSaved(t *testing.T) {
	previousSuccess := time.Date(2026, time.August, 3, 0, 1, 0, 0, time.UTC)
	successSaveErr := errors.New("save succeeded outcome")
	store := &fakeMarketStore{
		state: market.SyncState{
			Profile: market.BinanceSpotSyncProfile(market.IntervalDay), Status: market.SyncStatusSucceeded, LastSucceededAt: &previousSuccess,
		},
		saveErrors: []error{nil, successSaveErr, nil},
	}
	items := []market.Instrument{{Symbol: "BTCUSDT", BaseAsset: "BTC", QuoteAsset: "USDT", Status: "TRADING", Active: true}}

	err := marketsync.New(&fakeExchange{items: items}, store, slog.New(slog.DiscardHandler), 4, 1000, market.BinanceSpotSyncProfile(market.IntervalDay)).Sync(context.Background())
	if !errors.Is(err, successSaveErr) {
		t.Fatalf("Sync() error = %v, want successful-outcome persistence failure", err)
	}
	if len(store.saved) != 3 || store.saved[0].Status != market.SyncStatusRunning || store.saved[1].Status != market.SyncStatusSucceeded || store.saved[2].Status != market.SyncStatusFailed {
		t.Fatalf("saved state attempts = %#v, want running, succeeded, failed", store.saved)
	}
	failed := store.saved[2]
	if failed.LastSucceededAt == nil || !failed.LastSucceededAt.Equal(previousSuccess) || failed.ErrorMessage == "" {
		t.Fatalf("failed state did not preserve prior success: %#v", failed)
	}
}

func TestSynchronizerReturnsBothOutcomePersistenceFailures(t *testing.T) {
	successSaveErr := errors.New("save succeeded outcome")
	failureSaveErr := errors.New("save failed outcome")
	store := &fakeMarketStore{
		state:      market.SyncState{Profile: market.BinanceSpotSyncProfile(market.IntervalDay), Status: market.SyncStatusNeverRun},
		saveErrors: []error{nil, successSaveErr, failureSaveErr},
	}
	items := []market.Instrument{{Symbol: "BTCUSDT", BaseAsset: "BTC", QuoteAsset: "USDT", Status: "TRADING", Active: true}}

	err := marketsync.New(&fakeExchange{items: items}, store, slog.New(slog.DiscardHandler), 4, 1000, market.BinanceSpotSyncProfile(market.IntervalDay)).Sync(context.Background())
	if !errors.Is(err, successSaveErr) || !errors.Is(err, failureSaveErr) {
		t.Fatalf("Sync() error = %v, want both persistence failures", err)
	}
	if len(store.saved) != 3 || store.saved[2].Status != market.SyncStatusFailed {
		t.Fatalf("saved state attempts = %#v, want final failed attempt", store.saved)
	}
}

func flattenedCandles(batches [][]market.Candle) []market.Candle {
	var result []market.Candle
	for _, batch := range batches {
		result = append(result, batch...)
	}
	return result
}

type fakeExchange struct {
	mu             sync.Mutex
	items          []market.Instrument
	err            error
	candles        map[string][]market.Candle
	candleErrors   map[string]error
	candleRequests []market.CandleRequest
	started        chan struct{}
	release        chan struct{}
	workerStarted  chan struct{}
	workerRelease  chan struct{}
	active         int
	maxActive      int
}

func (exchange *fakeExchange) ListInstruments(ctx context.Context) ([]market.Instrument, error) {
	if exchange.started != nil {
		close(exchange.started)
		select {
		case <-exchange.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return exchange.items, exchange.err
}

func (*fakeExchange) RetryCount() uint64 { return 0 }

func (exchange *fakeExchange) ListClosedCandles(_ context.Context, request market.CandleRequest) ([]market.Candle, error) {
	exchange.mu.Lock()
	exchange.candleRequests = append(exchange.candleRequests, request)
	if exchange.workerStarted != nil {
		exchange.active++
		exchange.maxActive = max(exchange.maxActive, exchange.active)
		exchange.workerStarted <- struct{}{}
	}
	candles, err := exchange.candles[request.Symbol], exchange.candleErrors[request.Symbol]
	exchange.mu.Unlock()
	if exchange.workerRelease != nil {
		<-exchange.workerRelease
		exchange.mu.Lock()
		exchange.active--
		exchange.mu.Unlock()
	}
	return candles, err
}

type fakeMarketStore struct {
	mu         sync.Mutex
	state      market.SyncState
	saved      []market.SyncState
	applied    []market.Instrument
	applyErr   error
	active     []market.Instrument
	latest     map[int64][]market.Candle
	coverage   map[string]market.HistoryCoverage
	emptyGaps  map[string]marketsync.HistoryGap
	upserted   [][]market.Candle
	upsertErrs map[int64]error
	saveErrors []error
	saveCalls  int
}

func (store *fakeMarketStore) ApplyInstrumentSnapshot(_ context.Context, items []market.Instrument) error {
	store.applied = append([]market.Instrument(nil), items...)
	return store.applyErr
}

func (store *fakeMarketStore) ListActiveInstruments(context.Context) ([]market.Instrument, error) {
	return store.active, nil
}

func (store *fakeMarketStore) ListLatestCandles(_ context.Context, instrumentIDs []int64, interval market.CandleInterval, limit int) (map[int64][]market.Candle, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	result := make(map[int64][]market.Candle, len(instrumentIDs))
	for _, instrumentID := range instrumentIDs {
		var candles []market.Candle
		for _, candle := range store.latest[instrumentID] {
			if candle.Interval == interval {
				candles = append(candles, candle)
			}
		}
		candles = candles[:min(len(candles), limit)]
		slices.Reverse(candles) // store.latest is newest first
		result[instrumentID] = append([]market.Candle(nil), candles...)
	}
	return result, nil
}

func (store *fakeMarketStore) SummarizeCandleHistory(ctx context.Context, instrumentIDs []int64, interval market.CandleInterval, limit int) (map[int64]marketsync.HistorySummary, error) {
	latest, err := store.ListLatestCandles(ctx, instrumentIDs, interval, limit)
	return store.summarizeLatest(latest, interval), err
}

// summarizeLatest builds history summaries from chronological latest candles.
func (store *fakeMarketStore) summarizeLatest(latest map[int64][]market.Candle, interval market.CandleInterval) map[int64]marketsync.HistorySummary {
	store.mu.Lock()
	defer store.mu.Unlock()
	histories := map[int64]marketsync.HistorySummary{}
	for id, candles := range latest {
		if len(candles) == 0 {
			continue
		}
		history := marketsync.HistorySummary{Count: len(candles), Oldest: candles[0].OpenTime.UTC(), Latest: candles[len(candles)-1].OpenTime.UTC()}
		for index := 1; index < len(candles); index++ {
			if next := interval.NextOpenTime(candles[index-1].OpenTime); next.Before(candles[index].OpenTime) {
				gap := marketsync.HistoryGap{From: next, To: candles[index].OpenTime.UTC()}
				if empty, found := store.emptyGaps[emptyGapKey(id, interval, gap.From)]; found && empty.To.Equal(gap.To) {
					gap.RetryAfter = empty.RetryAfter
				}
				history.Gaps = append(history.Gaps, gap)
			}
		}
		histories[id] = history
	}
	return histories
}

func (store *fakeMarketStore) UpsertCandlesWithChanges(ctx context.Context, items []market.Candle) ([]market.Candle, error) {
	store.mu.Lock()
	existing := make(map[int64][]market.Candle, len(store.latest))
	for id, candles := range store.latest {
		existing[id] = append([]market.Candle(nil), candles...)
	}
	store.mu.Unlock()
	if err := store.UpsertCandles(ctx, items); err != nil {
		return nil, err
	}
	changed := make([]market.Candle, 0, len(items))
	for _, item := range items {
		found := false
		for _, old := range existing[item.InstrumentID] {
			if old.Interval == item.Interval && old.OpenTime.Equal(item.OpenTime) {
				found = true
				if !reflect.DeepEqual(old, item) {
					changed = append(changed, item)
				}
				break
			}
		}
		if !found {
			changed = append(changed, item)
		}
	}
	return changed, nil
}

func (store *fakeMarketStore) UpsertCandles(_ context.Context, items []market.Candle) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.upserted = append(store.upserted, append([]market.Candle(nil), items...))
	if len(items) == 0 {
		return nil
	}
	if err := store.upsertErrs[items[0].InstrumentID]; err != nil {
		return err
	}
	if store.latest == nil {
		store.latest = map[int64][]market.Candle{}
	}
	for _, item := range items {
		found := false
		for index := range store.latest[item.InstrumentID] {
			if store.latest[item.InstrumentID][index].Interval == item.Interval && store.latest[item.InstrumentID][index].OpenTime.Equal(item.OpenTime) {
				store.latest[item.InstrumentID][index] = item
				found = true
				break
			}
		}
		if !found {
			store.latest[item.InstrumentID] = append(store.latest[item.InstrumentID], item)
		}
	}
	for id := range store.latest {
		sort.Slice(store.latest[id], func(i, j int) bool { return store.latest[id][i].OpenTime.After(store.latest[id][j].OpenTime) })
	}
	return nil
}

func (store *fakeMarketStore) ListCandleHistoryCoverage(_ context.Context, instrumentIDs []int64, interval market.CandleInterval) (map[int64]market.HistoryCoverage, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	coverages := map[int64]market.HistoryCoverage{}
	for _, id := range instrumentIDs {
		if coverage, found := store.coverage[fmt.Sprintf("%d:%s", id, interval)]; found {
			coverages[id] = coverage
		}
	}
	return coverages, nil
}

func (store *fakeMarketStore) SaveEmptyCandleGap(_ context.Context, instrumentID int64, interval market.CandleInterval, gap marketsync.HistoryGap) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.emptyGaps == nil {
		store.emptyGaps = map[string]marketsync.HistoryGap{}
	}
	store.emptyGaps[emptyGapKey(instrumentID, interval, gap.From)] = gap
	return nil
}

func emptyGapKey(instrumentID int64, interval market.CandleInterval, from time.Time) string {
	return fmt.Sprintf("%d:%s:%d", instrumentID, interval, from.UnixMilli())
}

func (store *fakeMarketStore) SaveCandleHistoryCoverage(_ context.Context, coverage market.HistoryCoverage) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.coverage == nil {
		store.coverage = map[string]market.HistoryCoverage{}
	}
	store.coverage[fmt.Sprintf("%d:%s", coverage.InstrumentID, coverage.Interval)] = coverage
	return nil
}

func (store *fakeMarketStore) GetSyncState(context.Context, market.SyncProfile) (market.SyncState, error) {
	return store.state, nil
}

func (store *fakeMarketStore) SaveSyncState(_ context.Context, state market.SyncState) error {
	store.saved = append(store.saved, state)
	var err error
	if store.saveCalls < len(store.saveErrors) {
		err = store.saveErrors[store.saveCalls]
	}
	store.saveCalls++
	if err != nil {
		return err
	}
	store.state = state
	return nil
}
