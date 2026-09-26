package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestTradeStreamPartitionsSymbolsAcrossConnectionLimit(t *testing.T) {
	stream := newTradeStream("ws://unused", websocket.DefaultDialer, slog.New(slog.NewTextHandler(io.Discard, nil)), NewDialLimiter())
	symbols := make([]string, maxStreamsPerConnection*2+1)
	for index := range symbols {
		symbols[index] = fmt.Sprintf("ASSET%04dUSDT", index)
	}
	stream.SetSymbols(symbols)
	if len(stream.workers) != 3 {
		t.Fatalf("workers = %d, want 3", len(stream.workers))
	}
	seen := map[string]bool{}
	for _, worker := range stream.workers {
		owned := worker.keys()
		if len(owned) > maxStreamsPerConnection {
			t.Fatalf("worker owns %d streams", len(owned))
		}
		for _, symbol := range owned {
			if seen[symbol] {
				t.Fatalf("symbol %s has multiple upstream subscriptions", symbol)
			}
			seen[symbol] = true
		}
	}
	if len(seen) != len(symbols) {
		t.Fatalf("owned symbols = %d, want %d", len(seen), len(symbols))
	}
	originalWorker := stream.assignments[symbols[maxStreamsPerConnection]]
	updated := append([]string(nil), symbols[1:]...)
	updated = append(updated, "NEWASSETUSDT")
	stream.SetSymbols(updated)
	if got := stream.assignments[symbols[maxStreamsPerConnection]]; got != originalWorker {
		t.Fatalf("existing symbol moved from worker %d to %d", originalWorker, got)
	}
}

func TestTradeStreamWaitsForAcknowledgementBeforeConnectedAndDeliversTrade(t *testing.T) {
	type tradeCommand struct {
		ID     int64  `json:"id"`
		Method string `json:"method"`
	}
	command := make(chan tradeCommand, 2)
	allowACK := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		connection, err := (&websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}).Upgrade(response, request, nil)
		if err != nil {
			return
		}
		defer connection.Close()
		_, payload, err := connection.ReadMessage()
		if err != nil {
			return
		}
		var value tradeCommand
		if err := json.Unmarshal(payload, &value); err != nil {
			t.Error(err)
			return
		}
		command <- value
		<-allowACK
		if err := connection.WriteJSON(map[string]any{"result": nil, "id": value.ID}); err != nil {
			return
		}
		_ = connection.WriteJSON(map[string]any{"e": "trade", "E": time.Now().UnixMilli(), "s": "BTCUSDT", "t": 42, "p": "100.25"})
		<-request.Context().Done()
	}))
	defer server.Close()

	stream := newTradeStream("ws"+strings.TrimPrefix(server.URL, "http"), websocket.DefaultDialer, slog.New(slog.NewTextHandler(io.Discard, nil)), NewDialLimiter())
	stream.SetSymbols([]string{"BTCUSDT"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = stream.Run(ctx) }()
	select {
	case got := <-command:
		if got.Method != "SUBSCRIBE" {
			t.Fatalf("method = %s", got.Method)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("subscription command not received")
	}
	select {
	case status := <-stream.Statuses():
		t.Fatalf("status published before ACK: %+v", status)
	case <-time.After(100 * time.Millisecond):
	}
	close(allowACK)
	select {
	case status := <-stream.Statuses():
		if !status.Connected {
			t.Fatalf("status after ACK = %+v", status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("connected status not published after ACK")
	}
	select {
	case event := <-stream.Events():
		if event.Symbol != "BTCUSDT" || event.TradeID != 42 || event.Price != "100.25" || event.Epoch == 0 {
			t.Fatalf("trade event = %+v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("trade event not delivered")
	}
}
