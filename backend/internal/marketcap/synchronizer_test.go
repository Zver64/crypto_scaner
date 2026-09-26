package marketcap

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"crypto-scanner/internal/market"
)

func TestSynchronizerRefreshesPersistedFactsAndRecordsStatus(t *testing.T) {
	store := &fakeStore{done: true, mappings: map[string]Mapping{"BTC": {BaseAsset: "BTC", CoinID: "bitcoin", Status: "resolved"}}, caps: map[string]Cap{}}
	provider := &fakeProvider{marketValues: []Cap{{CoinID: "bitcoin", USD: 100, Available: true, ObservedAt: time.Now()}}}
	source := &syncSource{instruments: []market.Instrument{{BaseAsset: "BTC", QuoteAsset: "USDT"}}}
	synchronizer, err := NewCoinMetadataSynchronizer(New(store, provider), source, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Hour, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := synchronizer.runRefresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	states := source.snapshot()
	if provider.marketCalls != 1 || states[0].Status != market.SyncStatusRunning || states[1].Status != market.SyncStatusSucceeded || states[1].LastSucceededAt == nil {
		t.Fatalf("calls=%d states=%+v", provider.marketCalls, states)
	}
	if cap, ok := store.caps["bitcoin"]; !ok || cap.USD != 100 {
		t.Fatalf("persisted cap=%+v", cap)
	}
	if len(store.stablecoinIDs) != 1 || store.stablecoinIDs[0] != "tether" {
		t.Fatalf("stablecoin IDs=%v", store.stablecoinIDs)
	}
}

func TestSynchronizerFirstFailureHasNoSuccessfulTimestamp(t *testing.T) {
	store := &fakeStore{done: true, mappings: map[string]Mapping{"BTC": {BaseAsset: "BTC", CoinID: "bitcoin", Status: "resolved"}}, caps: map[string]Cap{}}
	source := &syncSource{instruments: []market.Instrument{{BaseAsset: "BTC", QuoteAsset: "USDT"}}}
	synchronizer, _ := NewCoinMetadataSynchronizer(New(store, &fakeProvider{marketErr: io.ErrUnexpectedEOF}), source, slog.New(slog.DiscardHandler), time.Hour, time.Minute)
	if err := synchronizer.runRefresh(context.Background()); err == nil {
		t.Fatal("provider failure unexpectedly succeeded")
	}
	states := source.snapshot()
	failed := states[len(states)-1]
	if failed.Status != market.SyncStatusFailed || failed.LastSucceededAt != nil {
		t.Fatalf("failed state=%+v", failed)
	}
}

func TestSynchronizerFailurePreservesPriorSuccessfulTimestampAndCap(t *testing.T) {
	store := &fakeStore{done: true, mappings: map[string]Mapping{"BTC": {BaseAsset: "BTC", CoinID: "bitcoin", Status: "resolved"}}, caps: map[string]Cap{}}
	provider := &fakeProvider{marketValues: []Cap{{CoinID: "bitcoin", USD: 100, Available: true, ObservedAt: time.Now()}}}
	source := &syncSource{instruments: []market.Instrument{{BaseAsset: "BTC", QuoteAsset: "USDT"}}}
	resolver := New(store, provider)
	synchronizer, _ := NewCoinMetadataSynchronizer(resolver, source, slog.New(slog.DiscardHandler), time.Hour, time.Minute)
	if err := synchronizer.runRefresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	prior := source.snapshot()[1].LastSucceededAt
	store.caps["bitcoin"] = Cap{CoinID: "bitcoin", USD: 100, Available: true, FetchedAt: time.Now().Add(-2 * time.Hour)}
	provider.marketErr = io.ErrUnexpectedEOF
	if err := synchronizer.runRefresh(context.Background()); err == nil {
		t.Fatal("provider failure unexpectedly succeeded")
	}
	failed := source.snapshot()[3]
	if failed.Status != market.SyncStatusFailed || failed.LastSucceededAt == nil || !failed.LastSucceededAt.Equal(*prior) || store.caps["bitcoin"].USD != 100 {
		t.Fatalf("failed state=%+v cap=%+v", failed, store.caps["bitcoin"])
	}
}

func TestSynchronizerRetriesObservableBootstrapThenRefreshes(t *testing.T) {
	store := &fakeStore{mappings: map[string]Mapping{}, caps: map[string]Cap{}}
	provider := &bootstrapRetryProvider{firstErr: errors.New("temporary bootstrap failure"), refreshed: make(chan struct{})}
	source := &syncSource{instruments: []market.Instrument{{BaseAsset: "BTC", QuoteAsset: "USDT"}}}
	synchronizer, _ := NewCoinMetadataSynchronizer(New(store, provider), source, slog.New(slog.DiscardHandler), time.Hour, time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- synchronizer.Run(ctx) }()
	select {
	case <-provider.refreshed:
	case <-time.After(time.Second):
		t.Fatal("bootstrap was not retried and followed by refresh")
	}
	cancel()
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	states := source.snapshot()
	if len(states) < 6 || states[0].Status != market.SyncStatusRunning || states[1].Status != market.SyncStatusFailed || states[2].Status != market.SyncStatusRunning || states[3].Status != market.SyncStatusSucceeded || states[4].Status != market.SyncStatusRunning {
		t.Fatalf("bootstrap/refresh states=%+v", states)
	}
}

func TestSynchronizerRetriesEmptyCatalogBeforeRefreshInterval(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := &fakeStore{done: true, mappings: map[string]Mapping{"BTC": {BaseAsset: "BTC", CoinID: "bitcoin", Status: "resolved"}}, caps: map[string]Cap{}}
		provider := &catalogProvider{refreshed: make(chan struct{})}
		source := newDelayedCatalogSource()
		const retryDelay = 40 * time.Millisecond
		synchronizer, _ := NewCoinMetadataSynchronizer(New(store, provider), source, slog.New(slog.DiscardHandler), time.Hour, retryDelay)
		ctx, cancel := context.WithCancel(context.Background())
		result := make(chan error, 1)
		go func() { result <- synchronizer.Run(ctx) }()

		// The failed-state signal is emitted only after ListActiveInstruments has
		// returned its first empty snapshot. Its save is held until this test has
		// verified that no provider call or second catalog read occurred.
		<-source.firstEmptyFailure
		if provider.calls() != 0 || source.listCount() != 1 {
			t.Fatalf("provider calls=%d catalog reads=%d before retry release", provider.calls(), source.listCount())
		}
		source.setInstruments([]market.Instrument{{BaseAsset: "BTC", QuoteAsset: "USDT"}})
		close(source.releaseFirstFailure)

		<-provider.refreshed
		listTimes := source.snapshotListTimes()
		if len(listTimes) != 2 || listTimes[1].Sub(listTimes[0]) < retryDelay {
			t.Fatalf("catalog read times=%v, retry delay=%s", listTimes, retryDelay)
		}
		cancel()
		if err := <-result; err != nil {
			t.Fatal(err)
		}
		states := source.state.snapshot()
		if len(states) < 6 || states[3].Status != market.SyncStatusFailed || states[3].LastSucceededAt == nil || states[5].Status != market.SyncStatusSucceeded {
			t.Fatalf("empty catalog retry states=%+v", states)
		}
	})
}

