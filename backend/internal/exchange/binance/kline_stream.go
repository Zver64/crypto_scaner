package binance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"crypto-scanner/internal/market"

	"github.com/gorilla/websocket"
	"golang.org/x/time/rate"
)

const (
	defaultStreamURL        = "wss://stream.binance.com:9443/ws"
	maxStreamsPerConnection = 1024
	maxStreamsPerCommand    = 100
)

var errStreamWorkerIdle = errors.New("stream worker has no subscriptions")

// KlineKey identifies one Binance Spot kline stream.
type KlineKey struct {
	Symbol   string
	Interval market.CandleInterval
}

func (key KlineKey) StreamName() string {
	return strings.ToLower(key.Symbol) + "@kline_" + string(key.Interval)
}

// KlineEvent is a complete exchange snapshot of one forming or final candle.
type KlineEvent struct {
	Key       KlineKey
	Candle    market.Candle
	Final     bool
	EventTime time.Time
}

// StreamStatus reports freshness for the keys owned by one upstream connection.
type StreamStatus struct {
	Keys      []KlineKey
	Connected bool
	Err       error
}

// KlineStream is a shared, dynamically subscribed Binance connection pool.
type KlineStream struct {
	url         string
	dialer      *websocket.Dialer
	logger      *slog.Logger
	events      chan KlineEvent
	statuses    chan StreamStatus
	dialLimiter *rate.Limiter

	mu      sync.Mutex
	workers []*streamWorker
	running bool
	ctx     context.Context
}

func NewKlineStream(logger *slog.Logger) *KlineStream {
	return newKlineStream(defaultStreamURL, websocket.DefaultDialer, logger)
}

func newKlineStream(url string, dialer *websocket.Dialer, logger *slog.Logger) *KlineStream {
	return &KlineStream{
		url: url, dialer: dialer, logger: logger,
		events: make(chan KlineEvent, 256), statuses: make(chan StreamStatus, 32),
		dialLimiter: sharedDialLimiter,
	}
}

func (stream *KlineStream) Events() <-chan KlineEvent     { return stream.events }
func (stream *KlineStream) Statuses() <-chan StreamStatus { return stream.statuses }

// Run owns all upstream connections until ctx is cancelled.
func (stream *KlineStream) Run(ctx context.Context) error {
	stream.mu.Lock()
	if stream.running {
		stream.mu.Unlock()
		return errors.New("kline stream already running")
	}
	stream.running, stream.ctx = true, ctx
	workers := append([]*streamWorker(nil), stream.workers...)
	stream.mu.Unlock()
	for _, worker := range workers {
		go worker.run(ctx)
	}
	<-ctx.Done()
	stream.mu.Lock()
	stream.running = false
	stream.mu.Unlock()
	return nil
}

func (stream *KlineStream) Subscribe(key KlineKey) error {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	for _, worker := range stream.workers {
		if worker.has(key) {
			return nil
		}
	}
	var worker *streamWorker
	for _, candidate := range stream.workers {
		if candidate.count() < maxStreamsPerConnection {
			worker = candidate
			break
		}
	}
	if worker == nil {
		worker = newStreamWorker(stream.url, stream.dialer, stream.logger, stream.events, stream.statuses, stream.dialLimiter)
		stream.workers = append(stream.workers, worker)
		if stream.running {
			go worker.run(stream.ctx)
		}
	}
	worker.subscribe(key)
	return nil
}

func (stream *KlineStream) Unsubscribe(key KlineKey) {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	for _, worker := range stream.workers {
		if worker.has(key) {
			worker.unsubscribe(key)
			return
		}
	}
}

type streamWorker struct {
	url         string
	dialer      *websocket.Dialer
	logger      *slog.Logger
	events      chan<- KlineEvent
	statuses    chan<- StreamStatus
	changes     chan struct{}
	mu          sync.Mutex
	desired     map[KlineKey]struct{}
	request     atomic.Int64
	dialLimiter *rate.Limiter
}

