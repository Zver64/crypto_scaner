// Package live coordinates shared, demand-driven candle subscriptions.
package live

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"crypto-scanner/internal/market"
	"crypto-scanner/internal/market/kline"
	"crypto-scanner/internal/platform/backoff"

	"golang.org/x/time/rate"
)

const (
	defaultReleaseDelay   = 15 * time.Second
	maxRetainedCandles    = 16
	recoveryPagesPerBatch = 16
	recoveryQueryRate     = 20
	// maxClientSubscriptions bounds the live keys of one client connection.
	maxClientSubscriptions = 8
)

var (
	ErrInactiveSymbol             = fmt.Errorf("live candles: %w", market.ErrInstrumentNotFound)
	ErrInvalidInterval            = errors.New("unsupported candle interval")
	ErrTooManyClientSubscriptions = errors.New("too many client subscriptions")
	ErrServiceStopped             = errors.New("live service is stopped")
)

type HistoryStore interface {
	GetActiveInstrumentBySymbol(context.Context, string) (market.Instrument, error)
	ListCandlePage(context.Context, int64, market.CandleInterval, *time.Time, int) (market.CandlePage, error)
}

type Freshness string

const (
	FreshnessWaiting    Freshness = "waiting"
	FreshnessFresh      Freshness = "fresh"
	FreshnessStale      Freshness = "stale"
	FreshnessRecovering Freshness = "recovering"
)

type CandleState struct {
	Candle market.Candle
	Final  bool
}

type MessageKind string

const (
	KindSubscribed MessageKind = "subscribed"
	KindSnapshot   MessageKind = "snapshot"
	KindUpdate     MessageKind = "update"
	KindStatus     MessageKind = "status"
)

type Message struct {
	Kind      MessageKind
	Key       kline.Key
	Candle    *CandleState
	Candles   []CandleState
	Freshness Freshness
	Reason    string
}

// Client receives manager messages without ever blocking the upstream reader.
type Client interface {
	ID() string
	Enqueue(Message) bool
	Close()
}

type clientState struct {
	client Client
	keys   map[kline.Key]struct{}
}

type keyState struct {
	instrument      market.Instrument
	clients         map[string]Client
	candles         map[time.Time]CandleState
	latestConfirmed time.Time // REST-confirmed high-water mark; independent of clock skew
	freshness       Freshness
	release         *time.Timer
	generation      uint64

	recovering         bool
	recoveryGeneration uint64
	recoveryStart      time.Time // inclusive
	recoveryThrough    time.Time // inclusive
	reconnectFrom      *time.Time
	upstreamConnected  bool
}

// delivery is one message for a set of clients, published outside the lock.
type delivery struct {
	clients []Client
	message Message
}

// settledFreshness is the freshness of a key that is not recovering.
func (state *keyState) settledFreshness() Freshness {
	if state.upstreamConnected {
		return FreshnessFresh
	}
	return FreshnessStale
}

type Options struct {
	ReleaseDelay time.Duration
}

type Service struct {
	upstream        kline.Feed
	store           HistoryStore
	logger          *slog.Logger
	delay           time.Duration
	recoveryLimiter *rate.Limiter

	// recoveries tracks recovery goroutines. They are started only from Run's
	// goroutine (via apply) with Run's context, and Run waits for them.
	recoveries sync.WaitGroup

	mu      sync.Mutex
	states  map[kline.Key]*keyState
	clients map[string]*clientState
	running bool
	stopped bool
}

func New(upstream kline.Feed, store HistoryStore, logger *slog.Logger, options Options) *Service {
	delay := options.ReleaseDelay
	if delay <= 0 {
		delay = defaultReleaseDelay
	}
	return &Service{upstream: upstream, store: store, logger: logger, delay: delay, recoveryLimiter: rate.NewLimiter(rate.Limit(recoveryQueryRate), 4), states: make(map[kline.Key]*keyState), clients: make(map[string]*clientState)}
}

