package live

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"crypto-scanner/internal/market"
	"crypto-scanner/internal/market/kline"

	"golang.org/x/time/rate"
)

type upstreamStub struct {
	events       chan kline.Event
	statuses     chan kline.Status
	mu           sync.Mutex
	subscribes   map[kline.Key]int
	unsubscribes map[kline.Key]int
}

func newUpstreamStub() *upstreamStub {
	return &upstreamStub{events: make(chan kline.Event, 8), statuses: make(chan kline.Status, 8), subscribes: make(map[kline.Key]int), unsubscribes: make(map[kline.Key]int)}
}
func (stub *upstreamStub) Subscribe(key kline.Key) error {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	stub.subscribes[key]++
	return nil
}
func (stub *upstreamStub) Unsubscribe(key kline.Key) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	stub.unsubscribes[key]++
}
func (stub *upstreamStub) Events() <-chan kline.Event    { return stub.events }
func (stub *upstreamStub) Statuses() <-chan kline.Status { return stub.statuses }
func (stub *upstreamStub) counts(key kline.Key) (int, int) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	return stub.subscribes[key], stub.unsubscribes[key]
}

type historyStub struct {
	mu      sync.Mutex
	candles []market.Candle
}

func (*historyStub) GetActiveInstrumentBySymbol(_ context.Context, symbol string) (market.Instrument, error) {
	return market.Instrument{ID: 1, Symbol: symbol, Active: true}, nil
}
func (stub *historyStub) ListCandlePage(_ context.Context, _ int64, _ market.CandleInterval, before *time.Time, limit int) (market.CandlePage, error) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	eligible := make([]market.Candle, 0, len(stub.candles))
	for _, candle := range stub.candles {
		if before == nil || candle.OpenTime.Before(*before) {
			eligible = append(eligible, candle)
		}
	}
	hasMore := len(eligible) > limit
	if hasMore {
		eligible = eligible[len(eligible)-limit:]
	}
	return market.CandlePage{Candles: eligible, HasMore: hasMore}, nil
}

type clientStub struct {
	id       string
	messages chan Message
	closed   chan struct{}
	once     sync.Once
}

func (client *clientStub) ID() string { return client.id }
func (client *clientStub) Close() {
	if client.closed != nil {
		client.once.Do(func() { close(client.closed) })
	}
}
func (client *clientStub) Enqueue(message Message) bool {
	select {
	case client.messages <- message:
		return true
	default:
		return false
	}
}

func TestShutdownClosesRegisteredClientWithoutSubscriptions(t *testing.T) {
	service := New(newUpstreamStub(), &historyStub{}, slog.New(slog.NewTextHandler(io.Discard, nil)), Options{})
	client := &clientStub{id: "idle", messages: make(chan Message, 1), closed: make(chan struct{})}
	service.RegisterClient(client)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = service.Run(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-client.closed:
	case <-time.After(time.Second):
		t.Fatal("registered client was not closed during shutdown")
	}
	<-done
}

func TestServiceSharesUpstreamAndCancelsDelayedRelease(t *testing.T) {
	upstream := newUpstreamStub()
	service := New(upstream, &historyStub{}, slog.New(slog.NewTextHandler(io.Discard, nil)), Options{ReleaseDelay: 25 * time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = service.Run(ctx) }()
	key := kline.Key{Symbol: "BTCUSDT", Interval: market.IntervalHour}
	first := &clientStub{id: "first", messages: make(chan Message, 16)}
	second := &clientStub{id: "second", messages: make(chan Message, 16)}
	if err := service.Subscribe(ctx, first, key.Symbol, key.Interval); err != nil {
		t.Fatal(err)
	}
	if err := service.Subscribe(ctx, first, key.Symbol, key.Interval); err != nil {
		t.Fatal(err)
	}
	if err := service.Subscribe(ctx, second, key.Symbol, key.Interval); err != nil {
		t.Fatal(err)
	}
	if subscribed, _ := upstream.counts(key); subscribed != 1 {
		t.Fatalf("upstream subscriptions = %d, want 1", subscribed)
	}
	service.Unsubscribe(first.ID(), key)
	time.Sleep(35 * time.Millisecond)
	if _, unsubscribed := upstream.counts(key); unsubscribed != 0 {
		t.Fatalf("unsubscribed while second client remains")
	}
	service.Unsubscribe(second.ID(), key)
	time.Sleep(10 * time.Millisecond)
	if err := service.Subscribe(ctx, first, key.Symbol, key.Interval); err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)
	if _, unsubscribed := upstream.counts(key); unsubscribed != 0 {
		t.Fatalf("stale timer removed returned subscription")
	}
	service.Unsubscribe(first.ID(), key)
	time.Sleep(40 * time.Millisecond)
	if _, unsubscribed := upstream.counts(key); unsubscribed != 1 {
		t.Fatalf("upstream unsubscriptions = %d, want 1", unsubscribed)
	}
}

