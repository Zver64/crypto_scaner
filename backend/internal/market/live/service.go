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
	"crypto-scanner/internal/platform/logging"

	"golang.org/x/time/rate"
)

const (
	defaultReleaseDelay = 15 * time.Second
	maxRetainedCandles  = 16
	// readQueryRate bounds the stored-candle reads of all recovering keys.
	readQueryRate = 20
	// readTimeout bounds one stored-candle read.
	readTimeout = 5 * time.Second
	// maxClientSubscriptions bounds the live keys of one client connection.
	maxClientSubscriptions = 8
	// seedTimeout bounds reading the forming candle of a new subscription.
	seedTimeout = 5 * time.Second
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

// CurrentCandles reads the newest candle of a symbol from the exchange.
type CurrentCandles interface {
	CurrentCandle(context.Context, string, market.CandleInterval) (market.Candle, error)
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

type Options struct {
	ReleaseDelay time.Duration
}

type Service struct {
	upstream    kline.Feed
	current     CurrentCandles
	store       HistoryStore
	logger      *slog.Logger
	delay       time.Duration
	readLimiter *rate.Limiter
	// readFailure warns once while stored-candle reads keep failing.
	readFailure *logging.Failure

	// workers tracks read and seed goroutines. They are started only from
	// Run's goroutine (via apply or startRequested) with Run's context, and Run
	// waits for them.
	workers sync.WaitGroup
	// releases wakes Run to reschedule its release timer; capacity 1 keeps the
	// send non-blocking, and a pending wake-up covers every later deadline.
	releases chan struct{}
	// requests wakes Run to start requested seeds; capacity 1 keeps the send
	// non-blocking, and Run starts every requested one per wake-up.
	requests chan struct{}

	mu      sync.Mutex
	states  map[kline.Key]*keyState
	clients map[string]*clientState
	running bool
	stopped bool
}

func New(upstream kline.Feed, current CurrentCandles, store HistoryStore, logger *slog.Logger, options Options) *Service {
	delay := options.ReleaseDelay
	if delay <= 0 {
		delay = defaultReleaseDelay
	}
	return &Service{
		upstream: upstream, current: current, store: store, logger: logger, delay: delay,
		readLimiter: rate.NewLimiter(rate.Limit(readQueryRate), 4), readFailure: logging.NewFailure(logger),
		releases: make(chan struct{}, 1), requests: make(chan struct{}, 1),
		states: make(map[kline.Key]*keyState), clients: make(map[string]*clientState),
	}
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
		service.workers.Wait()
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
		case <-service.upstream.StatusesReady():
			for _, status := range service.upstream.DrainStatuses() {
				service.applyStatus(status)
			}
		case <-service.releases:
			service.releaseDue(releaseTimer)
		case <-service.requests:
			service.startRequested(ctx)
		case <-releaseTimer.C:
			service.releaseDue(releaseTimer)
		}
	}
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

func (service *Service) logCounts(operation string) {
	service.mu.Lock()
	connections, subscriptions := len(service.clients), len(service.states)
	service.mu.Unlock()
	service.logger.Info("live candle subscriptions changed", "module", "market_live", "operation", operation, "clients", connections, "subscriptions", subscriptions)
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