func (service *Service) Run(ctx context.Context) error {
	service.mu.Lock()
	if service.running {
		service.mu.Unlock()
		return errors.New("live service already running")
	}
	service.running = true
	service.mu.Unlock()
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		service.recoveries.Wait()
		service.shutdown()
	}()
	for {
		select {
		case <-ctx.Done():
			return nil
		case event := <-service.upstream.Events():
			service.apply(ctx, event)
		case status := <-service.upstream.Statuses():
			service.applyStatus(status)
		}
	}
}

func (service *Service) RegisterClient(client Client) bool {
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.stopped {
		return false
	}
	state := service.clients[client.ID()]
	if state == nil {
		state = &clientState{keys: make(map[kline.Key]struct{})}
		service.clients[client.ID()] = state
	}
	state.client = client
	return true
}

func (service *Service) Subscribe(ctx context.Context, client Client, symbol string, interval market.CandleInterval) error {
	symbol = market.NormalizeSymbol(symbol)
	if !interval.Valid() {
		return ErrInvalidInterval
	}
	key := kline.Key{Symbol: symbol, Interval: interval}
	service.mu.Lock()
	if service.stopped {
		service.mu.Unlock()
		return ErrServiceStopped
	}
	if clientState := service.clients[client.ID()]; clientState != nil {
		if _, exists := clientState.keys[key]; exists {
			service.mu.Unlock()
			return nil
		}
		if len(clientState.keys) >= maxClientSubscriptions {
			service.mu.Unlock()
			return ErrTooManyClientSubscriptions
		}
	}
	state := service.states[key]
	service.mu.Unlock()
	if state == nil {
		instrument, err := service.store.GetActiveInstrumentBySymbol(ctx, symbol)
		if err != nil {
			if errors.Is(err, market.ErrInstrumentNotFound) {
				return ErrInactiveSymbol
			}
			return fmt.Errorf("look up active instrument: %w", err)
		}
		service.mu.Lock()
		if service.stopped {
			service.mu.Unlock()
			return ErrServiceStopped
		}
		state = service.states[key]
		if state == nil {
			state = &keyState{instrument: instrument, clients: make(map[string]Client), candles: make(map[time.Time]CandleState), freshness: FreshnessWaiting}
			service.states[key] = state
			if err := service.upstream.Subscribe(key); err != nil {
				delete(service.states, key)
				service.mu.Unlock()
				return fmt.Errorf("subscribe upstream: %w", err)
			}
		}
	} else {
		service.mu.Lock()
		if service.stopped {
			service.mu.Unlock()
			return ErrServiceStopped
		}
	}
	if state.release != nil {
		state.release.Stop()
		state.release = nil
		state.generation++
	}
	state.clients[client.ID()] = client
	registration := service.clients[client.ID()]
	if registration == nil {
		registration = &clientState{client: client, keys: make(map[kline.Key]struct{})}
		service.clients[client.ID()] = registration
	}
	registration.keys[key] = struct{}{}
	message := snapshotMessage(key, state)
	service.mu.Unlock()
	if !client.Enqueue(Message{Kind: KindSubscribed, Key: key}) || !client.Enqueue(message) {
		service.RemoveClient(client.ID())
		client.Close()
	}
	service.logCounts("subscribe")
	return nil
}

func (service *Service) Unsubscribe(clientID string, key kline.Key) {
	key.Symbol = market.NormalizeSymbol(key.Symbol)
	service.mu.Lock()
	service.unsubscribeLocked(clientID, key)
	service.mu.Unlock()
	service.logCounts("unsubscribe")
}

func (service *Service) RemoveClient(clientID string) {
	service.mu.Lock()
	clientState := service.clients[clientID]
	var keys []kline.Key
	if clientState != nil {
		keys = make([]kline.Key, 0, len(clientState.keys))
		for key := range clientState.keys {
			keys = append(keys, key)
		}
	}
	for _, key := range keys {
		service.unsubscribeLocked(clientID, key)
	}
	delete(service.clients, clientID)
	service.mu.Unlock()
	service.logCounts("disconnect")
}

func (service *Service) unsubscribeLocked(clientID string, key kline.Key) {
	state := service.states[key]
	if state == nil {
		return
	}
	if _, exists := state.clients[clientID]; !exists {
		return
	}
	delete(state.clients, clientID)
	if clientState := service.clients[clientID]; clientState != nil {
		delete(clientState.keys, key)
	}
	if len(state.clients) != 0 {
		return
	}
	state.generation++
	generation := state.generation
	state.release = time.AfterFunc(service.delay, func() { service.release(key, generation) })
}

