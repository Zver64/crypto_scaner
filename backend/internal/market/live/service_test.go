package live

import (
	"context"
	"errors"
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
	events        chan kline.Event
	statusesReady chan struct{}
	mu            sync.Mutex
	statuses      []kline.Status
	subscribes    map[kline.Key]int
	unsubscribes  map[kline.Key]int
}

func newUpstreamStub() *upstreamStub {
	return &upstreamStub{events: make(chan kline.Event, 8), statusesReady: make(chan struct{}, 1), subscribes: make(map[kline.Key]int), unsubscribes: make(map[kline.Key]int)}
}

// publishStatus queues status for DrainStatuses.
func (stub *upstreamStub) publishStatus(status kline.Status) {
	stub.mu.Lock()
	stub.statuses = append(stub.statuses, status)
	stub.mu.Unlock()
	select {
	case stub.statusesReady <- struct{}{}:
	default:
	}
}
func (stub *upstreamStub) Subscribe(key kline.Key) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	stub.subscribes[key]++
}
func (stub *upstreamStub) Unsubscribe(key kline.Key) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	stub.unsubscribes[key]++
}
func (stub *upstreamStub) Events() <-chan kline.Event     { return stub.events }
func (stub *upstreamStub) StatusesReady() <-chan struct{} { return stub.statusesReady }
func (stub *upstreamStub) DrainStatuses() []kline.Status {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	statuses := stub.statuses
	stub.statuses = nil
	return statuses
}
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

// currentStub returns its candle as the exchange's newest one.
type currentStub struct{ candle market.Candle }

func (stub *currentStub) CurrentCandle(context.Context, string, market.CandleInterval) (market.Candle, error) {
	return stub.candle, nil
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
	service := New(newUpstreamStub(), &currentStub{}, &historyStub{}, slog.New(slog.NewTextHandler(io.Discard, nil)), Options{})
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
	service := New(upstream, &currentStub{}, &historyStub{}, slog.New(slog.NewTextHandler(io.Discard, nil)), Options{ReleaseDelay: 25 * time.Millisecond})
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
	service := New(upstream, &currentStub{}, &historyStub{}, slog.New(slog.NewTextHandler(io.Discard, nil)), Options{ReleaseDelay: time.Second})
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
	service := New(newUpstreamStub(), &currentStub{}, &historyStub{}, slog.New(slog.NewTextHandler(io.Discard, nil)), Options{})
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
	service := New(upstream, &currentStub{}, history, slog.New(slog.NewTextHandler(io.Discard, nil)), Options{ReleaseDelay: time.Second})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = service.Run(ctx) }()
	client := &clientStub{id: "client", messages: make(chan Message, 32)}
	key := kline.Key{Symbol: "BTCUSDT", Interval: market.IntervalHour}
	if err := service.Subscribe(ctx, client, key.Symbol, key.Interval); err != nil {
		t.Fatal(err)
	}
	upstream.events <- kline.Event{Key: key, Candle: testCandle(open, key.Interval, 10), Final: false, Epoch: 1}
	waitForMessage(t, client.messages, func(message Message) bool { return message.Kind == KindUpdate })
	upstream.publishStatus(kline.Status{Keys: []kline.Key{key}, Connected: false, Epoch: 1})
	waitForMessage(t, client.messages, func(message Message) bool { return message.Freshness == FreshnessStale })
	upstream.publishStatus(kline.Status{Keys: []kline.Key{key}, Connected: true, Epoch: 2})
	waitForMessage(t, client.messages, func(message Message) bool { return message.Freshness == FreshnessRecovering })
	upstream.events <- kline.Event{Key: key, Candle: testCandle(open.Add(time.Hour), key.Interval, 12), Final: false, Epoch: 2}
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

// flakyHistory fails every page query while broken is set.
type flakyHistory struct {
	*historyStub
	broken bool
}

func (stub *flakyHistory) ListCandlePage(ctx context.Context, id int64, interval market.CandleInterval, before *time.Time, limit int) (market.CandlePage, error) {
	stub.mu.Lock()
	failed := stub.broken
	stub.mu.Unlock()
	if failed {
		return market.CandlePage{}, errors.New("query failed")
	}
	return stub.historyStub.ListCandlePage(ctx, id, interval, before, limit)
}

// recoveringService subscribes a client to an hourly key and returns the
// service with unlimited stored-candle reads.
func recoveringService(t *testing.T, store HistoryStore) (*Service, kline.Key, *clientStub) {
	t.Helper()
	service := New(newUpstreamStub(), &currentStub{}, store, slog.New(slog.NewTextHandler(io.Discard, nil)), Options{})
	service.readLimiter = rate.NewLimiter(rate.Inf, 0)
	key := kline.Key{Symbol: "BTCUSDT", Interval: market.IntervalHour}
	client := &clientStub{id: "client", messages: make(chan Message, 1024)}
	if err := service.Subscribe(context.Background(), client, key.Symbol, key.Interval); err != nil {
		t.Fatal(err)
	}
	return service, key, client
}

