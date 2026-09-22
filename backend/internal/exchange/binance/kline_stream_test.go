package binance

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"crypto-scanner/internal/market"

	"github.com/gorilla/websocket"
)

func TestKlineStreamClosesIdleUpstreamAfterAcknowledgedUnsubscribe(t *testing.T) {
	methods := make(chan string, 2)
	closed := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		connection, err := (&websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}).Upgrade(response, request, nil)
		if err != nil {
			return
		}
		defer connection.Close()
		defer close(closed)
		for {
			_, payload, err := connection.ReadMessage()
			if err != nil {
				return
			}
			var command struct {
				Method string `json:"method"`
				ID     int64  `json:"id"`
			}
			if err := json.Unmarshal(payload, &command); err != nil {
				t.Error(err)
				return
			}
			methods <- command.Method
			if err := connection.WriteJSON(map[string]any{"result": nil, "id": command.ID}); err != nil {
				return
			}
		}
	}))
	defer server.Close()
	stream := newKlineStream("ws"+strings.TrimPrefix(server.URL, "http"), websocket.DefaultDialer, slog.New(slog.NewTextHandler(io.Discard, nil)))
	key := KlineKey{Symbol: "BTCUSDT", Interval: market.IntervalHour}
	if err := stream.Subscribe(key); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = stream.Run(ctx) }()
	waitMethod := func(want string) {
		t.Helper()
		select {
		case got := <-methods:
			if got != want {
				t.Fatalf("control method = %s, want %s", got, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for %s", want)
		}
	}
	waitMethod("SUBSCRIBE")
	stream.Unsubscribe(key)
	waitMethod("UNSUBSCRIBE")
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("idle upstream connection remained open")
	}
}

func TestDecodeKlineReturnsCompleteSnapshot(t *testing.T) {
	value := wireKline{
		OpenTime:  time.Date(2026, 1, 2, 3, 0, 0, 0, time.UTC).UnixMilli(),
		CloseTime: time.Date(2026, 1, 2, 3, 59, 59, 999_000_000, time.UTC).UnixMilli(),
		Symbol:    "btcusdt", Interval: "1h", Open: "100.5", High: "110", Low: "99", Close: "108.25", Volume: "12.5", QuoteVolume: "1300.75", Trades: 42, Final: true,
	}
	event, err := decodeKline(time.Date(2026, 1, 2, 3, 30, 0, 0, time.UTC).UnixMilli(), value)
	if err != nil {
		t.Fatal(err)
	}
	if event.Key != (KlineKey{Symbol: "BTCUSDT", Interval: market.IntervalHour}) {
		t.Fatalf("unexpected key: %+v", event.Key)
	}
	if !event.Final || event.Candle.Open != 100.5 || event.Candle.High != 110 || event.Candle.Low != 99 || event.Candle.Close != 108.25 || event.Candle.Volume != 12.5 || event.Candle.QuoteAssetVolume != 1300.75 || event.Candle.TradeCount != 42 {
		t.Fatalf("incomplete candle: %+v", event)
	}
}

func TestWireKlineAcceptsStringAndNumericDecimals(t *testing.T) {
	var value wireKline
	if err := json.Unmarshal([]byte(`{"o":"100.5","h":110,"l":99.25,"c":"108.25","v":12.5,"q":"1300.75"}`), &value); err != nil {
		t.Fatal(err)
	}
	if value.Open != "100.5" || value.High != "110" || value.Low != "99.25" || value.Close != "108.25" || value.Volume != "12.5" || value.QuoteVolume != "1300.75" {
		t.Fatalf("unexpected wire decimals: %+v", value)
	}
}