func TestServiceDoesNotRollFinalCandleBack(t *testing.T) {
	upstream := newUpstreamStub()
	service := New(upstream, &historyStub{}, slog.New(slog.NewTextHandler(io.Discard, nil)), Options{ReleaseDelay: time.Second})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = service.Run(ctx) }()
	client := &clientStub{id: "client", messages: make(chan Message, 16)}
	key := kline.Key{Symbol: "BTCUSDT", Interval: market.IntervalHour}
	if err := service.Subscribe(ctx, client, key.Symbol, key.Interval); err != nil {
		t.Fatal(err)
	}
	open := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	closed := kline.Event{Key: key, Candle: market.Candle{Interval: key.Interval, OpenTime: open, CloseTime: open.Add(time.Hour - time.Millisecond), Open: 10, High: 12, Low: 9, Close: 11}, Final: true}
	upstream.events <- closed
	closed.Final = false
	closed.Candle.Close = 9
	upstream.events <- closed
	deadline := time.After(time.Second)
	for {
		select {
		case message := <-client.messages:
			if message.Kind == KindUpdate && message.Candle != nil {
				if !message.Candle.Final || message.Candle.Candle.Close != 11 {
					t.Fatalf("final candle rolled back: %+v", message.Candle)
				}
				time.Sleep(20 * time.Millisecond)
				select {
				case extra := <-client.messages:
					if extra.Kind == KindUpdate {
						t.Fatalf("received delayed non-final update")
					}
				default:
				}
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for update")
		}
	}
}

func TestHistoryCorrectionRefreshesActiveGraph(t *testing.T) {
	service := New(newUpstreamStub(), &historyStub{}, slog.New(slog.NewTextHandler(io.Discard, nil)), Options{})
	client := &clientStub{id: "graph", messages: make(chan Message, 16)}
	ctx := context.Background()
	if err := service.Subscribe(ctx, client, "BTCUSDT", market.IntervalDay); err != nil {
		t.Fatal(err)
	}
	open := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	key := kline.Key{Symbol: "BTCUSDT", Interval: market.IntervalDay}
	service.apply(ctx, kline.Event{Key: key, Candle: market.Candle{InstrumentID: 1, Interval: market.IntervalDay, OpenTime: open, Close: 10}, Final: true})
	for len(client.messages) > 0 {
		<-client.messages
	}
	service.HistoryChanged([]market.Candle{{InstrumentID: 1, Interval: market.IntervalDay, OpenTime: open, Close: 12}})
	// Exchange and server clocks need not agree: a final packet with a future
	// event time cannot undo a REST-confirmed candle either.
	service.apply(ctx, kline.Event{Key: key, Candle: market.Candle{InstrumentID: 1, Interval: market.IntervalDay, OpenTime: open, Close: 10}, Final: true, EventTime: time.Now().Add(time.Hour)})
	// A REST-confirmed candle outside the live buffer is protected as well.
	yesterday := open.AddDate(0, 0, -1)
	service.HistoryChanged([]market.Candle{{InstrumentID: 1, Interval: market.IntervalDay, OpenTime: yesterday, Close: 20}})
	service.apply(ctx, kline.Event{Key: key, Candle: market.Candle{InstrumentID: 1, Interval: market.IntervalDay, OpenTime: yesterday, Close: 9}, Final: true, EventTime: time.Now().Add(time.Hour)})
	corrected := false
	for len(client.messages) > 0 {
		message := <-client.messages
		if message.Kind == KindUpdate && message.Candle != nil {
			t.Fatal("delayed WS final was published")
		}
		if message.Kind == KindSnapshot && len(message.Candles) > 0 && message.Candles[0].Candle.Close == 12 {
			corrected = true
		}
	}
	if !corrected {
		t.Fatal("committed correction was not published to the active graph")
	}
}

