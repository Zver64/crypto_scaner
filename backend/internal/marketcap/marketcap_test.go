package marketcap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"crypto-scanner/internal/market"
)

func TestResolveBatchChunksAndUsesStaleFallbackOnProviderFailure(t *testing.T) {
	now := time.Now()
	store := &fakeStore{done: true, mappings: map[string]Mapping{}, caps: map[string]Cap{}}
	instruments := make([]market.Instrument, 251)
	for i := range instruments {
		base := fmt.Sprintf("A%d", i)
		id := fmt.Sprintf("id%d", i)
		instruments[i] = market.Instrument{BaseAsset: base, QuoteAsset: "USDT"}
		store.mappings[base] = Mapping{BaseAsset: base, CoinID: id, Status: "resolved"}
		store.caps[id] = Cap{CoinID: id, USD: float64(i), Available: true, FetchedAt: now.Add(-2 * time.Hour)}
	}
	provider := &fakeProvider{marketErr: errors.New("down")}
	r := New(store, provider)
	r.now = func() time.Time { return now }
	batch, err := r.ResolveBatch(context.Background(), instruments)
	if err != nil {
		t.Fatal(err)
	}
	if provider.marketCalls != 2 || !batch.ProviderWarning {
		t.Fatalf("calls=%d warning=%v", provider.marketCalls, batch.ProviderWarning)
	}
	if batch.Facts["A1"].USD == nil || *batch.Facts["A1"].USD != 1 {
		t.Fatalf("facts=%+v", batch.Facts["A1"])
	}
}
func TestResolveBatchTreatsMissingCapAsUnresolved(t *testing.T) {
	store := &fakeStore{done: true, mappings: map[string]Mapping{"BTC": {BaseAsset: "BTC", CoinID: "bitcoin", Status: "resolved"}}, caps: map[string]Cap{}}
	r := New(store, &fakeProvider{})
	batch, err := r.ResolveBatch(context.Background(), []market.Instrument{{BaseAsset: "BTC", QuoteAsset: "USDT"}})
	if err != nil {
		t.Fatal(err)
	}
	if batch.Facts["BTC"].USD != nil || batch.Facts["BTC"].Reason != "market_cap_missing" {
		t.Fatalf("fact=%+v", batch.Facts["BTC"])
	}
}
func TestBootstrapSkipsProviderWhenAlreadyComplete(t *testing.T) {
	provider := &fakeProvider{}
	if err := New(&fakeStore{done: true, mappings: map[string]Mapping{}, caps: map[string]Cap{}}, provider).Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	if provider.tickerCalls != 0 {
		t.Fatalf("ticker calls=%d", provider.tickerCalls)
	}
}
func TestBootstrapDoesNotCompleteOnInvalidTickerEnvelope(t *testing.T) {
	store := &fakeStore{mappings: map[string]Mapping{}, caps: map[string]Cap{}}
	err := New(store, &fakeProvider{tickerErr: errors.New("invalid ticker envelope")}).Bootstrap(context.Background())
	if err == nil || store.replacements != 0 {
		t.Fatalf("err=%v replacements=%d", err, store.replacements)
	}
}
func TestBootstrapDoesNotCompleteWithAnEmptyInitialSnapshot(t *testing.T) {
	store := &fakeStore{mappings: map[string]Mapping{}, caps: map[string]Cap{}}
	err := New(store, &fakeProvider{}).Bootstrap(context.Background())
	if err == nil || store.replacements != 0 {
		t.Fatalf("err=%v replacements=%d", err, store.replacements)
	}
}
func TestBootstrapSkipsNullableCoinIDTicker(t *testing.T) {
	store := &fakeStore{mappings: map[string]Mapping{}, caps: map[string]Cap{}}
	provider := &fakeProvider{tickers: []Ticker{{Base: "NULL", Target: "USDT"}, {Base: "BTC", Target: "USDT", CoinID: "bitcoin"}}}
	if err := New(store, provider).Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.replacements != 1 || len(store.snapshot) != 1 || store.snapshot[0].CoinID != "bitcoin" {
		t.Fatalf("snapshot=%+v", store.snapshot)
	}
}
func TestResolvedMappingIsReusedAcrossQuotes(t *testing.T) {
	now := time.Now()
	store := &fakeStore{done: true, mappings: map[string]Mapping{"BTC": {BaseAsset: "BTC", QuoteAsset: "USDT", CoinID: "bitcoin", Status: "resolved"}}, caps: map[string]Cap{"bitcoin": {CoinID: "bitcoin", USD: 1, Available: true, FetchedAt: now}}}
	provider := &fakeProvider{}
	r := New(store, provider)
	r.now = func() time.Time { return now }
	_, err := r.ResolveBatch(context.Background(), []market.Instrument{{BaseAsset: "BTC", QuoteAsset: "FDUSD"}})
	if err != nil {
		t.Fatal(err)
	}
	if provider.tickerCalls != 0 {
		t.Fatalf("ticker calls=%d", provider.tickerCalls)
	}
}
func TestMappingProviderFailureOnlyUnresolvesMissingBases(t *testing.T) {
	now := time.Now()
	store := &fakeStore{done: true, mappings: map[string]Mapping{"BTC": {BaseAsset: "BTC", CoinID: "bitcoin", Status: "resolved"}}, caps: map[string]Cap{"bitcoin": {CoinID: "bitcoin", USD: 10, Available: true, FetchedAt: now}}}
	provider := &fakeProvider{tickerErr: errors.New("ticker unavailable")}
	r := New(store, provider)
	r.now = func() time.Time { return now }
	batch, err := r.ResolveBatch(context.Background(), []market.Instrument{{BaseAsset: "BTC", QuoteAsset: "USDT"}, {BaseAsset: "NEW", QuoteAsset: "USDT"}})
	if err != nil || !batch.ProviderWarning {
		t.Fatalf("batch=%+v err=%v", batch, err)
	}
	if batch.Facts["BTC"].USD == nil || *batch.Facts["BTC"].USD != 10 {
		t.Fatalf("known fact=%+v", batch.Facts["BTC"])
	}
	if batch.Facts["NEW"].Reason != "mapping_provider_unavailable" {
		t.Fatalf("missing fact=%+v", batch.Facts["NEW"])
	}
}
func TestAllMappingsDoesNotHideConflictBehindUSDTPreference(t *testing.T) {
	r := New(&fakeStore{}, &fakeProvider{tickers: []Ticker{{Base: "BTC", Target: "FDUSD", CoinID: "first"}, {Base: "BTC", Target: "USDT", CoinID: "second"}}})
	mappings, err := r.allMappings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(mappings) != 1 || mappings[0].Reason != "mapping_conflict" {
		t.Fatalf("mappings=%+v", mappings)
	}
}
func TestClientMarketsSendsCompletePaginationAndRejectsPartialNullableResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("per_page") != "250" || q.Get("page") != "1" || q.Get("sparkline") != "false" {
			t.Errorf("query=%s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`[{"id":"id0","market_cap":null,"last_updated":"2026-01-01T00:00:00Z"}]`))
	}))
	defer server.Close()
	ids := make([]string, 250)
	for i := range ids {
		ids[i] = fmt.Sprintf("id%d", i)
	}
	values, err := NewClient(server.URL, "secret").Markets(context.Background(), ids)
	if err != nil || len(values) != 250 || values[0].Available {
		t.Fatalf("values=%d first=%+v err=%v", len(values), values[0], err)
	}
}
func TestClientMarketsRejectsNullResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`null`))
	}))
	defer server.Close()
	if _, err := NewClient(server.URL, "").Markets(context.Background(), []string{"bitcoin"}); err == nil {
		t.Fatal("Markets() accepted a null response")
	}
}
func TestConcurrentStaleRefreshIsDeduplicated(t *testing.T) {
	now := time.Now()
	store := &fakeStore{done: true, mappings: map[string]Mapping{"BTC": {BaseAsset: "BTC", CoinID: "bitcoin", Status: "resolved"}}, caps: map[string]Cap{"bitcoin": {CoinID: "bitcoin", USD: 1, Available: true, FetchedAt: now.Add(-2 * time.Hour)}}}
	provider := &fakeProvider{marketValues: []Cap{{CoinID: "bitcoin", USD: 2, Available: true, ObservedAt: now}}, marketDelay: 20 * time.Millisecond}
	r := New(store, provider)
	r.now = func() time.Time { return now }
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := r.ResolveBatch(context.Background(), []market.Instrument{{BaseAsset: "BTC"}}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if provider.marketCalls != 1 {
		t.Fatalf("market calls=%d", provider.marketCalls)
	}
}
func TestMissingCapIsCooledPerIDWithoutGlobalWarning(t *testing.T) {
	now := time.Now()
	store := &fakeStore{done: true, mappings: map[string]Mapping{"NULL": {BaseAsset: "NULL", CoinID: "null", Status: "resolved"}, "OK": {BaseAsset: "OK", CoinID: "ok", Status: "resolved"}, "OTHER": {BaseAsset: "OTHER", CoinID: "other", Status: "resolved"}}, caps: map[string]Cap{}}
	provider := &fakeProvider{marketValues: []Cap{{CoinID: "null"}, {CoinID: "ok", USD: 5, Available: true, ObservedAt: now}}}
	r := New(store, provider)
	r.now = func() time.Time { return now }
	items := []market.Instrument{{BaseAsset: "NULL"}, {BaseAsset: "OK"}}
	batch, err := r.ResolveBatch(context.Background(), items)
	if err != nil || batch.ProviderWarning || batch.Facts["NULL"].Reason != "market_cap_missing" {
		t.Fatalf("batch=%+v err=%v", batch, err)
	}
	_, err = r.ResolveBatch(context.Background(), []market.Instrument{{BaseAsset: "NULL"}, {BaseAsset: "OTHER"}})
	if err != nil || provider.marketCalls != 2 {
		t.Fatalf("calls=%d err=%v", provider.marketCalls, err)
	}
}

type fakeStore struct {
	done         bool
	replacements int
	snapshot     []Mapping
	mappings     map[string]Mapping
	caps         map[string]Cap
}

func (s *fakeStore) BootstrapCompleted(context.Context) (bool, error) { return s.done, nil }
func (s *fakeStore) ReplaceSnapshot(_ context.Context, mappings []Mapping) error {
	s.replacements++
	s.snapshot = append([]Mapping(nil), mappings...)
	return nil
}
func (s *fakeStore) GetMapping(_ context.Context, b string) (Mapping, error) {
	m, ok := s.mappings[b]
	if !ok {
		return Mapping{}, errors.New("missing")
	}
	return m, nil
}
func (s *fakeStore) SaveMapping(_ context.Context, m Mapping) error {
	s.mappings[m.BaseAsset] = m
	return nil
}
func (s *fakeStore) GetCap(_ context.Context, id string) (Cap, error) {
	c, ok := s.caps[id]
	if !ok {
		return Cap{}, errors.New("missing")
	}
	return c, nil
}
func (s *fakeStore) SaveCap(_ context.Context, c Cap) error { s.caps[c.CoinID] = c; return nil }

type fakeProvider struct {
	mu                       sync.Mutex
	marketCalls, tickerCalls int
	marketErr                error
	tickerErr                error
	tickers                  []Ticker
	marketValues             []Cap
	marketDelay              time.Duration
}

func (p *fakeProvider) Tickers(context.Context, int) ([]Ticker, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tickerCalls++
	return p.tickers, p.tickerErr
}
func (p *fakeProvider) Markets(_ context.Context, _ []string) ([]Cap, error) {
	p.mu.Lock()
	p.marketCalls++
	delay, values, err := p.marketDelay, p.marketValues, p.marketErr
	p.mu.Unlock()
	if delay > 0 {
		time.Sleep(delay)
	}
	return values, err
}