func TestFailedReadIsRetriedByNextStreamEvent(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	hour := func(index int) market.Candle {
		candle := testCandle(start.Add(time.Duration(index)*time.Hour), market.IntervalHour, float64(index))
		candle.InstrumentID = 1
		return candle
	}
	store := &flakyHistory{historyStub: &historyStub{candles: []market.Candle{hour(0), hour(1), hour(2)}}, broken: true}
	service, key, client := recoveringService(t, store)
	ctx := context.Background()
	service.apply(ctx, kline.Event{Key: key, Candle: hour(0), Final: true, Epoch: 1})
	// The gap after hour 0 is read while the store fails.
	service.apply(ctx, kline.Event{Key: key, Candle: hour(3), Epoch: 1})
	service.workers.Wait()
	service.mu.Lock()
	freshness := service.states[key].freshness()
	service.mu.Unlock()
	if freshness != FreshnessRecovering {
		t.Fatalf("freshness after a failed read = %s, want recovering", freshness)
	}
	store.mu.Lock()
	store.broken = false
	store.mu.Unlock()
	// The next packet, without a gap of its own, reads the gap again.
	service.apply(ctx, kline.Event{Key: key, Candle: hour(3), Epoch: 1})
	service.workers.Wait()
	message := waitForMessage(t, client.messages, func(message Message) bool {
		return message.Kind == KindSnapshot && message.Freshness == FreshnessFresh
	})
	recovered := make(map[time.Time]bool)
	for _, candle := range message.Candles {
		recovered[candle.Candle.OpenTime] = candle.Final
	}
	for index := range 3 {
		if !recovered[hour(index).OpenTime] {
			t.Fatalf("hour %d missing or not final after recovery", index)
		}
	}
}

func TestEvictionFromLiveBufferDoesNotRecover(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	service, key, client := recoveringService(t, &historyStub{})
	ctx := context.Background()
	for index := range maxRetainedCandles + 1 {
		service.apply(ctx, kline.Event{Key: key, Candle: testCandle(start.Add(time.Duration(index)*time.Hour), key.Interval, 1), Final: true, Epoch: 1})
	}
	service.apply(ctx, kline.Event{Key: key, Candle: testCandle(start.Add(time.Duration(maxRetainedCandles+1)*time.Hour), key.Interval, 1), Epoch: 1})
	service.workers.Wait()
	service.mu.Lock()
	missed := service.states[key].gap
	service.mu.Unlock()
	if !missed.empty() {
		t.Fatalf("eviction left a gap %+v", missed)
	}
	for len(client.messages) > 0 {
		if message := <-client.messages; message.Kind == KindUpdate && message.Freshness != FreshnessFresh {
			t.Fatalf("update freshness = %s, want fresh", message.Freshness)
		}
	}
}

func TestRecoveryWithoutClientsServesResubscribe(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	hour := func(index int) market.Candle {
		candle := testCandle(start.Add(time.Duration(index)*time.Hour), market.IntervalHour, float64(index))
		candle.InstrumentID = 1
		return candle
	}
	service, key, client := recoveringService(t, &historyStub{candles: []market.Candle{hour(0), hour(1), hour(2)}})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.apply(ctx, kline.Event{Key: key, Candle: hour(0), Final: true, Epoch: 1})
	service.Unsubscribe(client.ID(), key)
	// The gap is read while the key has no clients.
	service.apply(ctx, kline.Event{Key: key, Candle: hour(3), Epoch: 1})
	service.workers.Wait()
	go func() { _ = service.Run(ctx) }()
	if err := service.Subscribe(ctx, client, key.Symbol, key.Interval); err != nil {
		t.Fatal(err)
	}
	message := waitForMessage(t, client.messages, func(message Message) bool {
		return message.Kind == KindSnapshot && message.Freshness == FreshnessFresh
	})
	recovered := make(map[time.Time]bool)
	for _, candle := range message.Candles {
		recovered[candle.Candle.OpenTime] = candle.Final
	}
	if !recovered[hour(1).OpenTime] || !recovered[hour(2).OpenTime] {
		t.Fatal("recovery without clients did not restore the gap")
	}
}

func TestEventProvenReconnectRecoversLikeStatus(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	service, key, _ := recoveringService(t, &historyStub{})
	ctx := context.Background()
	service.apply(ctx, kline.Event{Key: key, Candle: testCandle(start, key.Interval, 1), Epoch: 1})
	service.applyStatus(kline.Status{Keys: []kline.Key{key}, Connected: false, Epoch: 1})
	// The connected status of epoch 2 was lost; its first packet reconnects.
	service.apply(ctx, kline.Event{Key: key, Candle: testCandle(start.Add(time.Hour), key.Interval, 2), Epoch: 2})
	service.workers.Wait()
	service.mu.Lock()
	missed := service.states[key].gap
	service.mu.Unlock()
	if !missed.from.Equal(start) {
		t.Fatalf("reconnect gap = %+v, want from %s", missed, start)
	}
}

