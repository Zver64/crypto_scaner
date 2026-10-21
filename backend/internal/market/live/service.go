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
	"crypto-scanner/internal/platform/logging"

	"golang.org/x/time/rate"
)

const (
	defaultReleaseDelay   = 15 * time.Second
	maxRetainedCandles    = 16
	recoveryPagesPerBatch = 16
	recoveryQueryRate     = 20
	// recoveryQuietFailures are logged at INFO: the newest closed candle is
	// usually just not synchronized yet; of the later ones only the first
	// warns. Recovery ends after maxRecoveryFailures
	// consecutive failed batches; a batch that advances resets the count.
	recoveryQuietFailures = 4
	maxRecoveryFailures   = 8
	// maxClientSubscriptions bounds the live keys of one client connection.
	maxClientSubscriptions = 8
)

var (
	ErrInactiveSymbol             = fmt.Errorf("live candles: %w", market.ErrInstrumentNotFound)
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
	// Enqueue reports false when the client cannot keep up. It runs under the
	// service lock and must not close the client: the service removes it and
	// calls Close after unlocking.
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
	// releaseAt is when Run releases the key if it still has no clients; zero
	// while clients remain.
	releaseAt time.Time

	recovering         bool
	recoveryGeneration uint64
	recoveryStart      time.Time // inclusive
	recoveryThrough    time.Time // inclusive
	// recoveryRequested asks Run to start the recovery goroutine of the
	// current generation; set when a recovery starts outside Run's goroutine.
	recoveryRequested bool
	reconnectFrom     *time.Time
	// link is the upstream connectivity of the key. Only connections newer
	// than disconnectEpoch, the newest connection reported down, connect it,
	// so buffered events of a dead connection never do.
	link            upstreamLink
	disconnectEpoch int64
	// unrecoveredStart and unrecoveredThrough (inclusive) are the range of an
	// abandoned or parked recovery. The key stays stale until a later recovery,
	// started by the next gap or reconnect, covers it.
	unrecoveredStart   time.Time
	unrecoveredThrough time.Time
	// parked marks an unrecovered range left because the key had no clients;
	// the next subscription resumes it. An abandoned range is not parked.
	parked bool
}

type upstreamLink int

const (
	linkWaiting upstreamLink = iota
	linkConnected
	linkDisconnected
)

// freshness derives the freshness clients see from the key's state.
func (state *keyState) freshness() Freshness {
	switch {
	case state.link == linkDisconnected:
		return FreshnessStale
	case state.link == linkWaiting || len(state.candles) == 0:
		return FreshnessWaiting
	case state.recovering || state.reconnectFrom != nil:
		return FreshnessRecovering
	case !state.unrecoveredStart.IsZero():
		return FreshnessStale
	default:
		return FreshnessFresh
	}
}

// connectedBy marks the key connected by data or a status of connection
// epoch, unless a connection at least as new was reported down. A reconnect
// recovers from the latest buffered candle, whichever signal proved it.
func (state *keyState) connectedBy(interval market.CandleInterval, epoch int64) {
	if epoch <= state.disconnectEpoch {
		return
	}
	if state.link == linkDisconnected && len(state.candles) != 0 {
		latest := latestOpen(state.candles)
		from := latest
		if state.candles[latest].Final {
			from = interval.NextOpenTime(latest)
		}
		state.reconnectFrom = &from
	}
	state.link = linkConnected
}

// accepts reports whether incoming may enter the buffer.
func (state *keyState) accepts(incoming CandleState) bool {
	open := incoming.Candle.OpenTime
	if incoming.Final && !state.latestConfirmed.IsZero() && !open.After(state.latestConfirmed) {
		return false
	}
	current, exists := state.candles[open]
	if exists {
		// A final candle is never rolled back to an open one.
		return !current.Final || incoming.Final
	}
	// Old packets cannot re-enter the bounded buffer after their REST
	// confirmation has been evicted and replace persisted chart history.
	if len(state.candles) < maxRetainedCandles {
		return true
	}
	for buffered := range state.candles {
		if buffered.Before(open) {
			return true
		}
	}
	return false
}