func newStreamWorker(url string, dialer *websocket.Dialer, logger *slog.Logger, events chan<- KlineEvent, statuses chan<- StreamStatus, dialLimiter *rate.Limiter) *streamWorker {
	return &streamWorker{url: url, dialer: dialer, logger: logger, events: events, statuses: statuses, changes: make(chan struct{}, 1), desired: make(map[KlineKey]struct{}), dialLimiter: dialLimiter}
}

func (worker *streamWorker) has(key KlineKey) bool {
	worker.mu.Lock()
	defer worker.mu.Unlock()
	_, ok := worker.desired[key]
	return ok
}
func (worker *streamWorker) count() int {
	worker.mu.Lock()
	defer worker.mu.Unlock()
	return len(worker.desired)
}
func (worker *streamWorker) subscribe(key KlineKey) {
	worker.mu.Lock()
	worker.desired[key] = struct{}{}
	worker.mu.Unlock()
	worker.signal()
}
func (worker *streamWorker) unsubscribe(key KlineKey) {
	worker.mu.Lock()
	delete(worker.desired, key)
	worker.mu.Unlock()
	worker.signal()
}
func (worker *streamWorker) signal() {
	select {
	case worker.changes <- struct{}{}:
	default:
	}
}
func (worker *streamWorker) keys() []KlineKey {
	worker.mu.Lock()
	defer worker.mu.Unlock()
	keys := make([]KlineKey, 0, len(worker.desired))
	for key := range worker.desired {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].StreamName() < keys[j].StreamName() })
	return keys
}

func (worker *streamWorker) run(ctx context.Context) {
	runDynamicStreamWorker(ctx, func() bool { return len(worker.keys()) > 0 }, worker.changes, worker.connect,
		func(err error) { worker.publishStatus(false, err) }, worker.logger, "binance_live", "kline")
}

func (worker *streamWorker) connect(ctx context.Context) error {
	conn, err := dialBinanceStream(ctx, worker.url, worker.dialer, worker.dialLimiter, "kline")
	if err != nil {
		return err
	}
	defer conn.Close()

	readResult := make(chan error, 1)
	acks := make(chan controlReply, 16)
	go func() { readResult <- worker.readLoop(ctx, conn, acks) }()
	active := make(map[KlineKey]struct{})
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
			return errors.New("scheduled Binance connection rotation")
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

type wireDecimal string

func (value *wireDecimal) UnmarshalJSON(data []byte) error {
	var raw string
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
	} else {
		raw = string(data)
		if _, err := strconv.ParseFloat(raw, 64); err != nil {
			return err
		}
	}
	*value = wireDecimal(raw)
	return nil
}

type wireKline struct {
	OpenTime    int64       `json:"t"`
	CloseTime   int64       `json:"T"`
	Symbol      string      `json:"s"`
	Interval    string      `json:"i"`
	Open        wireDecimal `json:"o"`
	Close       wireDecimal `json:"c"`
	High        wireDecimal `json:"h"`
	Low         wireDecimal `json:"l"`
	Volume      wireDecimal `json:"v"`
	QuoteVolume wireDecimal `json:"q"`
	Trades      int64       `json:"n"`
	Final       bool        `json:"x"`
}

func (worker *streamWorker) reconcile(ctx context.Context, conn *websocket.Conn, limiter *rate.Limiter, active map[KlineKey]struct{}, acks <-chan controlReply) error {
	desired := worker.keys()
	desiredSet := make(map[KlineKey]struct{}, len(desired))
	for _, key := range desired {
		desiredSet[key] = struct{}{}
	}
	var add, remove []KlineKey
	for _, key := range desired {
		if _, ok := active[key]; !ok {
			add = append(add, key)
		}
	}
	for key := range active {
		if _, ok := desiredSet[key]; !ok {
			remove = append(remove, key)
		}
	}
	if err := worker.control(ctx, conn, limiter, "UNSUBSCRIBE", remove, acks); err != nil {
		return err
	}
	if err := worker.control(ctx, conn, limiter, "SUBSCRIBE", add, acks); err != nil {
		return err
	}
	clear(active)
	for key := range desiredSet {
		active[key] = struct{}{}
	}
	return nil
}

