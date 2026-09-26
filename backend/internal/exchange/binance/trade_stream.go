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
type TradeStream struct {
	*streamPool[string]
	events      chan markettrade.Event
	statuses    chan markettrade.Status
	assignments map[string]int // symbol -> worker index; guarded by mu
}

func NewTradeStream(logger *slog.Logger, dialLimiter *rate.Limiter) *TradeStream {
	return newTradeStream(defaultStreamURL, websocket.DefaultDialer, logger, dialLimiter)
}

func newTradeStream(url string, dialer *websocket.Dialer, logger *slog.Logger, dialLimiter *rate.Limiter) *TradeStream {
	stream := &TradeStream{events: make(chan markettrade.Event, 512), statuses: make(chan markettrade.Status, 64), assignments: map[string]int{}}
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

func (stream *TradeStream) Events() <-chan markettrade.Event    { return stream.events }
func (stream *TradeStream) Statuses() <-chan markettrade.Status { return stream.statuses }

func (stream *TradeStream) SetSymbols(symbols []string) {
	set := make(map[string]struct{}, len(symbols))
	for _, symbol := range symbols {
		symbol = market.NormalizeSymbol(symbol)
		if symbol != "" {
			set[symbol] = struct{}{}
		}
	}
	sorted := slices.Sorted(maps.Keys(set))
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
	event := markettrade.Event{Symbol: strings.ToUpper(message.Symbol), Price: message.Price, TradeID: message.TradeID, Epoch: epoch, EventTime: time.UnixMilli(message.EventTime).UTC()}
	select {
	case stream.events <- event:
		return nil
	case <-ctx.Done():
		return nil
	default:
		return errors.New("trade event queue full")
	}
}