func (service *Service) release(key kline.Key, generation uint64) {
	service.mu.Lock()
	state := service.states[key]
	if state == nil || state.generation != generation || len(state.clients) != 0 {
		service.mu.Unlock()
		return
	}
	delete(service.states, key)
	service.upstream.Unsubscribe(key)
	service.mu.Unlock()
	service.logCounts("release")
}

// apply runs on Run's goroutine; ctx bounds any recovery it starts.
func (service *Service) apply(ctx context.Context, event kline.Event) {
	service.mu.Lock()
	state := service.states[event.Key]
	if state == nil {
		service.mu.Unlock()
		return
	}
	if event.Final && !state.latestConfirmed.IsZero() && !event.Candle.OpenTime.After(state.latestConfirmed) {
		service.mu.Unlock()
		return
	}
	// Old packets cannot re-enter the bounded buffer after their REST
	// confirmation has been evicted and replace persisted chart history.
	if _, exists := state.candles[event.Candle.OpenTime]; !exists && len(state.candles) >= maxRetainedCandles {
		older := false
		for open := range state.candles {
			if open.Before(event.Candle.OpenTime) {
				older = true
				break
			}
		}
		if !older {
			service.mu.Unlock()
			return
		}
	}
	incoming := CandleState{Candle: event.Candle, Final: event.Final}
	if state.freshness != FreshnessStale {
		state.upstreamConnected = true
	}
	if current, exists := state.candles[event.Candle.OpenTime]; exists && current.Final && !incoming.Final {
		service.mu.Unlock()
		return
	}
	previousLatest := latestOpen(state.candles)
	previous, hadPrevious := state.candles[previousLatest]
	state.candles[event.Candle.OpenTime] = incoming
	evicted := trimCandles(state.candles)

	var recoveryStart, recoveryThrough time.Time
	if state.reconnectFrom != nil {
		recoveryStart = *state.reconnectFrom
		recoveryThrough = closedThrough(event.Key.Interval, incoming)
		state.reconnectFrom = nil
	} else if hadPrevious && event.Candle.OpenTime.After(event.Key.Interval.NextOpenTime(previousLatest)) {
		recoveryStart = previousLatest
		if previous.Final {
			recoveryStart = event.Key.Interval.NextOpenTime(previousLatest)
		}
		recoveryThrough = closedThrough(event.Key.Interval, incoming)
	}
	if !evicted.IsZero() {
		if recoveryStart.IsZero() || evicted.Before(recoveryStart) {
			recoveryStart = evicted
		}
		if recoveryThrough.Before(evicted) {
			recoveryThrough = evicted
		}
	}
	if state.recovering {
		through := closedThrough(event.Key.Interval, incoming)
		if through.After(state.recoveryThrough) {
			if recoveryStart.IsZero() {
				recoveryStart = state.recoveryStart
			}
			recoveryThrough = through
		}
	}
	generation, recoverNow := service.startRecoveryLocked(state, recoveryStart, recoveryThrough)
	if !state.recovering {
		state.freshness = state.settledFreshness()
	}
	clients := clientSlice(state.clients)
	freshness := state.freshness
	service.mu.Unlock()
	service.publish(clients, Message{Kind: KindUpdate, Key: event.Key, Candle: &incoming, Freshness: freshness})
	if recoverNow {
		service.recoveries.Go(func() { service.recover(ctx, event.Key, generation) })
	}
}

// HistoryChanged publishes committed REST closures and corrections to active
// graph subscribers. This path is demand-driven: it adds no Binance subscription.
func (service *Service) HistoryChanged(candles []market.Candle) {
	service.mu.Lock()
	changed := make(map[kline.Key]struct{})
	for key, state := range service.states {
		for _, candle := range candles {
			if candle.InstrumentID != state.instrument.ID || candle.Interval != key.Interval {
				continue
			}
			if candle.OpenTime.After(state.latestConfirmed) {
				state.latestConfirmed = candle.OpenTime
			}
			if _, exists := state.candles[candle.OpenTime]; exists {
				state.candles[candle.OpenTime] = CandleState{Candle: candle, Final: true}
			}
			changed[key] = struct{}{}
		}
	}
	messages := make([]delivery, 0, len(changed))
	for key := range changed {
		state := service.states[key]
		messages = append(messages, delivery{clientSlice(state.clients), snapshotMessage(key, state)})
	}
	service.mu.Unlock()
	for _, item := range messages {
		service.publish(item.clients, item.message)
	}
}