func (state *keyState) endRecovery() {
	state.recovering = false
	state.recoveryRequested = false
	state.recoveryStart = time.Time{}
	state.recoveryThrough = time.Time{}
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
	// recoveryRetryDelay is the wait after the given number of consecutive
	// failed recovery batches.
	recoveryRetryDelay func(failures int) time.Duration

	// recoveries tracks recovery goroutines. They are started only from Run's
	// goroutine (via apply or startRequestedRecoveries) with Run's context, and
	// Run waits for them.
	recoveries sync.WaitGroup
	// releases wakes Run to reschedule its release timer; capacity 1 keeps the
	// send non-blocking, and a pending wake-up covers every later deadline.
	releases chan struct{}
	// recoveryRequests wakes Run to start requested recoveries; capacity 1
	// keeps the send non-blocking, and Run starts every requested one per wake-up.
	recoveryRequests chan struct{}

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
	return &Service{upstream: upstream, store: store, logger: logger, delay: delay, recoveryLimiter: rate.NewLimiter(rate.Limit(recoveryQueryRate), 4), recoveryRetryDelay: defaultRecoveryRetryDelay, releases: make(chan struct{}, 1), recoveryRequests: make(chan struct{}, 1), states: make(map[kline.Key]*keyState), clients: make(map[string]*clientState)}
}

func defaultRecoveryRetryDelay(failures int) time.Duration {
	return backoff.Exponential(time.Second, 30*time.Second, failures-1)
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
	releaseTimer := time.NewTimer(0)
	defer releaseTimer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case event := <-service.upstream.Events():
			service.apply(ctx, event)
		case status := <-service.upstream.Statuses():
			service.applyStatus(status)
		case <-service.releases:
			service.releaseDue(releaseTimer)
		case <-service.recoveryRequests:
			service.startRequestedRecoveries(ctx)
		case <-releaseTimer.C:
			service.releaseDue(releaseTimer)
		}
	}
}

// startRequestedRecoveries starts the recoveries requested outside Run's
// goroutine; ctx bounds them.
func (service *Service) startRequestedRecoveries(ctx context.Context) {
	service.mu.Lock()
	defer service.mu.Unlock()
	for key, state := range service.states {
		if !state.recoveryRequested {
			continue
		}
		state.recoveryRequested = false
		if state.recovering {
			generation := state.recoveryGeneration
			service.recoveries.Go(func() { service.recover(ctx, key, generation) })
		}
	}
}