func TestReconnectRestoresMissedFinalBeforeFresh(t *testing.T) {
	upstream := newUpstreamStub()
	open := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	finalA := testCandle(open, market.IntervalHour, 11)
	history := &historyStub{candles: []market.Candle{finalA}}
	service := New(upstream, history, slog.New(slog.NewTextHandler(io.Discard, nil)), Options{ReleaseDelay: time.Second})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = service.Run(ctx) }()
	client := &clientStub{id: "client", messages: make(chan Message, 32)}
	key := kline.Key{Symbol: "BTCUSDT", Interval: market.IntervalHour}
	if err := service.Subscribe(ctx, client, key.Symbol, key.Interval); err != nil {
		t.Fatal(err)
	}
	upstream.events <- kline.Event{Key: key, Candle: testCandle(open, key.Interval, 10), Final: false}
	waitForMessage(t, client.messages, func(message Message) bool { return message.Kind == KindUpdate })
	upstream.statuses <- kline.Status{Keys: []kline.Key{key}, Connected: false}
	waitForMessage(t, client.messages, func(message Message) bool { return message.Freshness == FreshnessStale })
	upstream.statuses <- kline.Status{Keys: []kline.Key{key}, Connected: true}
	waitForMessage(t, client.messages, func(message Message) bool { return message.Freshness == FreshnessRecovering })
	upstream.events <- kline.Event{Key: key, Candle: testCandle(open.Add(time.Hour), key.Interval, 12), Final: false}
	message := waitForMessage(t, client.messages, func(message Message) bool {
		return message.Kind == KindSnapshot && message.Freshness == FreshnessFresh
	})
	for _, candle := range message.Candles {
		if candle.Candle.OpenTime.Equal(open) {
			if !candle.Final || candle.Candle.Close != 11 {
				t.Fatalf("missed final was not recovered: %+v", candle)
			}
			return
		}
	}
	t.Fatal("recovered snapshot omitted the missed candle")
}

func TestLoadRecoveryPaginatesBeyondLiveBuffer(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := make([]market.Candle, recoveryPagesPerBatch*maxRetainedCandles+40)
	for index := range candles {
		candles[index] = testCandle(start.Add(time.Duration(index)*time.Hour), market.IntervalHour, float64(index))
	}
	service := New(newUpstreamStub(), &historyStub{candles: candles}, slog.New(slog.NewTextHandler(io.Discard, nil)), Options{})
	service.recoveryLimiter = rate.NewLimiter(rate.Inf, 0)
	progress := newRecoveryProgress(candles[len(candles)-1].OpenTime)
	for {
		complete, stalled, err := service.loadRecoveryBatch(context.Background(), 1, market.IntervalHour, candles[0].OpenTime, &progress)
		if err != nil {
			t.Fatal(err)
		}
		if stalled {
			t.Fatal("recovery stalled while paginating continuous history")
		}
		if complete {
			break
		}
	}
	if len(progress.recent) != maxRetainedCandles {
		t.Fatalf("retained recovery candles = %d, want %d", len(progress.recent), maxRetainedCandles)
	}
}

func TestNewerRecoveryGenerationSupersedesOlderRange(t *testing.T) {
	service := New(newUpstreamStub(), &historyStub{}, slog.New(slog.NewTextHandler(io.Discard, nil)), Options{})
	state := &keyState{}
	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	first, ok := service.startRecoveryLocked(state, start, start.Add(time.Hour))
	if !ok {
		t.Fatal("first recovery was not started")
	}
	second, ok := service.startRecoveryLocked(state, start, start.Add(3*time.Hour))
	if !ok || second <= first {
		t.Fatalf("recovery generations did not advance: first=%d second=%d", first, second)
	}
	if !state.recoveryThrough.Equal(start.Add(3 * time.Hour)) {
		t.Fatalf("newer recovery target was lost: %s", state.recoveryThrough)
	}
}

func waitForMessage(t *testing.T, messages <-chan Message, matches func(Message) bool) Message {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case message := <-messages:
			if matches(message) {
				return message
			}
		case <-deadline:
			t.Fatal("timed out waiting for live message")
		}
	}
}

func testCandle(open time.Time, interval market.CandleInterval, closePrice float64) market.Candle {
	return market.Candle{Interval: interval, OpenTime: open, CloseTime: interval.NextOpenTime(open).Add(-time.Millisecond), Open: closePrice - 1, High: closePrice + 1, Low: closePrice - 2, Close: closePrice, Volume: 1, QuoteAssetVolume: 1, TradeCount: 1}
}
