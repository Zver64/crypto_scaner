package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/indicator"
	indicatortalib "crypto-scanner/internal/indicator/talib"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/market/kline"
	marketlive "crypto-scanner/internal/market/live"
)

var chartTestStart = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// chartHistoryStub serves 30 closed candles per interval. A gated interval
// blocks its reads until the gate closes.
type chartHistoryStub struct {
	mu      sync.Mutex
	lists   map[market.CandleInterval]int
	gates   map[market.CandleInterval]chan struct{}
	entered chan market.CandleInterval
	// ignoreCancel keeps a gated read waiting for its gate when canceled, and
	// fail then fails it, like a read that outlives its subscription.
	ignoreCancel bool
	fail         bool
}

func newChartHistoryStub() *chartHistoryStub {
	return &chartHistoryStub{lists: map[market.CandleInterval]int{}, gates: map[market.CandleInterval]chan struct{}{}, entered: make(chan market.CandleInterval, 16)}
}

func (store *chartHistoryStub) GetActiveInstrumentBySymbol(_ context.Context, symbol string) (market.Instrument, error) {
	return market.Instrument{ID: 1, Symbol: symbol, Active: true}, nil
}

func (store *chartHistoryStub) ListCandlePage(ctx context.Context, _ int64, interval market.CandleInterval, _ *time.Time, limit int) (market.CandlePage, error) {
	store.mu.Lock()
	store.lists[interval]++
	gate := store.gates[interval]
	store.mu.Unlock()
	store.entered <- interval
	switch {
	case gate != nil && store.ignoreCancel:
		<-gate
	case gate != nil:
		select {
		case <-gate:
		case <-ctx.Done():
			return market.CandlePage{}, ctx.Err()
		}
	}
	if store.fail {
		return market.CandlePage{}, errors.New("storage unavailable")
	}
	candles := chartTestCandles(interval, 30)
	return market.CandlePage{Candles: candles[max(0, len(candles)-limit):]}, nil
}

func (store *chartHistoryStub) listCalls(interval market.CandleInterval) int {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.lists[interval]
}

func (store *chartHistoryStub) gate(interval market.CandleInterval) chan struct{} {
	gate := make(chan struct{})
	store.mu.Lock()
	store.gates[interval] = gate
	store.mu.Unlock()
	return gate
}

func TestChartClientBuildsANewSubscriptionOnce(t *testing.T) {
	store := newChartHistoryStub()
	client, socket := newTestChartClient(t, store)
	key := kline.Key{Symbol: "BTCUSDT", Interval: market.IntervalHour}

	client.setRange(context.Background(), key, chart.DefaultRange, nil)
	client.Enqueue(marketlive.Message{Kind: marketlive.KindSubscribed, Key: key})
	client.Enqueue(marketlive.Message{Kind: marketlive.KindSnapshot, Key: key, Freshness: marketlive.FreshnessWaiting})

	if message := receiveChartMessage(t, socket); message.Type != Subscribed {
		t.Fatalf("first message = %s, want subscribed", message.Type)
	}
	snapshot := receiveChartMessage(t, socket)
	if snapshot.Type != Snapshot || *snapshot.Version != 1 || len(snapshot.Chart.Candles) != 30 {
		t.Fatalf("second message = %s version %v, want the first snapshot of 30 candles", snapshot.Type, snapshot.Version)
	}
	expectNoChartMessage(t, socket)
	if calls := store.listCalls(market.IntervalHour); calls != 1 {
		t.Fatalf("history reads = %d, want 1", calls)
	}
}

func TestChartClientMergesEventsQueuedDuringABuild(t *testing.T) {
	store := newChartHistoryStub()
	gate := store.gate(market.IntervalHour)
	client, socket := newTestChartClient(t, store)
	key := kline.Key{Symbol: "BTCUSDT", Interval: market.IntervalHour}
	selections := []indicator.Selection{{Type: indicatortalib.RSIType, Parameters: indicator.Parameters{"period": 14}}}

	client.setRange(context.Background(), key, chart.DefaultRange, selections)
	client.Enqueue(marketlive.Message{Kind: marketlive.KindSnapshot, Key: key})
	<-store.entered
	forming := chartTestCandles(market.IntervalHour, 31)[30]
	for index := range 10 {
		trade := forming
		trade.Close += float64(index)
		client.Enqueue(marketlive.Message{Kind: marketlive.KindUpdate, Key: key, Candle: &marketlive.CandleState{Candle: trade}, Freshness: marketlive.FreshnessFresh})
	}
	client.Enqueue(marketlive.Message{Kind: marketlive.KindUpdate, Key: key, Candle: &marketlive.CandleState{Candle: forming, Final: true}, Freshness: marketlive.FreshnessFresh})
	close(gate)

	if first := receiveChartMessage(t, socket); first.Type != Snapshot || *first.Version != 1 {
		t.Fatalf("first frame = %s version %v, want snapshot 1", first.Type, first.Version)
	}
	second := receiveChartMessage(t, socket)
	if second.Type != Snapshot || *second.Version != 2 || len(second.Chart.Candles) != 31 {
		t.Fatalf("second frame = %s version %v, want one snapshot with the final candle", second.Type, second.Version)
	}
	expectNoChartMessage(t, socket)
}

