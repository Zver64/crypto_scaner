package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"crypto-scanner/internal/auth"
	authtelegram "crypto-scanner/internal/auth/telegram"
	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/exchange/binance"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
	marketlive "crypto-scanner/internal/market/live"

	"github.com/gorilla/websocket"
	"golang.org/x/time/rate"
)

const (
	maxLiveConnections        = 128
	maxLiveConnectionsPerUser = 4
	clientWriteTimeout        = 5 * time.Second
	clientPongTimeout         = 60 * time.Second
	clientPingPeriod          = 25 * time.Second
	clientAuthTimeout         = 5 * time.Second
	maxClientMessage          = 16 << 10
)

type LiveAuthenticator interface {
	AuthenticateInitData(context.Context, string) (auth.User, error)
}

type LiveCandles interface {
	RegisterClient(marketlive.Client) bool
	Subscribe(context.Context, marketlive.Client, string, market.CandleInterval) error
	Unsubscribe(string, binance.KlineKey)
	RemoveClient(string)
}

type liveCandleHandler struct {
	authenticator LiveAuthenticator
	service       LiveCandles
	charts        ChartService
	logger        *slog.Logger
	upgrader      websocket.Upgrader
	connections   chan struct{}
	userMu        sync.Mutex
	userCounts    map[int64]int
}

func newLiveCandleHandler(authenticator LiveAuthenticator, service LiveCandles, charts ChartService, logger *slog.Logger) http.Handler {
	handler := &liveCandleHandler{authenticator: authenticator, service: service, charts: charts, logger: logger, connections: make(chan struct{}, maxLiveConnections), userCounts: make(map[int64]int)}
	handler.upgrader = websocket.Upgrader{HandshakeTimeout: 5 * time.Second, CheckOrigin: sameWebSocketOrigin, EnableCompression: false}
	return handler
}

func sameWebSocketOrigin(request *http.Request) bool {
	raw := request.Header.Get("Origin")
	if raw == "" {
		return false
	}
	origin, err := url.Parse(raw)
	return err == nil && (origin.Scheme == "http" || origin.Scheme == "https") && strings.EqualFold(origin.Host, request.Host)
}

