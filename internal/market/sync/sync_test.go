package sync_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	"crypto-scanner/internal/market"
	marketsync "crypto-scanner/internal/market/sync"
)

func TestSynchronizerAppliesCompleteSnapshotAndRecordsSuccess(t *testing.T) {
	previousSuccess := time.Date(2026, time.August, 4, 0, 1, 0, 0, time.UTC)
	previousClosed := time.Date(2026, time.August, 3, 0, 0, 0, 0, time.UTC)
	profile := marketsync.MVPProfile()
	store := &fakeMarketStore{state: market.SyncState{
		Profile: profile, Status: market.SyncStatusRunning,
		LastSucceededAt: &previousSuccess, LastClosedOpenTime: &previousClosed,
	}}
	wantSnapshot := []market.Instrument{
		{Symbol: "BTCUSDT", BaseAsset: "BTC", QuoteAsset: "USDT", Status: "TRADING", Active: true},
		{Symbol: "ETHUSDT", BaseAsset: "ETH", QuoteAsset: "USDT", Status: "BREAK", Active: false},
	}
	synchronizer := marketsync.New(&fakeExchange{items: wantSnapshot}, store)

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
		state:  market.SyncState{Profile: marketsync.MVPProfile(), Status: market.SyncStatusNeverRun},
		active: []market.Instrument{instrument}, latest: map[int64][]market.Candle{},
	}
	var logs bytes.Buffer

	if err := marketsync.NewWithLogger(exchange, store, slog.New(slog.NewJSONHandler(&logs, nil))).Sync(context.Background()); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if len(exchange.candleRequests) != 1 {
		t.Fatalf("candle requests = %#v, want one", exchange.candleRequests)
	}
	request := exchange.candleRequests[0]
	if request.Symbol != "BTCUSDT" || request.Interval != "1d" || request.Limit != 30 || request.ClosedBefore.IsZero() {
		t.Fatalf("candle request = %#v, want latest 30 daily candles at synchronization cutoff", request)
	}
	if len(store.upserted) != 1 || len(store.upserted[0]) != 1 {
		t.Fatalf("upserted batches = %#v, want one candle", store.upserted)
	}
	got := store.upserted[0][0]
	if got.InstrumentID != instrument.ID || got.OpenTime != closed.OpenTime || !got.CloseTime.Before(request.ClosedBefore) {
		t.Fatalf("upserted candle = %#v, want instrument identity and closed history", got)
	}
	if len(store.saved) != 2 || store.saved[1].Status != market.SyncStatusSucceeded || store.saved[1].LastClosedOpenTime == nil || !store.saved[1].LastClosedOpenTime.Equal(closed.OpenTime) {
		t.Fatalf("saved states = %#v, want successful candle progress", store.saved)
	}
	for _, field := range []string{`"outcome":"succeeded"`, `"instruments_total":1`, `"instruments_succeeded":1`, `"instruments_failed":0`, `"candle_rows_written":1`} {
		if !strings.Contains(logs.String(), field) {
			t.Fatalf("structured log %s missing %s", logs.String(), field)
		}
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
		state:  market.SyncState{Profile: marketsync.MVPProfile(), Status: market.SyncStatusNeverRun},
		active: []market.Instrument{btc, eth}, latest: map[int64][]market.Candle{},
	}
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))

	err := marketsync.NewWithLogger(exchange, store, logger).Sync(context.Background())
	if !errors.Is(err, permanentErr) {
		t.Fatalf("Sync() error = %v, want permanent instrument failure", err)
	}
	if len(exchange.candleRequests) != 2 || exchange.candleRequests[0].Symbol != "BTCUSDT" || exchange.candleRequests[1].Symbol != "ETHUSDT" {
		t.Fatalf("candle requests = %#v, want both instruments attempted", exchange.candleRequests)
	}
	if len(store.upserted) != 1 || len(store.upserted[0]) != 1 || store.upserted[0][0].InstrumentID != eth.ID {
		t.Fatalf("upserted batches = %#v, want successful ETH history retained", store.upserted)
	}
	if len(store.saved) != 2 || store.saved[1].Status != market.SyncStatusFailed || store.saved[1].ErrorMessage == "" {
		t.Fatalf("saved states = %#v, want running then failed", store.saved)
	}
	for _, field := range []string{`"outcome":"failed"`, `"instruments_total":2`, `"instruments_succeeded":1`, `"instruments_failed":1`, `"candle_rows_written":1`} {
		if !strings.Contains(logs.String(), field) {
			t.Fatalf("structured log %s missing %s", logs.String(), field)
		}
	}
}