func TestChartClientSendsNothingAfterASubscriptionEnds(t *testing.T) {
	for _, fail := range []bool{false, true} {
		store := newChartHistoryStub()
		store.ignoreCancel, store.fail = true, fail
		gate := store.gate(market.IntervalHour)
		client, socket := newTestChartClient(t, store)
		key := kline.Key{Symbol: "BTCUSDT", Interval: market.IntervalHour}

		client.setRange(context.Background(), key, chart.DefaultRange, nil)
		client.Enqueue(marketlive.Message{Kind: marketlive.KindSnapshot, Key: key})
		<-store.entered
		client.forget(key)
		close(gate)
		// The live service may still publish to recipients it captured earlier.
		client.Enqueue(marketlive.Message{Kind: marketlive.KindStatus, Key: key, Freshness: marketlive.FreshnessFresh})

		expectNoChartMessage(t, socket)
	}
}

func TestChartClientBuildsKeysIndependently(t *testing.T) {
	store := newChartHistoryStub()
	gate := store.gate(market.IntervalHour)
	t.Cleanup(func() { close(gate) })
	client, socket := newTestChartClient(t, store)
	hour := kline.Key{Symbol: "BTCUSDT", Interval: market.IntervalHour}
	day := kline.Key{Symbol: "BTCUSDT", Interval: market.IntervalDay}

	client.setRange(context.Background(), hour, chart.DefaultRange, nil)
	client.setRange(context.Background(), day, chart.DefaultRange, nil)
	client.Enqueue(marketlive.Message{Kind: marketlive.KindSnapshot, Key: hour})
	client.Enqueue(marketlive.Message{Kind: marketlive.KindSnapshot, Key: day})

	message := receiveChartMessage(t, socket)
	if message.Type != Snapshot || *message.Interval != CandleInterval(market.IntervalDay) {
		t.Fatalf("frame = %s for %v, want the day snapshot while the hour build is blocked", message.Type, message.Interval)
	}
}

func newTestChartClient(t *testing.T, store chart.Store) (*chartClient, *liveSocketClient) {
	t.Helper()
	registry, err := indicator.NewRegistry(indicatortalib.New()...)
	if err != nil {
		t.Fatal(err)
	}
	service, err := chart.NewService(store, registry, emptyChartCatalog{}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	// The socket has no connection; the test never lets its queue overflow.
	socket := &liveSocketClient{id: "test", queue: make(chan LiveCandleServerMessage, 64), done: make(chan struct{})}
	client := newChartClient(socket, service, slog.New(slog.DiscardHandler))
	// The socket has no connection to close, so only the streams stop.
	t.Cleanup(func() { client.stop(); client.Wait() })
	return client, socket
}

type emptyChartCatalog struct{}

func (emptyChartCatalog) ChartCatalog(market.CandleInterval) []chart.CatalogIndicator { return nil }

func receiveChartMessage(t *testing.T, socket *liveSocketClient) LiveCandleServerMessage {
	t.Helper()
	select {
	case message := <-socket.queue:
		return message
	case <-time.After(2 * time.Second):
		t.Fatal("no chart message")
		return LiveCandleServerMessage{}
	}
}

func expectNoChartMessage(t *testing.T, socket *liveSocketClient) {
	t.Helper()
	select {
	case message := <-socket.queue:
		t.Fatalf("unexpected %s message", message.Type)
	case <-time.After(100 * time.Millisecond):
	}
}

func chartTestCandles(interval market.CandleInterval, count int) []market.Candle {
	candles := make([]market.Candle, count)
	open := chartTestStart
	for index := range candles {
		next := interval.NextOpenTime(open)
		price := 100 + float64(index%7)
		candles[index] = market.Candle{OpenTime: open, CloseTime: next.Add(-time.Millisecond), Open: price, High: price + 2, Low: price - 2, Close: price + 1}
		open = next
	}
	return candles
}