func (handler *liveCandleHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	select {
	case handler.connections <- struct{}{}:
		defer func() { <-handler.connections }()
	default:
		http.Error(response, "Live connection limit reached", http.StatusServiceUnavailable)
		return
	}
	connection, err := handler.upgrader.Upgrade(response, request, nil)
	if err != nil {
		return
	}
	client := newLiveSocketClient(newRequestID(), connection)
	subscriber := newChartClient(client, handler.charts)
	defer func() { handler.service.RemoveClient(client.ID()); subscriber.Close() }()
	connection.SetReadLimit(maxClientMessage)
	_ = connection.SetReadDeadline(time.Now().Add(clientAuthTimeout))
	message, err := readLiveClientMessage(connection)
	if err != nil || message.Type != Authenticate || message.InitData == nil {
		client.writeError("unauthenticated", "Telegram authentication is required")
		return
	}
	user, err := handler.authenticator.AuthenticateInitData(request.Context(), *message.InitData)
	if err != nil {
		switch {
		case errors.Is(err, authtelegram.ErrAccessDenied):
			client.writeError("access_denied", "Telegram user is not allowed")
		default:
			client.writeError("unauthenticated", "Telegram authentication is invalid or expired")
		}
		return
	}
	if !handler.acquireUser(user.ID) {
		client.writeError("rate_limited", "Live connection limit exceeded")
		return
	}
	defer handler.releaseUser(user.ID)
	if !handler.service.RegisterClient(subscriber) {
		return
	}
	_ = connection.SetReadDeadline(time.Now().Add(clientPongTimeout))
	connection.SetPongHandler(func(string) error { return connection.SetReadDeadline(time.Now().Add(clientPongTimeout)) })
	go client.writeLoop()
	if !client.enqueueWire(LiveCandleServerMessage{Type: Authenticated}) {
		return
	}
	limiter := rate.NewLimiter(rate.Limit(10), 20)
	for {
		if !limiter.Allow() {
			client.writeError("rate_limited", "Too many WebSocket messages")
			return
		}
		message, err = readLiveClientMessage(connection)
		if err != nil {
			return
		}
		if message.Type == Authenticate {
			client.enqueueError("invalid_message", "The connection is already authenticated")
			continue
		}
		if message.Symbol == nil || message.Interval == nil || strings.TrimSpace(*message.Symbol) == "" || !market.CandleInterval(*message.Interval).Valid() {
			client.enqueueError("invalid_argument", "A valid symbol and interval are required")
			continue
		}
		key := binance.KlineKey{Symbol: strings.ToUpper(strings.TrimSpace(*message.Symbol)), Interval: market.CandleInterval(*message.Interval)}
		switch message.Type {
		case Subscribe:
			limit := defaultCandlePageSize
			if message.Limit != nil {
				limit = *message.Limit
			}
			if limit < 1 || limit > 5000 {
				client.enqueueKeyError(key, "invalid_argument", "Invalid chart range")
				continue
			}
			if message.Indicators == nil || len(*message.Indicators) == 0 || len(*message.Indicators) > 8 {
				client.enqueueKeyError(key, "invalid_argument", "Invalid indicator selection")
				continue
			}
			configs := make([]chart.IndicatorConfig, len(*message.Indicators))
			for i, item := range *message.Indicators {
				configs[i] = chart.IndicatorConfig{Type: indicator.Type(item.Type), Parameters: indicator.Parameters(item.Parameters)}
			}
			// Subscribing again to the same key only changes the chart range.
			err := handler.service.Subscribe(request.Context(), subscriber, key.Symbol, key.Interval)
			if err == nil {
				subscriber.setRange(key, limit, configs)
				subscriber.Enqueue(marketlive.Message{Kind: "refresh", Key: key})
			}
			switch {
			case errors.Is(err, marketlive.ErrInactiveSymbol):
				client.enqueueKeyError(key, "symbol_not_found", "Symbol is unknown or inactive")
			case errors.Is(err, marketlive.ErrTooManyClientSubscriptions):
				client.enqueueKeyError(key, "subscription_limit", "Subscription limit exceeded")
			case err != nil:
				handler.logger.Error("live candle subscription failed", "module", "httpapi_live", "operation", "subscribe", "error", err)
				client.enqueueKeyError(key, "unavailable", "Live candles are temporarily unavailable")
			}
		case Unsubscribe:
			handler.service.Unsubscribe(client.ID(), key)
			subscriber.forget(key)
			interval := CandleInterval(key.Interval)
			symbol := key.Symbol
			if !client.enqueueWire(LiveCandleServerMessage{Type: Unsubscribed, Symbol: &symbol, Interval: &interval}) {
				return
			}
		default:
			client.enqueueError("invalid_message", "Unsupported WebSocket message")
		}
	}
}

func (handler *liveCandleHandler) acquireUser(userID int64) bool {
	handler.userMu.Lock()
	defer handler.userMu.Unlock()
	if handler.userCounts[userID] >= maxLiveConnectionsPerUser {
		return false
	}
	handler.userCounts[userID]++
	return true
}

func (handler *liveCandleHandler) releaseUser(userID int64) {
	handler.userMu.Lock()
	defer handler.userMu.Unlock()
	if handler.userCounts[userID] <= 1 {
		delete(handler.userCounts, userID)
		return
	}
	handler.userCounts[userID]--
}