func (worker *streamWorker) control(ctx context.Context, conn *websocket.Conn, limiter *rate.Limiter, method string, keys []KlineKey, acks <-chan controlReply) error {
	names := make([]string, len(keys))
	for index, key := range keys {
		names[index] = key.StreamName()
	}
	return controlBinanceStreams(ctx, conn, limiter, &worker.request, "kline", method, names, acks)
}

func (worker *streamWorker) readLoop(ctx context.Context, conn *websocket.Conn, acks chan<- controlReply) error {
	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read Binance stream: %w", err)
		}
		var envelope struct {
			ID        *int64          `json:"id"`
			Result    json.RawMessage `json:"result"`
			Code      *int            `json:"code"`
			Msg       string          `json:"msg"`
			Event     string          `json:"e"`
			EventTime int64           `json:"E"`
			Symbol    string          `json:"s"`
			Kline     wireKline       `json:"k"`
		}
		if err := json.Unmarshal(payload, &envelope); err != nil {
			return fmt.Errorf("decode Binance stream: %w", err)
		}
		if envelope.ID != nil {
			select {
			case acks <- controlReply{ID: *envelope.ID, Result: envelope.Result, Code: envelope.Code, Msg: envelope.Msg}:
			case <-ctx.Done():
				return nil
			}
			continue
		}
		if envelope.Event != "kline" {
			continue
		}
		event, err := decodeKline(envelope.EventTime, envelope.Kline)
		if err != nil {
			return err
		}
		select {
		case worker.events <- event:
		case <-ctx.Done():
			return nil
		}
	}
}

func decodeKline(eventTime int64, value wireKline) (KlineEvent, error) {
	interval := market.CandleInterval(value.Interval)
	if !interval.Valid() || value.Symbol == "" || value.OpenTime <= 0 || value.CloseTime <= value.OpenTime || value.Trades < 0 {
		return KlineEvent{}, errors.New("invalid Binance kline metadata")
	}
	parse := func(name, raw string) (float64, error) {
		number, err := strconv.ParseFloat(raw, 64)
		if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
			return 0, fmt.Errorf("decode Binance kline %s: invalid number", name)
		}
		return number, nil
	}
	open, err := parse("open", string(value.Open))
	if err != nil {
		return KlineEvent{}, err
	}
	high, err := parse("high", string(value.High))
	if err != nil {
		return KlineEvent{}, err
	}
	low, err := parse("low", string(value.Low))
	if err != nil {
		return KlineEvent{}, err
	}
	closePrice, err := parse("close", string(value.Close))
	if err != nil {
		return KlineEvent{}, err
	}
	volume, err := parse("volume", string(value.Volume))
	if err != nil {
		return KlineEvent{}, err
	}
	quote, err := parse("quote volume", string(value.QuoteVolume))
	if err != nil {
		return KlineEvent{}, err
	}
	if high < max(open, closePrice) || low > min(open, closePrice) || volume < 0 || quote < 0 {
		return KlineEvent{}, errors.New("invalid Binance kline values")
	}
	return KlineEvent{Key: KlineKey{Symbol: strings.ToUpper(value.Symbol), Interval: interval}, Candle: market.Candle{Interval: interval, OpenTime: time.UnixMilli(value.OpenTime).UTC(), CloseTime: time.UnixMilli(value.CloseTime).UTC(), Open: open, High: high, Low: low, Close: closePrice, Volume: volume, QuoteAssetVolume: quote, TradeCount: value.Trades}, Final: value.Final, EventTime: time.UnixMilli(eventTime).UTC()}, nil
}

func (worker *streamWorker) publishStatus(connected bool, err error) {
	status := StreamStatus{Keys: worker.keys(), Connected: connected, Err: err}
	select {
	case worker.statuses <- status:
	default:
		worker.logger.Warn("Binance status queue full", "module", "binance_live", "operation", "status")
	}
}