func TestSynchronizerRecordsDiscoveryFailureWithoutApplyingSnapshot(t *testing.T) {
	previousSuccess := time.Date(2026, time.August, 4, 0, 1, 0, 0, time.UTC)
	store := &fakeMarketStore{state: market.SyncState{
		Profile: marketsync.MVPProfile(), Status: market.SyncStatusSucceeded, LastSucceededAt: &previousSuccess,
	}}
	discoveryErr := errors.New("exchange unavailable")
	synchronizer := marketsync.New(&fakeExchange{err: discoveryErr}, store)

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
	store := &fakeMarketStore{state: market.SyncState{Profile: marketsync.MVPProfile(), Status: market.SyncStatusNeverRun}}
	synchronizer := marketsync.New(&fakeExchange{items: []market.Instrument{}}, store)

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
		state: market.SyncState{Profile: marketsync.MVPProfile(), Status: market.SyncStatusNeverRun}, applyErr: applyErr,
	}
	synchronizer := marketsync.New(&fakeExchange{items: items}, store)

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
			Profile: marketsync.MVPProfile(), Status: market.SyncStatusSucceeded, LastSucceededAt: &previousSuccess,
		},
		saveErrors: []error{nil, successSaveErr, nil},
	}
	items := []market.Instrument{{Symbol: "BTCUSDT", BaseAsset: "BTC", QuoteAsset: "USDT", Status: "TRADING", Active: true}}

	err := marketsync.New(&fakeExchange{items: items}, store).Sync(context.Background())
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
		state:      market.SyncState{Profile: marketsync.MVPProfile(), Status: market.SyncStatusNeverRun},
		saveErrors: []error{nil, successSaveErr, failureSaveErr},
	}
	items := []market.Instrument{{Symbol: "BTCUSDT", BaseAsset: "BTC", QuoteAsset: "USDT", Status: "TRADING", Active: true}}

	err := marketsync.New(&fakeExchange{items: items}, store).Sync(context.Background())
	if !errors.Is(err, successSaveErr) || !errors.Is(err, failureSaveErr) {
		t.Fatalf("Sync() error = %v, want both persistence failures", err)
	}
	if len(store.saved) != 3 || store.saved[2].Status != market.SyncStatusFailed {
		t.Fatalf("saved state attempts = %#v, want final failed attempt", store.saved)
	}
}

type fakeExchange struct {
	items          []market.Instrument
	err            error
	candles        map[string][]market.Candle
	candleErrors   map[string]error
	candleRequests []market.CandleRequest
}

func (exchange *fakeExchange) ListInstruments(context.Context) ([]market.Instrument, error) {
	return exchange.items, exchange.err
}

func (exchange *fakeExchange) ListClosedCandles(_ context.Context, request market.CandleRequest) ([]market.Candle, error) {
	exchange.candleRequests = append(exchange.candleRequests, request)
	return exchange.candles[request.Symbol], exchange.candleErrors[request.Symbol]
}

type fakeMarketStore struct {
	state      market.SyncState
	saved      []market.SyncState
	applied    []market.Instrument
	applyErr   error
	active     []market.Instrument
	latest     map[int64][]market.Candle
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

func (store *fakeMarketStore) ListLatestCandles(_ context.Context, instrumentID int64, _ int) ([]market.Candle, error) {
	return store.latest[instrumentID], nil
}

func (store *fakeMarketStore) UpsertCandles(_ context.Context, items []market.Candle) error {
	store.upserted = append(store.upserted, append([]market.Candle(nil), items...))
	if len(items) == 0 {
		return nil
	}
	return store.upsertErrs[items[0].InstrumentID]
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