func readLiveClientMessage(connection *websocket.Conn) (LiveCandleClientMessage, error) {
	_, reader, err := connection.NextReader()
	if err != nil {
		return LiveCandleClientMessage{}, err
	}
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var message LiveCandleClientMessage
	if err := decoder.Decode(&message); err != nil || !message.Type.Valid() {
		return LiveCandleClientMessage{}, errors.New("invalid client message")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return LiveCandleClientMessage{}, errors.New("invalid trailing JSON data")
	}
	return message, nil
}

type liveSocketClient struct {
	id         string
	connection *websocket.Conn
	queue      chan LiveCandleServerMessage
	done       chan struct{}
	once       sync.Once
	writeMu    sync.Mutex
}

func newLiveSocketClient(id string, connection *websocket.Conn) *liveSocketClient {
	return &liveSocketClient{id: id, connection: connection, queue: make(chan LiveCandleServerMessage, 32), done: make(chan struct{})}
}
func (client *liveSocketClient) ID() string { return client.id }
func (client *liveSocketClient) Enqueue(message marketlive.Message) bool {
	return client.enqueueWire(liveWireMessage(message))
}
func (client *liveSocketClient) enqueueWire(message LiveCandleServerMessage) bool {
	select {
	case <-client.done:
		return false
	case client.queue <- message:
		return true
	default:
		client.Close()
		return false
	}
}
func (client *liveSocketClient) Close() {
	client.once.Do(func() {
		close(client.done)
		_ = client.connection.Close()
	})
}
func (client *liveSocketClient) enqueueError(code, message string) {
	c := LiveCandleServerMessageCode(code)
	client.enqueueWire(LiveCandleServerMessage{Type: Error, Code: &c, Message: &message})
}
func (client *liveSocketClient) enqueueKeyError(key binance.KlineKey, code, message string) {
	c := LiveCandleServerMessageCode(code)
	symbol, interval := key.Symbol, CandleInterval(key.Interval)
	client.enqueueWire(LiveCandleServerMessage{Type: Error, Code: &c, Message: &message, Symbol: &symbol, Interval: &interval})
}
func (client *liveSocketClient) writeError(code, message string) {
	client.writeMu.Lock()
	defer client.writeMu.Unlock()
	c := LiveCandleServerMessageCode(code)
	_ = client.connection.SetWriteDeadline(time.Now().Add(clientWriteTimeout))
	_ = client.connection.WriteJSON(LiveCandleServerMessage{Type: Error, Code: &c, Message: &message})
	_ = client.connection.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, message), time.Now().Add(clientWriteTimeout))
}
func (client *liveSocketClient) writeLoop() {
	ticker := time.NewTicker(clientPingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-client.done:
			return
		case message := <-client.queue:
			if !client.write(message) {
				client.Close()
				return
			}
		case <-ticker.C:
			client.writeMu.Lock()
			err := client.connection.WriteControl(websocket.PingMessage, nil, time.Now().Add(clientWriteTimeout))
			client.writeMu.Unlock()
			if err != nil {
				client.Close()
				return
			}
		}
	}
}
func (client *liveSocketClient) write(message LiveCandleServerMessage) bool {
	client.writeMu.Lock()
	defer client.writeMu.Unlock()
	_ = client.connection.SetWriteDeadline(time.Now().Add(clientWriteTimeout))
	return client.connection.WriteJSON(message) == nil
}

func liveWireMessage(message marketlive.Message) LiveCandleServerMessage {
	symbol := message.Key.Symbol
	interval := CandleInterval(message.Key.Interval)
	result := LiveCandleServerMessage{Type: LiveCandleServerMessageType(message.Kind), Symbol: &symbol, Interval: &interval}
	if message.Freshness != "" {
		value := LiveCandleServerMessageFreshness(message.Freshness)
		result.Freshness = &value
	}
	if message.Reason != "" {
		result.Message = &message.Reason
	}
	return result
}

// Hijack preserves WebSocket upgrades through request logging middleware.
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}
	r.status = http.StatusSwitchingProtocols
	r.wroteHeader = true
	return hijacker.Hijack()
}