// releaseDue releases keys whose release delay passed without clients and
// sets timer to the next pending release.
func (service *Service) releaseDue(timer *time.Timer) {
	service.mu.Lock()
	now := time.Now()
	var next time.Time
	released := false
	for key, state := range service.states {
		switch {
		case state.releaseAt.IsZero() || len(state.clients) != 0:
		case !state.releaseAt.After(now):
			delete(service.states, key)
			service.upstream.Unsubscribe(key)
			released = true
		case next.IsZero() || state.releaseAt.Before(next):
			next = state.releaseAt
		}
	}
	service.mu.Unlock()
	timer.Stop()
	if !next.IsZero() {
		timer.Reset(time.Until(next))
	}
	if released {
		service.logCounts("release")
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
		return fmt.Errorf("live candles: unsupported candle interval %q", interval)
	}
	key := kline.Key{Symbol: symbol, Interval: interval}
	service.mu.Lock()
	if done, err := service.checkSubscriptionLocked(client.ID(), key); done {
		service.mu.Unlock()
		return err
	}
	state := service.states[key]
	if state == nil {
		service.mu.Unlock()
		instrument, err := service.store.GetActiveInstrumentBySymbol(ctx, symbol)
		if err != nil {
			if errors.Is(err, market.ErrInstrumentNotFound) {
				return ErrInactiveSymbol
			}
			return fmt.Errorf("look up active instrument: %w", err)
		}
		service.mu.Lock()
		// The lookup ran unlocked: recheck everything it may have changed,
		// including a state created or released meanwhile.
		if done, err := service.checkSubscriptionLocked(client.ID(), key); done {
			service.mu.Unlock()
			return err
		}
		state = service.states[key]
		if state == nil {
			state = &keyState{instrument: instrument, clients: make(map[string]Client), candles: make(map[time.Time]CandleState)}
			service.states[key] = state
			service.upstream.Subscribe(key)
		}
	}
	state.releaseAt = time.Time{}
	state.clients[client.ID()] = client
	registration := service.clients[client.ID()]
	if registration == nil {
		registration = &clientState{client: client, keys: make(map[kline.Key]struct{})}
		service.clients[client.ID()] = registration
	}
	registration.keys[key] = struct{}{}
	// A recovery parked while the key had no clients resumes.
	if state.parked && !state.recovering {
		service.requestRecoveryLocked(state)
	}
	var dropped []Client
	if !client.Enqueue(Message{Kind: KindSubscribed, Key: key}) || !client.Enqueue(snapshotMessage(key, state)) {
		service.removeClientLocked(client.ID())
		dropped = append(dropped, client)
	}
	service.mu.Unlock()
	service.closeClients(dropped)
	service.logCounts("subscribe")
	return nil
}

// checkSubscriptionLocked reports whether Subscribe is done before attaching
// the client, with its result; service.mu must be held.
func (service *Service) checkSubscriptionLocked(clientID string, key kline.Key) (bool, error) {
	if service.stopped {
		return true, ErrServiceStopped
	}
	if clientState := service.clients[clientID]; clientState != nil {
		if _, exists := clientState.keys[key]; exists {
			return true, nil
		}
		if len(clientState.keys) >= maxClientSubscriptions {
			return true, ErrTooManyClientSubscriptions
		}
	}
	return false, nil
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
	service.removeClientLocked(clientID)
	service.mu.Unlock()
	service.logCounts("disconnect")
}

func (service *Service) removeClientLocked(clientID string) {
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
	state.releaseAt = time.Now().Add(service.delay)
	select {
	case service.releases <- struct{}{}:
	default:
	}
}

// apply runs on Run's goroutine; ctx bounds any recovery it starts.
func (service *Service) apply(ctx context.Context, event kline.Event) {
	service.mu.Lock()
	state := service.states[event.Key]
	if state == nil {
		service.mu.Unlock()
		return
	}
	// Data proves its connection alive, even if the packet itself is dropped;
	// a dropped packet still publishes the connectivity it changed.
	before := state.freshness()
	state.connectedBy(event.Key.Interval, event.Epoch)
	incoming := CandleState{Candle: event.Candle, Final: event.Final}
	if !state.accepts(incoming) {
		var dropped []Client
		if freshness := state.freshness(); freshness != before {
			dropped = service.publishLocked(nil, state.clients, Message{Kind: KindStatus, Key: event.Key, Freshness: freshness})
		}
		service.mu.Unlock()
		service.closeClients(dropped)
		return
	}
	previousLatest := latestOpen(state.candles)
	previous, hadPrevious := state.candles[previousLatest]
	state.candles[event.Candle.OpenTime] = incoming
	// Evicted candles need no recovery: their corrections arrive through
	// HistoryChanged.
	trimCandles(state.candles)

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
	if recoverNow {
		// This goroutine serves the current generation, also a requested one.
		state.recoveryRequested = false
		service.recoveries.Go(func() { service.recover(ctx, event.Key, generation) })
	}
	dropped := service.publishLocked(nil, state.clients, Message{Kind: KindUpdate, Key: event.Key, Candle: &incoming, Freshness: state.freshness()})
	service.mu.Unlock()
	service.closeClients(dropped)
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
	var dropped []Client
	for key := range changed {
		state := service.states[key]
		dropped = service.publishLocked(dropped, state.clients, snapshotMessage(key, state))
	}
	service.mu.Unlock()
	service.closeClients(dropped)
}

