package binance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	markettrade "crypto-scanner/internal/market/trade"

	"github.com/gorilla/websocket"
	"golang.org/x/time/rate"
)

type TradeEvent = markettrade.Event
type TradeStatus = markettrade.Status

// sharedDialLimiter accounts for both kline and trade connections in the
// process-wide Binance/IP connection budget.
var sharedDialLimiter = rate.NewLimiter(rate.Every(1200*time.Millisecond), 5)

// TradeStream is a process-wide pool of dynamically subscribed Binance Spot
// <symbol>@trade connections. Each symbol belongs to exactly one worker.
type TradeStream struct {
	url      string
	dialer   *websocket.Dialer
	logger   *slog.Logger
	events   chan TradeEvent
	statuses chan TradeStatus
	epoch    atomic.Int64

	mu          sync.Mutex
	workers     []*tradeWorker
	assignments map[string]int
	running     bool
	ctx         context.Context
}

func NewTradeStream(logger *slog.Logger) *TradeStream {
	return newTradeStream(defaultStreamURL, websocket.DefaultDialer, logger)
}
func newTradeStream(url string, dialer *websocket.Dialer, logger *slog.Logger) *TradeStream {
	return &TradeStream{url: url, dialer: dialer, logger: logger, events: make(chan TradeEvent, 512), statuses: make(chan TradeStatus, 64), assignments: map[string]int{}}
}
func (stream *TradeStream) Events() <-chan TradeEvent    { return stream.events }
func (stream *TradeStream) Statuses() <-chan TradeStatus { return stream.statuses }

func (stream *TradeStream) SetSymbols(symbols []string) {
	set := make(map[string]struct{}, len(symbols))
	for _, symbol := range symbols {
		symbol = strings.ToUpper(strings.TrimSpace(symbol))
		if symbol != "" {
			set[symbol] = struct{}{}
		}
	}
	sorted := make([]string, 0, len(set))
	for symbol := range set {
		sorted = append(sorted, symbol)
	}
	sort.Strings(sorted)
	stream.mu.Lock()
	defer stream.mu.Unlock()
	for symbol := range stream.assignments {
		if _, wanted := set[symbol]; !wanted {
			delete(stream.assignments, symbol)
		}
	}
	counts := make([]int, len(stream.workers))
	for _, workerIndex := range stream.assignments {
		counts[workerIndex]++
	}
	for _, symbol := range sorted {
		if _, assigned := stream.assignments[symbol]; assigned {
			continue
		}
		workerIndex := -1
		for index, count := range counts {
			if count < maxStreamsPerConnection {
				workerIndex = index
				break
			}
		}
		if workerIndex < 0 {
			worker := newTradeWorker(stream.url, stream.dialer, stream.logger, stream.events, stream.statuses, sharedDialLimiter, &stream.epoch)
			stream.workers = append(stream.workers, worker)
			counts = append(counts, 0)
			workerIndex = len(stream.workers) - 1
			if stream.running {
				go worker.run(stream.ctx)
			}
		}
		stream.assignments[symbol] = workerIndex
		counts[workerIndex]++
	}
	groups := make([][]string, len(stream.workers))
	for symbol, workerIndex := range stream.assignments {
		groups[workerIndex] = append(groups[workerIndex], symbol)
	}
	for index, worker := range stream.workers {
		sort.Strings(groups[index])
		worker.setSymbols(groups[index])
	}
}