func TestSynchronizerCancellationResultPersistenceIsBounded(t *testing.T) {
	store := &fakeStore{mappings: map[string]Mapping{}, caps: map[string]Cap{}}
	provider := &blockingBootstrapProvider{started: make(chan struct{})}
	source := &blockingStateSource{resultSaveStarted: make(chan struct{})}
	synchronizer, _ := NewCoinMetadataSynchronizer(New(store, provider), source, slog.New(slog.DiscardHandler), time.Hour, time.Hour)
	synchronizer.stateSaveTimeout = 20 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- synchronizer.Run(ctx) }()
	<-provider.started
	started := time.Now()
	cancel()
	<-source.resultSaveStarted
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
		if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
			t.Fatalf("shutdown persistence exceeded bound: %s", elapsed)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("synchronizer shutdown blocked on status persistence")
	}
}

func TestSynchronizerCancellationDuringBootstrapIsRecorded(t *testing.T) {
	store := &fakeStore{mappings: map[string]Mapping{}, caps: map[string]Cap{}}
	provider := &blockingBootstrapProvider{started: make(chan struct{})}
	source := &syncSource{}
	synchronizer, _ := NewCoinMetadataSynchronizer(New(store, provider), source, slog.New(slog.DiscardHandler), time.Hour, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- synchronizer.Run(ctx) }()
	<-provider.started
	cancel()
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	states := source.snapshot()
	if len(states) != 2 || states[0].Status != market.SyncStatusRunning || states[1].Status != market.SyncStatusFailed || states[1].LastSucceededAt != nil {
		t.Fatalf("cancellation states=%+v", states)
	}
}

type delayedCatalogSource struct {
	mu                  sync.Mutex
	instruments         []market.Instrument
	listTimes           []time.Time
	firstEmptyFailure   chan struct{}
	releaseFirstFailure chan struct{}
	failureOnce         sync.Once
	state               *syncSource
}

