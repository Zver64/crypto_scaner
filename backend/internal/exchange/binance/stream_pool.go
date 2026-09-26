package binance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"crypto-scanner/internal/platform/backoff"

	"github.com/gorilla/websocket"
	"golang.org/x/time/rate"
)

const (
	defaultStreamURL        = "wss://stream.binance.com:9443/ws"
	maxStreamsPerConnection = 1024
	maxStreamsPerCommand    = 100
)

var errStreamWorkerIdle = errors.New("stream worker has no subscriptions")

// NewDialLimiter returns the process-wide Binance WebSocket connection budget.
// Every stream of one process must share it.
func NewDialLimiter() *rate.Limiter {
	return rate.NewLimiter(rate.Every(1200*time.Millisecond), 5)
}

// streamKind describes one Binance stream family served by a streamPool.
type streamKind[K comparable] struct {
	label  string // "kline" or "trade"
	module string
	event  string // envelope "e" value of data messages
	name   func(K) string
	// handle decodes one data message of connection epoch and delivers it.
	handle func(ctx context.Context, payload []byte, epoch int64) error
	// status reports the keys of one connection going up or down.
	status func(keys []K, connected bool, err error) bool
}

// streamPool is a dynamically subscribed pool of Binance connections. Each key
// belongs to exactly one worker, and a worker owns one connection.
type streamPool[K comparable] struct {
	url         string
	dialer      *websocket.Dialer
	logger      *slog.Logger
	dialLimiter *rate.Limiter
	kind        streamKind[K]
	epoch       atomic.Int64

	// added wakes Run when a worker is appended; capacity 1 keeps the send
	// non-blocking under mu, and Run starts every pending worker per wake-up.
	added chan struct{}

	mu      sync.Mutex
	workers []*streamWorker[K]
	started int // workers[:started] run under the current Run's context
	running bool
}

func newStreamPool[K comparable](url string, dialer *websocket.Dialer, logger *slog.Logger, dialLimiter *rate.Limiter, kind streamKind[K]) *streamPool[K] {
	return &streamPool[K]{url: url, dialer: dialer, logger: logger, dialLimiter: dialLimiter, kind: kind, added: make(chan struct{}, 1)}
}

// Run owns all upstream connections until ctx is cancelled. Workers are only
// ever started here, with Run's own context; workers added while Run is active
// are handed over through pool.added.
func (pool *streamPool[K]) Run(ctx context.Context) error {
	pool.mu.Lock()
	if pool.running {
		pool.mu.Unlock()
		return fmt.Errorf("%s stream already running", pool.kind.label)
	}
	pool.running = true
	pool.startPendingLocked(ctx)
	pool.mu.Unlock()
	for {
		select {
		case <-ctx.Done():
			pool.mu.Lock()
			pool.running, pool.started = false, 0
			pool.mu.Unlock()
			return nil
		case <-pool.added:
			pool.mu.Lock()
			pool.startPendingLocked(ctx)
			pool.mu.Unlock()
		}
	}
}

// startPendingLocked starts workers not yet running; pool.mu must be held.
func (pool *streamPool[K]) startPendingLocked(ctx context.Context) {
	for _, worker := range pool.workers[pool.started:] {
		go worker.run(ctx)
	}
	pool.started = len(pool.workers)
}

// addWorkerLocked appends a new worker; pool.mu must be held. It never blocks:
// a pending wake-up already covers every worker appended before Run handles it.
func (pool *streamPool[K]) addWorkerLocked() *streamWorker[K] {
	worker := &streamWorker[K]{pool: pool, changes: make(chan struct{}, 1), desired: map[K]struct{}{}}
	pool.workers = append(pool.workers, worker)
	select {
	case pool.added <- struct{}{}:
	default:
	}
	return worker
}

type streamWorker[K comparable] struct {
	pool    *streamPool[K]
	changes chan struct{}
	request atomic.Int64

	mu      sync.Mutex
	desired map[K]struct{}
}

func (worker *streamWorker[K]) has(key K) bool {
	worker.mu.Lock()
	defer worker.mu.Unlock()
	_, ok := worker.desired[key]
	return ok
}

func (worker *streamWorker[K]) count() int {
	worker.mu.Lock()
	defer worker.mu.Unlock()
	return len(worker.desired)
}

func (worker *streamWorker[K]) update(change func(map[K]struct{})) {
	worker.mu.Lock()
	change(worker.desired)
	worker.mu.Unlock()
	select {
	case worker.changes <- struct{}{}:
	default:
	}
}

// keys returns the desired keys ordered by stream name.
func (worker *streamWorker[K]) keys() []K {
	worker.mu.Lock()
	defer worker.mu.Unlock()
	keys := make([]K, 0, len(worker.desired))
	for key := range worker.desired {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(left, right K) int {
		return strings.Compare(worker.pool.kind.name(left), worker.pool.kind.name(right))
	})
	return keys
}