func TestDeadConnectionEventsKeepKeyStale(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	service, key, client := recoveringService(t, &historyStub{})
	ctx := context.Background()
	service.apply(ctx, kline.Event{Key: key, Candle: testCandle(start, key.Interval, 1), Epoch: 1})
	service.applyStatus(kline.Status{Keys: []kline.Key{key}, Connected: false, Epoch: 1})
	// A packet of the dead connection still buffered after its status.
	service.apply(ctx, kline.Event{Key: key, Candle: testCandle(start, key.Interval, 2), Epoch: 1})
	update := waitForMessage(t, client.messages, func(message Message) bool {
		return message.Kind == KindUpdate && message.Candle.Candle.Close == 2
	})
	if update.Freshness != FreshnessStale {
		t.Fatalf("freshness after dead-connection packet = %s, want stale", update.Freshness)
	}
	// Data of a newer connection proves it alive, even if its status was lost.
	service.apply(ctx, kline.Event{Key: key, Candle: testCandle(start, key.Interval, 3), Epoch: 2})
	update = waitForMessage(t, client.messages, func(message Message) bool {
		return message.Kind == KindUpdate && message.Candle.Candle.Close == 3
	})
	if update.Freshness != FreshnessFresh {
		t.Fatalf("freshness after newer connection packet = %s, want fresh", update.Freshness)
	}
}

func TestDroppedPacketPublishesReconnect(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	service, key, client := recoveringService(t, &historyStub{})
	ctx := context.Background()
	service.apply(ctx, kline.Event{Key: key, Candle: testCandle(start, key.Interval, 1), Final: true, Epoch: 1})
	service.applyStatus(kline.Status{Keys: []kline.Key{key}, Connected: false, Epoch: 1})
	// An open packet of the final candle is dropped, but its connection is new.
	service.apply(ctx, kline.Event{Key: key, Candle: testCandle(start, key.Interval, 2), Epoch: 2})
	waitForMessage(t, client.messages, func(message Message) bool {
		return message.Kind == KindStatus && message.Freshness == FreshnessRecovering
	})
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

func TestSubscribePublishesFormingCandleBeforeAnyStreamEvent(t *testing.T) {
	open := market.IntervalHour.OpenTime(time.Now())
	forming := market.Candle{Interval: market.IntervalHour, OpenTime: open, CloseTime: open.Add(time.Hour - time.Millisecond), Open: 1, High: 2, Low: 1, Close: 2}
	service := New(newUpstreamStub(), &currentStub{candle: forming}, &historyStub{}, slog.New(slog.NewTextHandler(io.Discard, nil)), Options{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = service.Run(ctx) }()
	client := &clientStub{id: "viewer", messages: make(chan Message, 16)}
	if err := service.Subscribe(ctx, client, "BTCUSDT", market.IntervalHour); err != nil {
		t.Fatal(err)
	}
	for {
		select {
		case message := <-client.messages:
			if message.Kind != KindUpdate {
				continue
			}
			if message.Candle == nil || message.Candle.Final || message.Candle.Candle != forming {
				t.Fatalf("update = %+v, want the forming candle", message.Candle)
			}
			return
		case <-time.After(time.Second):
			t.Fatal("forming candle was not published without a stream event")
		}
	}
}

func TestGapRecoversWhenSynchronizationCommitsLate(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	hour := func(index int) market.Candle {
		candle := testCandle(start.Add(time.Duration(index)*time.Hour), market.IntervalHour, float64(index))
		candle.InstrumentID = 1
		return candle
	}
	// Hours 1 and 2 are stored; hour 3 is not synchronized yet.
	service, key, client := recoveringService(t, &historyStub{candles: []market.Candle{hour(0), hour(1), hour(2)}})
	ctx := context.Background()
	service.apply(ctx, kline.Event{Key: key, Candle: hour(0), Final: true, Epoch: 1})
	service.apply(ctx, kline.Event{Key: key, Candle: hour(4), Epoch: 1})
	service.workers.Wait()
	service.mu.Lock()
	freshness := service.states[key].freshness()
	service.mu.Unlock()
	if freshness != FreshnessRecovering {
		t.Fatalf("freshness while hour 3 is not synchronized = %s, want recovering", freshness)
	}
	for len(client.messages) > 0 {
		<-client.messages
	}
	service.HistoryChanged([]market.Candle{hour(3)})
	message := waitForMessage(t, client.messages, func(message Message) bool {
		return message.Kind == KindSnapshot && message.Freshness != FreshnessRecovering
	})
	if message.Freshness != FreshnessFresh {
		t.Fatalf("freshness after synchronization committed hour 3 = %s, want fresh", message.Freshness)
	}
	recovered := make(map[time.Time]bool)
	for _, candle := range message.Candles {
		recovered[candle.Candle.OpenTime] = candle.Final
	}
	for index := range 4 {
		if !recovered[hour(index).OpenTime] {
			t.Fatalf("hour %d missing or not final after recovery", index)
		}
	}
}