func newDelayedCatalogSource() *delayedCatalogSource {
	return &delayedCatalogSource{
		firstEmptyFailure:   make(chan struct{}),
		releaseFirstFailure: make(chan struct{}),
		state:               &syncSource{},
	}
}
func (s *delayedCatalogSource) ListActiveInstruments(context.Context) ([]market.Instrument, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listTimes = append(s.listTimes, time.Now())
	return append([]market.Instrument(nil), s.instruments...), nil
}
func (s *delayedCatalogSource) setInstruments(instruments []market.Instrument) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.instruments = append([]market.Instrument(nil), instruments...)
}
func (s *delayedCatalogSource) SaveSyncState(ctx context.Context, state market.SyncState) error {
	if err := s.state.SaveSyncState(ctx, state); err != nil {
		return err
	}
	if state.Status == market.SyncStatusFailed {
		s.failureOnce.Do(func() {
			close(s.firstEmptyFailure)
			<-s.releaseFirstFailure
		})
	}
	return nil
}
func (s *delayedCatalogSource) listCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.listTimes)
}
func (s *delayedCatalogSource) snapshotListTimes() []time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]time.Time(nil), s.listTimes...)
}

type blockingStateSource struct {
	mu                sync.Mutex
	calls             int
	resultSaveStarted chan struct{}
}

func (*blockingStateSource) ListActiveInstruments(context.Context) ([]market.Instrument, error) {
	return nil, nil
}
func (s *blockingStateSource) SaveSyncState(ctx context.Context, _ market.SyncState) error {
	s.mu.Lock()
	s.calls++
	call := s.calls
	s.mu.Unlock()
	if call == 1 {
		return nil
	}
	if call == 2 {
		close(s.resultSaveStarted)
	}
	<-ctx.Done()
	return ctx.Err()
}

type syncSource struct {
	mu          sync.Mutex
	instruments []market.Instrument
	states      []market.SyncState
	lastSuccess *time.Time
}

func (s *syncSource) ListActiveInstruments(context.Context) ([]market.Instrument, error) {
	return s.instruments, nil
}
func (s *syncSource) SaveSyncState(_ context.Context, state market.SyncState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Match PostgreSQL's COALESCE semantics for failed/running attempts.
	if state.LastSucceededAt == nil {
		state.LastSucceededAt = s.lastSuccess
	} else {
		value := *state.LastSucceededAt
		s.lastSuccess = &value
	}
	s.states = append(s.states, state)
	return nil
}
func (s *syncSource) snapshot() []market.SyncState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]market.SyncState(nil), s.states...)
}

type bootstrapRetryProvider struct {
	mu        sync.Mutex
	calls     int
	firstErr  error
	refreshed chan struct{}
	once      sync.Once
}

func (p *bootstrapRetryProvider) Tickers(context.Context, int) ([]Ticker, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	if p.calls == 1 {
		return nil, p.firstErr
	}
	return []Ticker{{Base: "BTC", Target: "USDT", CoinID: "bitcoin"}}, nil
}
func (*bootstrapRetryProvider) StablecoinIDs(context.Context) ([]string, error) {
	return []string{"tether"}, nil
}
func (p *bootstrapRetryProvider) Markets(context.Context, []string) ([]Cap, error) {
	p.once.Do(func() { close(p.refreshed) })
	return []Cap{{CoinID: "bitcoin", USD: 100, Available: true, ObservedAt: time.Now()}}, nil
}

type catalogProvider struct {
	mu          sync.Mutex
	marketCalls int
	refreshed   chan struct{}
	once        sync.Once
}

func (*catalogProvider) Tickers(context.Context, int) ([]Ticker, error) {
	return nil, errors.New("unexpected ticker call")
}
func (*catalogProvider) StablecoinIDs(context.Context) ([]string, error) {
	return []string{"tether"}, nil
}
func (p *catalogProvider) Markets(context.Context, []string) ([]Cap, error) {
	p.mu.Lock()
	p.marketCalls++
	p.mu.Unlock()
	p.once.Do(func() { close(p.refreshed) })
	return []Cap{{CoinID: "bitcoin", USD: 100, Available: true, ObservedAt: time.Now()}}, nil
}
func (p *catalogProvider) calls() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.marketCalls
}

type blockingBootstrapProvider struct {
	started chan struct{}
	once    sync.Once
}

func (p *blockingBootstrapProvider) Tickers(ctx context.Context, _ int) ([]Ticker, error) {
	p.once.Do(func() { close(p.started) })
	<-ctx.Done()
	return nil, ctx.Err()
}
func (*blockingBootstrapProvider) StablecoinIDs(context.Context) ([]string, error) {
	return nil, errors.New("unexpected stablecoin call")
}
func (*blockingBootstrapProvider) Markets(context.Context, []string) ([]Cap, error) {
	return nil, errors.New("unexpected markets call")
}