func (worker *streamWorker[K]) publishStatus(connected bool, err error) {
	if !worker.pool.kind.status(worker.keys(), connected, err) {
		worker.pool.logger.Warn("Binance status queue full", "module", worker.pool.kind.module, "operation", "status")
	}
}

// run keeps one connection open while the worker has subscriptions.
func (worker *streamWorker[K]) run(ctx context.Context) {
	failures := 0
	for ctx.Err() == nil {
		if worker.count() == 0 {
			select {
			case <-ctx.Done():
				return
			case <-worker.changes:
				continue
			}
		}
		connectedAt := time.Now()
		err := worker.connect(ctx)
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, errStreamWorkerIdle) || time.Since(connectedAt) >= time.Minute {
			failures = 0
		}
		if errors.Is(err, errStreamWorkerIdle) {
			continue
		}
		worker.publishStatus(false, err)
		worker.pool.logger.Warn("Binance WebSocket reconnecting", "module", worker.pool.kind.module, "operation", "reconnect", "stream", worker.pool.kind.label, "error", err)
		if backoff.Sleep(ctx, backoff.Jitter(backoff.Exponential(time.Second, 30*time.Second, failures), 0.5)) != nil {
			return
		}
		failures++
	}
}

func (worker *streamWorker[K]) connect(ctx context.Context) error {
	pool := worker.pool
	conn, err := dialBinanceStream(ctx, pool.url, pool.dialer, pool.dialLimiter, pool.kind.label)
	if err != nil {
		return err
	}
	defer conn.Close()

	epoch := pool.epoch.Add(1)
	readResult := make(chan error, 1)
	acks := make(chan controlReply, 16)
	go func() { readResult <- worker.readLoop(ctx, conn, acks, epoch) }()
	active := make(map[K]struct{})
	limiter := rate.NewLimiter(rate.Every(250*time.Millisecond), 1) // reserve capacity below Binance's 5 msg/s limit.
	rotation := time.NewTimer(streamRotation)
	defer rotation.Stop()
	if err := worker.reconcile(ctx, conn, limiter, active, acks); err != nil {
		return err
	}
	if len(active) == 0 {
		return errStreamWorkerIdle
	}
	worker.publishStatus(true, nil)
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-readResult:
			return err
		case <-rotation.C:
			return fmt.Errorf("scheduled Binance %s connection rotation", pool.kind.label)
		case <-worker.changes:
			if err := worker.reconcile(ctx, conn, limiter, active, acks); err != nil {
				return err
			}
			if len(active) == 0 {
				return errStreamWorkerIdle
			}
		}
	}
}

func (worker *streamWorker[K]) reconcile(ctx context.Context, conn *websocket.Conn, limiter *rate.Limiter, active map[K]struct{}, acks <-chan controlReply) error {
	desired := worker.keys()
	desiredSet := make(map[K]struct{}, len(desired))
	var add, remove []string
	for _, key := range desired {
		desiredSet[key] = struct{}{}
		if _, ok := active[key]; !ok {
			add = append(add, worker.pool.kind.name(key))
		}
	}
	for key := range active {
		if _, ok := desiredSet[key]; !ok {
			remove = append(remove, worker.pool.kind.name(key))
		}
	}
	slices.Sort(remove)
	if err := controlBinanceStreams(ctx, conn, limiter, &worker.request, worker.pool.kind.label, "UNSUBSCRIBE", remove, acks); err != nil {
		return err
	}
	if err := controlBinanceStreams(ctx, conn, limiter, &worker.request, worker.pool.kind.label, "SUBSCRIBE", add, acks); err != nil {
		return err
	}
	clear(active)
	for key := range desiredSet {
		active[key] = struct{}{}
	}
	return nil
}

func (worker *streamWorker[K]) readLoop(ctx context.Context, conn *websocket.Conn, acks chan<- controlReply, epoch int64) error {
	label := worker.pool.kind.label
	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read Binance %s stream: %w", label, err)
		}
		// encoding/json matches keys case-insensitively, so every key that
		// differs only in case from a decoded one must have its own field.
		var envelope struct {
			ID        *int64          `json:"id"`
			Result    json.RawMessage `json:"result"`
			Code      *int            `json:"code"`
			Msg       string          `json:"msg"`
			Event     string          `json:"e"`
			EventTime json.RawMessage `json:"E"`
		}
		if err := json.Unmarshal(payload, &envelope); err != nil {
			return fmt.Errorf("decode Binance %s stream: %w", label, err)
		}
		if envelope.ID != nil {
			select {
			case acks <- controlReply{ID: *envelope.ID, Result: envelope.Result, Code: envelope.Code, Msg: envelope.Msg}:
			case <-ctx.Done():
				return nil
			}
			continue
		}
		if envelope.Event != worker.pool.kind.event {
			continue
		}
		if err := worker.pool.kind.handle(ctx, payload, epoch); err != nil {
			return err
		}
	}
}