func (service *Service) applyStatus(status kline.Status) {
	service.mu.Lock()
	messages := make([]delivery, 0, len(status.Keys))
	for _, key := range status.Keys {
		state := service.states[key]
		if state == nil {
			continue
		}
		state.upstreamConnected = status.Connected
		if status.Connected {
			if len(state.candles) == 0 {
				state.freshness = FreshnessWaiting
			} else {
				state.freshness = FreshnessRecovering
				latest := latestOpen(state.candles)
				from := latest
				if state.candles[latest].Final {
					from = key.Interval.NextOpenTime(latest)
				}
				state.reconnectFrom = &from
			}
		} else {
			state.freshness = FreshnessStale
		}
		messages = append(messages, delivery{clientSlice(state.clients), Message{Kind: KindStatus, Key: key, Freshness: state.freshness, Reason: errorString(status.Err)}})
	}
	service.mu.Unlock()
	for _, item := range messages {
		service.publish(item.clients, item.message)
	}
}

func (service *Service) startRecoveryLocked(state *keyState, start, through time.Time) (uint64, bool) {
	if start.IsZero() || through.Before(start) {
		if state.reconnectFrom == nil && !state.recovering {
			state.freshness = FreshnessFresh
		}
		return 0, false
	}
	if state.recovering {
		if state.recoveryStart.Before(start) {
			start = state.recoveryStart
		}
		if state.recoveryThrough.After(through) {
			through = state.recoveryThrough
		}
	}
	state.recovering = true
	state.freshness = FreshnessRecovering
	state.recoveryStart = start
	state.recoveryThrough = through
	state.recoveryGeneration++
	return state.recoveryGeneration, true
}

func (service *Service) recover(ctx context.Context, key kline.Key, generation uint64) {
	delay := time.Duration(0)
	failures := 0
	var progress recoveryProgress
	for {
		if delay > 0 && backoff.Sleep(ctx, delay) != nil {
			return
		}
		service.mu.Lock()
		state := service.states[key]
		if state == nil || !state.recovering || state.recoveryGeneration != generation || len(state.clients) == 0 {
			service.mu.Unlock()
			return
		}
		instrument, start, through := state.instrument, state.recoveryStart, state.recoveryThrough
		service.mu.Unlock()
		if progress.expected.IsZero() {
			progress = newRecoveryProgress(through)
		}

		batchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		complete, stalled, err := service.loadRecoveryBatch(batchCtx, instrument.ID, key.Interval, start, &progress)
		cancel()
		service.mu.Lock()
		state = service.states[key]
		if state == nil || !state.recovering || state.recoveryGeneration != generation {
			service.mu.Unlock()
			return
		}
		if complete {
			for _, candle := range progress.recent {
				current, exists := state.candles[candle.OpenTime]
				if !exists || !current.Final {
					state.candles[candle.OpenTime] = CandleState{Candle: candle, Final: true}
				}
			}
			trimCandles(state.candles)
			state.recovering = false
			state.recoveryStart = time.Time{}
			state.recoveryThrough = time.Time{}
			state.freshness = state.settledFreshness()
		}
		clients, snapshot := clientSlice(state.clients), snapshotMessage(key, state)
		service.mu.Unlock()
		service.publish(clients, snapshot)
		if complete {
			return
		}
		if stalled {
			progress = newRecoveryProgress(through)
		}
		if err == nil && !stalled {
			// The cursor is retained across bounded batches, while the process-wide
			// limiter prevents many recovering streams from overwhelming PostgreSQL.
			delay = 100 * time.Millisecond
			failures = 0
			continue
		}
		delay = backoff.Exponential(time.Second, 30*time.Second, failures)
		failures++
		service.logger.Warn("live candle history recovery incomplete", "module", "market_live", "operation", "recover", "symbol", key.Symbol, "interval", key.Interval, "error", errorString(err), "retry_in", delay)
	}
}

