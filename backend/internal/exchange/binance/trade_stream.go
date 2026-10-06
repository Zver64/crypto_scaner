package binance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"math/big"
	"slices"
	"strings"
	"sync"
	"time"

	"crypto-scanner/internal/market"
	markettrade "crypto-scanner/internal/market/trade"

	"github.com/gorilla/websocket"
	"golang.org/x/time/rate"
)

var _ markettrade.Feed = (*TradeStream)(nil)

func tradeStreamName(symbol string) string { return strings.ToLower(symbol) + "@trade" }

// TradeStream is a process-wide pool of dynamically subscribed Binance Spot
// <symbol>@trade connections. Each symbol belongs to exactly one worker.
// Trades merge into one pending event per symbol and epoch, so a slow
// consumer never drops trades or forces a reconnect.
type TradeStream struct {
	*streamPool[string]
	statuses    chan markettrade.Status
	assignments map[string]int // symbol -> worker index; guarded by mu

	ready     chan struct{}
	pendingMu sync.Mutex
	pending   map[string][]*pendingTrades // in epoch order
}

// pendingTrades is the merged event of one symbol and epoch.
type pendingTrades struct {
	event     markettrade.Event
	low, high *big.Rat
}

func NewTradeStream(logger *slog.Logger, dialLimiter *rate.Limiter) *TradeStream {
	return newTradeStream(defaultStreamURL, websocket.DefaultDialer, logger, dialLimiter)
}

func newTradeStream(url string, dialer *websocket.Dialer, logger *slog.Logger, dialLimiter *rate.Limiter) *TradeStream {
	stream := &TradeStream{statuses: make(chan markettrade.Status, 64), assignments: map[string]int{}, ready: make(chan struct{}, 1), pending: map[string][]*pendingTrades{}}
	stream.streamPool = newStreamPool(url, dialer, logger, dialLimiter, streamKind[string]{
		label: "trade", module: "binance_trade", event: "trade", name: tradeStreamName,
		handle: stream.handle,
		status: func(symbols []string, connected bool, err error) bool {
			select {
			case stream.statuses <- markettrade.Status{Symbols: symbols, Connected: connected, Err: err}:
				return true
			default:
				return false
			}
		},
	})
	return stream
}

func (stream *TradeStream) Ready() <-chan struct{}              { return stream.ready }
func (stream *TradeStream) Statuses() <-chan markettrade.Status { return stream.statuses }

func (stream *TradeStream) Drain() []markettrade.Event {
	stream.pendingMu.Lock()
	defer stream.pendingMu.Unlock()
	events := make([]markettrade.Event, 0, len(stream.pending))
	for _, merged := range stream.pending {
		for _, item := range merged {
			events = append(events, item.event)
		}
	}
	clear(stream.pending)
	return events
}

func (stream *TradeStream) SetSymbols(symbols []string) {
	set := make(map[string]struct{}, len(symbols))
	for _, symbol := range symbols {
		symbol = market.NormalizeSymbol(symbol)
		if symbol != "" {
			set[symbol] = struct{}{}
		}
	}
	sorted := slices.Sorted(maps.Keys(set))
	stream.pendingMu.Lock()
	for symbol := range stream.pending {
		if _, wanted := set[symbol]; !wanted {
			delete(stream.pending, symbol)
		}
	}
	stream.pendingMu.Unlock()
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
		workerIndex := slices.IndexFunc(counts, func(count int) bool { return count < maxStreamsPerConnection })
		if workerIndex < 0 {
			stream.addWorkerLocked()
			counts = append(counts, 0)
			workerIndex = len(stream.workers) - 1
		}
		stream.assignments[symbol] = workerIndex
		counts[workerIndex]++
	}
	groups := make([]map[string]struct{}, len(stream.workers))
	for index := range groups {
		groups[index] = map[string]struct{}{}
	}
	for symbol, workerIndex := range stream.assignments {
		groups[workerIndex][symbol] = struct{}{}
	}
	for index, worker := range stream.workers {
		worker.update(func(desired map[string]struct{}) {
			clear(desired)
			maps.Copy(desired, groups[index])
		})
	}
}

func (stream *TradeStream) handle(ctx context.Context, payload []byte, epoch int64) error {
	// Case-only duplicates ("e"/"E", "t"/"T", "m"/"M") need their own fields
	// because encoding/json matches keys case-insensitively.
	var message struct {
		Event     string `json:"e"`
		EventTime int64  `json:"E"`
		Symbol    string `json:"s"`
		TradeID   int64  `json:"t"`
		TradeTime int64  `json:"T"`
		Price     string `json:"p"`
		Maker     bool   `json:"m"`
		Ignore    bool   `json:"M"`
	}
	if err := json.Unmarshal(payload, &message); err != nil {
		return fmt.Errorf("decode Binance trade stream: %w", err)
	}
	price, validPrice := new(big.Rat).SetString(message.Price)
	if message.Symbol == "" || message.TradeID < 0 || !validPrice || price.Sign() <= 0 || message.EventTime <= 0 {
		return errors.New("invalid Binance trade event")
	}
	symbol := market.NormalizeSymbol(message.Symbol)
	eventTime := time.UnixMilli(message.EventTime).UTC()
	stream.pendingMu.Lock()
	merged := stream.pending[symbol]
	if count := len(merged); count > 0 && merged[count-1].event.Epoch == epoch {
		last := merged[count-1]
		if price.Cmp(last.low) < 0 {
			last.low, last.event.Low = price, message.Price
		}
		if price.Cmp(last.high) > 0 {
			last.high, last.event.High = price, message.Price
		}
		last.event.Price, last.event.TradeID, last.event.EventTime = message.Price, message.TradeID, eventTime
	} else {
		stream.pending[symbol] = append(merged, &pendingTrades{
			event: markettrade.Event{Symbol: symbol, Price: message.Price, Low: message.Price, High: message.Price, TradeID: message.TradeID, Epoch: epoch, EventTime: eventTime},
			low:   price, high: price,
		})
	}
	stream.pendingMu.Unlock()
	select {
	case stream.ready <- struct{}{}:
	default:
	}
	return nil
}