func (service *Service) applyStatus(status kline.Status) {
	service.mu.Lock()
	var dropped []Client
	for _, key := range status.Keys {
		state := service.states[key]
		if state == nil {
			continue
		}
		if !status.Connected {
			state.link = linkDisconnected
			state.disconnectEpoch = max(state.disconnectEpoch, status.Epoch)
		} else {
			state.connectedBy(key.Interval, status.Epoch)
		}
		dropped = service.publishLocked(dropped, state.clients, Message{Kind: KindStatus, Key: key, Freshness: state.freshness(), Reason: errorString(status.Err)})
	}
	service.mu.Unlock()
	service.closeClients(dropped)
}

func (service *Service) startRecoveryLocked(state *keyState, start, through time.Time) (uint64, bool) {
	if start.IsZero() || through.Before(start) {
		return 0, false
	}
	// A new recovery also retries the range an abandoned one left behind.
	if !state.unrecoveredStart.IsZero() {
		start = minTime(start, state.unrecoveredStart)
		through = maxTime(through, state.unrecoveredThrough)
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
	state.parked = false
	state.recoveryStart = start
	state.recoveryThrough = through
	state.recoveryGeneration++
	return state.recoveryGeneration, true
}

// requestRecoveryLocked starts a recovery of the parked range outside
// Run's goroutine: Run starts its goroutine. service.mu must be held.
func (service *Service) requestRecoveryLocked(state *keyState) {
	if _, ok := service.startRecoveryLocked(state, state.unrecoveredStart, state.unrecoveredThrough); !ok {
		return
	}
	state.recoveryRequested = true
	select {
	case service.recoveryRequests <- struct{}{}:
	default:
	}
}

func (service *Service) recover(ctx context.Context, key kline.Key, generation uint64) {
	delay := time.Duration(0)
	failures := 0
	// incomplete warns once while batches past the quiet ones keep failing.
	incomplete := logging.NewFailure(service.logger)
	var progress recoveryProgress
	for {
		if delay > 0 && backoff.Sleep(ctx, delay) != nil {
			return
		}
		service.mu.Lock()
		state := service.states[key]
		if state == nil || !state.recovering || state.recoveryGeneration != generation {
			service.mu.Unlock()
			return
		}
		if len(state.clients) == 0 {
			// Park the range; the next subscription resumes it.
			state.unrecoveredStart, state.unrecoveredThrough = state.recoveryStart, state.recoveryThrough
			state.parked = true
			state.endRecovery()
			service.mu.Unlock()
			return
		}
		instrument, start, through := state.instrument, state.recoveryStart, state.recoveryThrough
		service.mu.Unlock()
		if progress.expected.IsZero() {
			progress = newRecoveryProgress(through)
		}

		recentBefore, cursorBefore := len(progress.recent), progress.before
		batchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		complete, stalled, err := service.loadRecoveryBatch(batchCtx, instrument.ID, key.Interval, start, &progress)
		cancel()
		// A batch that moved the cursor or found candles made progress, even if
		// it then failed: only consecutive fruitless batches count toward giving up.
		if !stalled && (len(progress.recent) != recentBefore || progress.before != cursorBefore) {
			failures = 0
			incomplete.Forget()
		}
		service.mu.Lock()
		state = service.states[key]
		if state == nil || !state.recovering || state.recoveryGeneration != generation {
			service.mu.Unlock()
			return
		}
		failed := !complete && (err != nil || stalled)
		if failed {
			failures++
		}
		// Recovery gives up with what it found rather than retrying forever.
		// The found candles are REST-confirmed, but a hole may remain, so the
		// key stays stale until the next gap or reconnect recovers the range.
		abandoned := failures >= maxRecoveryFailures
		if complete || abandoned {
			for _, candle := range progress.recent {
				current, exists := state.candles[candle.OpenTime]
				if !exists || !current.Final {
					state.candles[candle.OpenTime] = CandleState{Candle: candle, Final: true}
				}
			}
			trimCandles(state.candles)
			if complete {
				state.unrecoveredStart, state.unrecoveredThrough = time.Time{}, time.Time{}
			} else {
				state.unrecoveredStart, state.unrecoveredThrough = state.recoveryStart, state.recoveryThrough
			}
			state.endRecovery()
		}
		dropped := service.publishLocked(nil, state.clients, snapshotMessage(key, state))
		service.mu.Unlock()
		service.closeClients(dropped)
		if complete {
			return
		}
		if abandoned {
			service.logger.Warn("live candle history recovery abandoned", "module", "market_live", "operation", "recover", "symbol", key.Symbol, "interval", key.Interval, "attempts", failures, "error", errorString(err))
			return
		}
		if !failed {
			// The cursor is retained across bounded batches, while the process-wide
			// limiter prevents many recovering streams from overwhelming PostgreSQL.
			delay = 100 * time.Millisecond
			continue
		}
		if stalled {
			progress = newRecoveryProgress(through)
		}
		delay = service.recoveryRetryDelay(failures)
		args := []any{"module", "market_live", "operation", "recover", "symbol", key.Symbol, "interval", key.Interval, "attempt", failures, "error", errorString(err), "retry_in", delay}
		if failures > recoveryQuietFailures {
			incomplete.Warn(ctx, "live candle history recovery incomplete", args...)
		} else {
			service.logger.InfoContext(ctx, "live candle history recovery incomplete", args...)
		}
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
				// The newest closed candle is not synchronized yet.
				if len(progress.recent) == 0 {
					return false, true, nil
				}
				// Below a synchronized candle, a hole is a period without
				// exchange data: the next older candle is the expected one.
				if candle.OpenTime.Before(start) {
					return true, false, nil
				}
				progress.expected = candle.OpenTime
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
			// Stored history ends; it is complete if it reached the newest candle.
			return len(progress.recent) > 0, len(progress.recent) == 0, nil
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

// publishLocked enqueues message to clients while service.mu is held, so each
// client receives the messages of a key in the order of its state changes.
// Clients that cannot keep up are removed and appended to dropped, which the
// caller closes with closeClients after unlocking.
func (service *Service) publishLocked(dropped []Client, clients map[string]Client, message Message) []Client {
	for id, client := range clients {
		if !client.Enqueue(message) {
			service.removeClientLocked(id)
			dropped = append(dropped, client)
		}
	}
	return dropped
}

func (service *Service) closeClients(clients []Client) {
	for _, client := range clients {
		client.Close()
	}
	if len(clients) != 0 {
		service.logCounts("disconnect")
	}
}

func snapshotMessage(key kline.Key, state *keyState) Message {
	candles := make([]CandleState, 0, len(state.candles))
	for _, candle := range state.candles {
		candles = append(candles, candle)
	}
	slices.SortFunc(candles, func(left, right CandleState) int { return left.Candle.OpenTime.Compare(right.Candle.OpenTime) })
	return Message{Kind: KindSnapshot, Key: key, Candles: candles, Freshness: state.freshness()}
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

func trimCandles(values map[time.Time]CandleState) {
	if len(values) <= maxRetainedCandles {
		return
	}
	keys := make([]time.Time, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, time.Time.Compare)
	for _, key := range keys[:len(keys)-maxRetainedCandles] {
		delete(values, key)
	}
}

func minTime(left, right time.Time) time.Time {
	if right.Before(left) {
		return right
	}
	return left
}

func maxTime(left, right time.Time) time.Time {
	if right.After(left) {
		return right
	}
	return left
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
	for key := range service.states {
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