type recoveryProgress struct {
	expected time.Time
	before   *time.Time
	recent   []market.Candle
}

func newRecoveryProgress(through time.Time) recoveryProgress {
	return recoveryProgress{expected: through, recent: make([]market.Candle, 0, maxRetainedCandles)}
}

func (service *Service) loadRecoveryBatch(ctx context.Context, instrumentID int64, interval market.CandleInterval, start time.Time, progress *recoveryProgress) (complete, stalled bool, err error) {
	for range recoveryPagesPerBatch {
		if err := service.recoveryLimiter.Wait(ctx); err != nil {
			return false, false, err
		}
		page, err := service.store.ListCandlePage(ctx, instrumentID, interval, progress.before, maxRetainedCandles)
		if err != nil {
			return false, false, err
		}
		for index := len(page.Candles) - 1; index >= 0; index-- {
			candle := page.Candles[index]
			if candle.OpenTime.After(progress.expected) {
				continue
			}
			if candle.OpenTime.Before(progress.expected) {
				return false, true, nil
			}
			if len(progress.recent) < maxRetainedCandles {
				progress.recent = append(progress.recent, candle)
			}
			progress.expected = interval.PreviousOpenTime(progress.expected)
			if progress.expected.Before(start) {
				return true, false, nil
			}
		}
		if !page.HasMore || len(page.Candles) == 0 {
			return false, true, nil
		}
		cursor := page.Candles[0].OpenTime.UTC()
		progress.before = &cursor
	}
	return false, false, nil
}

func closedThrough(interval market.CandleInterval, candle CandleState) time.Time {
	if candle.Final {
		return candle.Candle.OpenTime
	}
	return interval.PreviousOpenTime(candle.Candle.OpenTime)
}

func (service *Service) publish(clients []Client, message Message) {
	for _, client := range clients {
		if !client.Enqueue(message) {
			client.Close()
			service.RemoveClient(client.ID())
		}
	}
}

func snapshotMessage(key kline.Key, state *keyState) Message {
	candles := make([]CandleState, 0, len(state.candles))
	for _, candle := range state.candles {
		candles = append(candles, candle)
	}
	slices.SortFunc(candles, func(left, right CandleState) int { return left.Candle.OpenTime.Compare(right.Candle.OpenTime) })
	return Message{Kind: KindSnapshot, Key: key, Candles: candles, Freshness: state.freshness}
}

func clientSlice(values map[string]Client) []Client {
	result := make([]Client, 0, len(values))
	for _, value := range values {
		result = append(result, value)
	}
	return result
}

func latestOpen(values map[time.Time]CandleState) time.Time {
	var latest time.Time
	for open := range values {
		if open.After(latest) {
			latest = open
		}
	}
	return latest
}

func trimCandles(values map[time.Time]CandleState) time.Time {
	if len(values) <= maxRetainedCandles {
		return time.Time{}
	}
	keys := make([]time.Time, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, time.Time.Compare)
	removed := keys[:len(keys)-maxRetainedCandles]
	for _, key := range removed {
		delete(values, key)
	}
	return removed[len(removed)-1]
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (service *Service) shutdown() {
	service.mu.Lock()
	service.stopped = true
	clients := make([]Client, 0, len(service.clients))
	for _, state := range service.clients {
		clients = append(clients, state.client)
	}
	for key, state := range service.states {
		if state.release != nil {
			state.release.Stop()
		}
		service.upstream.Unsubscribe(key)
	}
	service.states = make(map[kline.Key]*keyState)
	service.clients = make(map[string]*clientState)
	service.mu.Unlock()
	for _, client := range clients {
		client.Close()
	}
}

func (service *Service) logCounts(operation string) {
	service.mu.Lock()
	connections, subscriptions := len(service.clients), len(service.states)
	service.mu.Unlock()
	service.logger.Info("live candle subscriptions changed", "module", "market_live", "operation", operation, "clients", connections, "subscriptions", subscriptions)
}
