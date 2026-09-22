// Package live coordinates shared, demand-driven candle subscriptions.
package live

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"crypto-scanner/internal/exchange/binance"
	"crypto-scanner/internal/market"

	"golang.org/x/time/rate"
)

const (
	defaultReleaseDelay   = 15 * time.Second
	maxRetainedCandles    = 16
	recoveryPagesPerBatch = 16
	recoveryQueryRate     = 20
)

var (
	ErrInactiveSymbol             = errors.New("symbol is unknown or inactive")
	ErrInvalidInterval            = errors.New("unsupported candle interval")
	ErrTooManyClientSubscriptions = errors.New("too many client subscriptions")
	ErrServiceStopped             = errors.New("live service is stopped")
)

type Upstream interface {
	Subscribe(binance.KlineKey) error
	Unsubscribe(binance.KlineKey)
	Events() <-chan binance.KlineEvent
	Statuses() <-chan binance.StreamStatus
}

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

type Message struct {
	Kind      string
	Key       binance.KlineKey
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
	keys   map[binance.KlineKey]struct{}
}

type keyState struct {
	instrument market.Instrument
	clients    map[string]Client
	candles    map[time.Time]CandleState
	freshness  Freshness
	release    *time.Timer
	generation uint64

	recovering         bool
	recoveryGeneration uint64
	recoveryStart      time.Time // inclusive
	recoveryThrough    time.Time // inclusive
	reconnectFrom      *time.Time
	upstreamConnected  bool
}

type Options struct {
	ReleaseDelay time.Duration
}

type Service struct {
	upstream        Upstream
	store           HistoryStore
	logger          *slog.Logger
	delay           time.Duration
	recoveryLimiter *rate.Limiter

	mu      sync.Mutex
	states  map[binance.KlineKey]*keyState
	clients map[string]*clientState
	ctx     context.Context
	stopped bool
}

func New(upstream Upstream, store HistoryStore, logger *slog.Logger) *Service {
	return NewWithOptions(upstream, store, logger, Options{})
}

func NewWithOptions(upstream Upstream, store HistoryStore, logger *slog.Logger, options Options) *Service {
	delay := options.ReleaseDelay
	if delay <= 0 {
		delay = defaultReleaseDelay
	}
	return &Service{upstream: upstream, store: store, logger: logger, delay: delay, recoveryLimiter: rate.NewLimiter(rate.Limit(recoveryQueryRate), 4), states: make(map[binance.KlineKey]*keyState), clients: make(map[string]*clientState)}
}

func (service *Service) Run(ctx context.Context) error {
	service.mu.Lock()
	if service.ctx != nil {
		service.mu.Unlock()
		return errors.New("live service already running")
	}
	service.ctx = ctx
	service.mu.Unlock()
	defer service.shutdown()
	for {
		select {
		case <-ctx.Done():
			return nil
		case event := <-service.upstream.Events():
			service.apply(event)
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
		state = &clientState{keys: make(map[binance.KlineKey]struct{})}
		service.clients[client.ID()] = state
	}
	state.client = client
	return true
}

func (service *Service) Subscribe(ctx context.Context, client Client, symbol string, interval market.CandleInterval) error {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if !interval.Valid() {
		return ErrInvalidInterval
	}
	key := binance.KlineKey{Symbol: symbol, Interval: interval}
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
		if len(clientState.keys) >= 8 {
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
		registration = &clientState{client: client, keys: make(map[binance.KlineKey]struct{})}
		service.clients[client.ID()] = registration
	}
	registration.keys[key] = struct{}{}
	message := snapshotMessage(key, state)
	service.mu.Unlock()
	if !client.Enqueue(Message{Kind: "subscribed", Key: key}) || !client.Enqueue(message) {
		service.RemoveClient(client.ID())
		client.Close()
	}
	service.logCounts("subscribe")
	return nil
}

func (service *Service) Unsubscribe(clientID string, key binance.KlineKey) {
	key.Symbol = strings.ToUpper(strings.TrimSpace(key.Symbol))
	service.mu.Lock()
	service.unsubscribeLocked(clientID, key)
	service.mu.Unlock()
	service.logCounts("unsubscribe")
}

func (service *Service) RemoveClient(clientID string) {
	service.mu.Lock()
	clientState := service.clients[clientID]
	var keys []binance.KlineKey
	if clientState != nil {
		keys = make([]binance.KlineKey, 0, len(clientState.keys))
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

func (service *Service) unsubscribeLocked(clientID string, key binance.KlineKey) {
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

func (service *Service) release(key binance.KlineKey, generation uint64) {
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

func (service *Service) apply(event binance.KlineEvent) {
	service.mu.Lock()
	state := service.states[event.Key]
	if state == nil {
		service.mu.Unlock()
		return
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
		if state.upstreamConnected {
			state.freshness = FreshnessFresh
		} else {
			state.freshness = FreshnessStale
		}
	}
	clients := clientSlice(state.clients)
	freshness := state.freshness
	service.mu.Unlock()
	service.publish(clients, Message{Kind: "update", Key: event.Key, Candle: &incoming, Freshness: freshness})
	if recoverNow {
		go service.recover(event.Key, generation)
	}
}

func (service *Service) applyStatus(status binance.StreamStatus) {
	service.mu.Lock()
	messages := make([]struct {
		clients []Client
		message Message
	}, 0, len(status.Keys))
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
		messages = append(messages, struct {
			clients []Client
			message Message
		}{clientSlice(state.clients), Message{Kind: "status", Key: key, Freshness: state.freshness, Reason: errorString(status.Err)}})
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

func (service *Service) recover(key binance.KlineKey, generation uint64) {
	delay := time.Duration(0)
	var progress recoveryProgress
	for {
		if delay > 0 {
			select {
			case <-service.context().Done():
				return
			case <-time.After(delay):
			}
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

		batchCtx, cancel := context.WithTimeout(service.context(), 5*time.Second)
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
			if state.upstreamConnected {
				state.freshness = FreshnessFresh
			} else {
				state.freshness = FreshnessStale
			}
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
			continue
		}
		if delay < time.Second {
			delay = time.Second
		} else if delay < 30*time.Second {
			delay *= 2
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}
		}
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

func (service *Service) context() context.Context {
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.ctx != nil {
		return service.ctx
	}
	return context.Background()
}

func (service *Service) publish(clients []Client, message Message) {
	for _, client := range clients {
		if !client.Enqueue(message) {
			client.Close()
			go service.RemoveClient(client.ID())
		}
	}
}

func snapshotMessage(key binance.KlineKey, state *keyState) Message {
	candles := make([]CandleState, 0, len(state.candles))
	for _, candle := range state.candles {
		candles = append(candles, candle)
	}
	sort.Slice(candles, func(i, j int) bool { return candles[i].Candle.OpenTime.Before(candles[j].Candle.OpenTime) })
	return Message{Kind: "snapshot", Key: key, Candles: candles, Freshness: state.freshness}
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
	sort.Slice(keys, func(i, j int) bool { return keys[i].Before(keys[j]) })
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
	service.states = make(map[binance.KlineKey]*keyState)
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