func (stream *TradeStream) Run(ctx context.Context) error {
	stream.mu.Lock()
	if stream.running {
		stream.mu.Unlock()
		return errors.New("trade stream already running")
	}
	stream.running, stream.ctx = true, ctx
	workers := append([]*tradeWorker(nil), stream.workers...)
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

type tradeWorker struct {
	url         string
	dialer      *websocket.Dialer
	logger      *slog.Logger
	events      chan<- TradeEvent
	statuses    chan<- TradeStatus
	dialLimiter *rate.Limiter
	epoch       *atomic.Int64
	changes     chan struct{}
	request     atomic.Int64
	mu          sync.Mutex
	desired     map[string]struct{}
}

func newTradeWorker(url string, dialer *websocket.Dialer, logger *slog.Logger, events chan<- TradeEvent, statuses chan<- TradeStatus, dialLimiter *rate.Limiter, epoch *atomic.Int64) *tradeWorker {
	return &tradeWorker{url: url, dialer: dialer, logger: logger, events: events, statuses: statuses, dialLimiter: dialLimiter, epoch: epoch, changes: make(chan struct{}, 1), desired: map[string]struct{}{}}
}
func (worker *tradeWorker) setSymbols(symbols []string) {
	desired := make(map[string]struct{}, len(symbols))
	for _, symbol := range symbols {
		desired[symbol] = struct{}{}
	}
	worker.mu.Lock()
	worker.desired = desired
	worker.mu.Unlock()
	select {
	case worker.changes <- struct{}{}:
	default:
	}
}
func (worker *tradeWorker) symbols() []string {
	worker.mu.Lock()
	defer worker.mu.Unlock()
	result := make([]string, 0, len(worker.desired))
	for symbol := range worker.desired {
		result = append(result, symbol)
	}
	sort.Strings(result)
	return result
}
func (worker *tradeWorker) run(ctx context.Context) {
	runDynamicStreamWorker(ctx, func() bool { return len(worker.symbols()) > 0 }, worker.changes, worker.connect,
		func(err error) { worker.publish(false, err) }, worker.logger, "binance_trade", "trade")
}

func (worker *tradeWorker) connect(ctx context.Context) error {
	conn, err := dialBinanceStream(ctx, worker.url, worker.dialer, worker.dialLimiter, "trade")
	if err != nil {
		return err
	}
	defer conn.Close()

	epoch := worker.epoch.Add(1)
	readResult := make(chan error, 1)
	acks := make(chan controlReply, 16)
	go func() { readResult <- worker.readLoop(ctx, conn, acks, epoch) }()
	active := make(map[string]struct{})
	limiter := rate.NewLimiter(rate.Every(250*time.Millisecond), 1)
	rotation := time.NewTimer(streamRotation)
	defer rotation.Stop()
	if err := worker.reconcile(ctx, conn, limiter, active, acks); err != nil {
		return err
	}
	if len(active) == 0 {
		return errStreamWorkerIdle
	}
	worker.publish(true, nil)
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-readResult:
			return err
		case <-rotation.C:
			return errors.New("scheduled Binance trade connection rotation")
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

func (worker *tradeWorker) reconcile(ctx context.Context, conn *websocket.Conn, limiter *rate.Limiter, active map[string]struct{}, acks <-chan controlReply) error {
	desired := worker.symbols()
	desiredSet := make(map[string]struct{}, len(desired))
	for _, symbol := range desired {
		desiredSet[symbol] = struct{}{}
	}
	var add, remove []string
	for _, symbol := range desired {
		if _, ok := active[symbol]; !ok {
			add = append(add, symbol)
		}
	}
	for symbol := range active {
		if _, ok := desiredSet[symbol]; !ok {
			remove = append(remove, symbol)
		}
	}
	sort.Strings(remove)
	if err := worker.control(ctx, conn, limiter, "UNSUBSCRIBE", remove, acks); err != nil {
		return err
	}
	if err := worker.control(ctx, conn, limiter, "SUBSCRIBE", add, acks); err != nil {
		return err
	}
	clear(active)
	for symbol := range desiredSet {
		active[symbol] = struct{}{}
	}
	return nil
}

func (worker *tradeWorker) control(ctx context.Context, conn *websocket.Conn, limiter *rate.Limiter, method string, symbols []string, acks <-chan controlReply) error {
	names := make([]string, len(symbols))
	for index, symbol := range symbols {
		names[index] = strings.ToLower(symbol) + "@trade"
	}
	return controlBinanceStreams(ctx, conn, limiter, &worker.request, "trade", method, names, acks)
}

func (worker *tradeWorker) readLoop(ctx context.Context, conn *websocket.Conn, acks chan<- controlReply, epoch int64) error {
	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read Binance trade stream: %w", err)
		}
		var envelope struct {
			ID        *int64          `json:"id"`
			Result    json.RawMessage `json:"result"`
			Code      *int            `json:"code"`
			Msg       string          `json:"msg"`
			Event     string          `json:"e"`
			EventTime int64           `json:"E"`
			Symbol    string          `json:"s"`
			TradeID   int64           `json:"t"`
			Price     string          `json:"p"`
		}
		if err := json.Unmarshal(payload, &envelope); err != nil {
			return fmt.Errorf("decode Binance trade stream: %w", err)
		}
		if envelope.ID != nil {
			select {
			case acks <- controlReply{ID: *envelope.ID, Result: envelope.Result, Code: envelope.Code, Msg: envelope.Msg}:
			case <-ctx.Done():
				return nil
			}
			continue
		}
		if envelope.Event != "trade" {
			continue
		}
		price, validPrice := new(big.Rat).SetString(envelope.Price)
		if envelope.Symbol == "" || envelope.TradeID < 0 || !validPrice || price.Sign() <= 0 || envelope.EventTime <= 0 {
			return errors.New("invalid Binance trade event")
		}
		event := TradeEvent{Symbol: strings.ToUpper(envelope.Symbol), Price: envelope.Price, TradeID: envelope.TradeID, Epoch: epoch, EventTime: time.UnixMilli(envelope.EventTime).UTC()}
		select {
		case worker.events <- event:
		case <-ctx.Done():
			return nil
		default:
			return errors.New("trade event queue full")
		}
	}
}

func (worker *tradeWorker) publish(connected bool, err error) {
	status := TradeStatus{Symbols: worker.symbols(), Connected: connected, Err: err}
	select {
	case worker.statuses <- status:
	default:
		worker.logger.Warn("Binance trade status queue full", "module", "binance_trade", "operation", "status")
	}
}
