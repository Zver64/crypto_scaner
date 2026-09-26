package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/time/rate"
)

type controlReply struct {
	ID     int64           `json:"id"`
	Result json.RawMessage `json:"result"`
	Code   *int            `json:"code,omitempty"`
	Msg    string          `json:"msg,omitempty"`
}

const (
	streamReadTimeout  = 70 * time.Second
	streamWriteTimeout = 5 * time.Second
	streamACKTimeout   = 10 * time.Second
	streamRotation     = 23*time.Hour + 55*time.Minute
)

func dialBinanceStream(ctx context.Context, url string, dialer *websocket.Dialer, limiter *rate.Limiter, label string) (*websocket.Conn, error) {
	if err := limiter.Wait(ctx); err != nil {
		return nil, err
	}
	conn, response, err := dialer.DialContext(ctx, url, http.Header{})
	if err != nil {
		if response != nil {
			return nil, fmt.Errorf("dial Binance %s stream: HTTP %d: %w", label, response.StatusCode, err)
		}
		return nil, fmt.Errorf("dial Binance %s stream: %w", label, err)
	}
	conn.SetReadLimit(1 << 20)
	_ = conn.SetReadDeadline(time.Now().Add(streamReadTimeout))
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(streamReadTimeout)) })
	conn.SetPingHandler(func(data string) error {
		_ = conn.SetReadDeadline(time.Now().Add(streamReadTimeout))
		return conn.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(streamWriteTimeout))
	})
	return conn, nil
}

func controlBinanceStreams(ctx context.Context, conn *websocket.Conn, limiter *rate.Limiter, request *atomic.Int64, label, method string, names []string, acks <-chan controlReply) error {
	for start := 0; start < len(names); start += maxStreamsPerCommand {
		end := min(start+maxStreamsPerCommand, len(names))
		if err := limiter.Wait(ctx); err != nil {
			return err
		}
		id := request.Add(1)
		_ = conn.SetWriteDeadline(time.Now().Add(streamWriteTimeout))
		if err := conn.WriteJSON(struct {
			Method string   `json:"method"`
			Params []string `json:"params"`
			ID     int64    `json:"id"`
		}{method, names[start:end], id}); err != nil {
			return fmt.Errorf("write %s %s: %w", label, method, err)
		}
		timer := time.NewTimer(streamACKTimeout)
		for {
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
				return fmt.Errorf("Binance %s %s acknowledgement timeout", label, method)
			case reply := <-acks:
				if reply.ID != id {
					continue
				}
				timer.Stop()
				if reply.Code != nil {
					return fmt.Errorf("Binance %s %s rejected: code %d: %s", label, method, *reply.Code, reply.Msg)
				}
				if string(reply.Result) != "null" {
					return fmt.Errorf("Binance %s %s unexpected acknowledgement", label, method)
				}
				goto acknowledged
			}
		}
	acknowledged:
	}
	return nil
}
