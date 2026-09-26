package binance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"crypto-scanner/internal/market"
	"crypto-scanner/internal/market/kline"
	"crypto-scanner/internal/platform/numeric"

	"github.com/gorilla/websocket"
	"golang.org/x/time/rate"
)

var _ kline.Feed = (*KlineStream)(nil)

func klineStreamName(key kline.Key) string {
	return strings.ToLower(key.Symbol) + "@kline_" + string(key.Interval)
}

// KlineStream is a shared, dynamically subscribed Binance kline pool.
type KlineStream struct {
	*streamPool[kline.Key]
	events   chan kline.Event
	statuses chan kline.Status
}

func NewKlineStream(logger *slog.Logger, dialLimiter *rate.Limiter) *KlineStream {
	return newKlineStream(defaultStreamURL, websocket.DefaultDialer, logger, dialLimiter)
}

func newKlineStream(url string, dialer *websocket.Dialer, logger *slog.Logger, dialLimiter *rate.Limiter) *KlineStream {
	stream := &KlineStream{events: make(chan kline.Event, 256), statuses: make(chan kline.Status, 32)}
	stream.streamPool = newStreamPool(url, dialer, logger, dialLimiter, streamKind[kline.Key]{
		label: "kline", module: "binance_kline", event: "kline", name: klineStreamName,
		handle: stream.handle,
		status: func(keys []kline.Key, connected bool, err error) bool {
			select {
			case stream.statuses <- kline.Status{Keys: keys, Connected: connected, Err: err}:
				return true
			default:
				return false
			}
		},
	})
	return stream
}

func (stream *KlineStream) Events() <-chan kline.Event    { return stream.events }
func (stream *KlineStream) Statuses() <-chan kline.Status { return stream.statuses }

func (stream *KlineStream) Subscribe(key kline.Key) error {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	var target *streamWorker[kline.Key]
	for _, worker := range stream.workers {
		if worker.has(key) {
			return nil
		}
		if target == nil && worker.count() < maxStreamsPerConnection {
			target = worker
		}
	}
	if target == nil {
		target = stream.addWorkerLocked()
	}
	target.update(func(desired map[kline.Key]struct{}) { desired[key] = struct{}{} })
	return nil
}

func (stream *KlineStream) Unsubscribe(key kline.Key) {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	for _, worker := range stream.workers {
		if worker.has(key) {
			worker.update(func(desired map[kline.Key]struct{}) { delete(desired, key) })
			return
		}
	}
}

func (stream *KlineStream) handle(ctx context.Context, payload []byte, _ int64) error {
	var message struct {
		Event     string    `json:"e"`
		EventTime int64     `json:"E"`
		Kline     wireKline `json:"k"`
	}
	if err := json.Unmarshal(payload, &message); err != nil {
		return fmt.Errorf("decode Binance kline stream: %w", err)
	}
	event, err := decodeKline(message.EventTime, message.Kline)
	if err != nil {
		return err
	}
	select {
	case stream.events <- event:
	case <-ctx.Done():
	}
	return nil
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
	// Keys equal to the ones above except for case; encoding/json would
	// otherwise decode taker volumes into Volume and QuoteVolume.
	FirstTradeID int64       `json:"f"`
	LastTradeID  int64       `json:"L"`
	TakerVolume  wireDecimal `json:"V"`
	TakerQuote   wireDecimal `json:"Q"`
}

func decodeKline(eventTime int64, value wireKline) (kline.Event, error) {
	interval := market.CandleInterval(value.Interval)
	if !interval.Valid() || value.Symbol == "" || value.OpenTime <= 0 || value.CloseTime <= value.OpenTime || value.Trades < 0 {
		return kline.Event{}, errors.New("invalid Binance kline metadata")
	}
	fields := []struct {
		name string
		raw  wireDecimal
	}{{"open", value.Open}, {"high", value.High}, {"low", value.Low}, {"close", value.Close}, {"volume", value.Volume}, {"quote volume", value.QuoteVolume}}
	numbers := make([]float64, len(fields))
	for index, field := range fields {
		number, err := numeric.ParseFinite(string(field.raw))
		if err != nil {
			return kline.Event{}, fmt.Errorf("decode Binance kline %s: invalid number", field.name)
		}
		numbers[index] = number
	}
	open, high, low, closePrice, volume, quote := numbers[0], numbers[1], numbers[2], numbers[3], numbers[4], numbers[5]
	if high < max(open, closePrice) || low > min(open, closePrice) || volume < 0 || quote < 0 {
		return kline.Event{}, errors.New("invalid Binance kline values")
	}
	return kline.Event{Key: kline.Key{Symbol: strings.ToUpper(value.Symbol), Interval: interval}, Candle: market.Candle{Interval: interval, OpenTime: time.UnixMilli(value.OpenTime).UTC(), CloseTime: time.UnixMilli(value.CloseTime).UTC(), Open: open, High: high, Low: low, Close: closePrice, Volume: volume, QuoteAssetVolume: quote, TradeCount: value.Trades}, Final: value.Final, EventTime: time.UnixMilli(eventTime).UTC()}, nil
}
