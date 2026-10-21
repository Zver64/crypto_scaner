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
	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/market/kline"
	marketlive "crypto-scanner/internal/market/live"

	"github.com/gorilla/websocket"
	"golang.org/x/time/rate"
)

const (
	maxLiveConnections        = 128
	maxLiveConnectionsPerUser = 4
	// liveCandleProtocol is the WebSocket subprotocol of live candles. The
	// client offers it followed by its session token, which the handshake
	// authenticates, so no unauthenticated socket is ever upgraded and the
	// token never appears in the URL. The server selects only this protocol.
	liveCandleProtocol = "crypto-scanner.live.v1"
	clientWriteTimeout = 5 * time.Second
	clientPongTimeout  = 60 * time.Second
	clientPingPeriod   = 25 * time.Second
	// sessionCheckPeriod bounds how long a connection outlives its session,
	// for example after the user is deleted.
	sessionCheckPeriod  = time.Minute
	sessionCheckTimeout = 5 * time.Second
	maxClientMessage    = 16 << 10
	maxSymbolLength     = 32
)

// errSessionRequired rejects an API token: live candles need a Mini App
// session.
var errSessionRequired = errors.New("live candles need a Mini App session")

type LiveCandles interface {
	RegisterClient(marketlive.Client) bool
	Subscribe(context.Context, marketlive.Client, string, market.CandleInterval) error
	Unsubscribe(string, kline.Key)
	RemoveClient(string)
}

type liveCandleHandler struct {
	sessions Sessions
	service  LiveCandles
	charts   ChartService
	logger   *slog.Logger
	upgrader websocket.Upgrader
	// connectionsMu guards connections and userConnections, the slots
	// reserved before an upgrade and released when its handler returns.
	connectionsMu   sync.Mutex
	connections     int
	userConnections map[int64]int
}

func newLiveCandleHandler(sessions Sessions, service LiveCandles, charts ChartService, logger *slog.Logger) http.Handler {
	handler := &liveCandleHandler{sessions: sessions, service: service, charts: charts, logger: logger, userConnections: make(map[int64]int)}
	// Chart snapshots are large, repetitive JSON; compressing them saves round
	// trips on high-latency links, where TCP slow start dominates delivery.
	handler.upgrader = websocket.Upgrader{HandshakeTimeout: 5 * time.Second, CheckOrigin: sameWebSocketOrigin, EnableCompression: true, Subprotocols: []string{liveCandleProtocol}}
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
	token, ok := liveCandleToken(request)
	if !ok {
		writeAPIError(response, http.StatusUnauthorized, "unauthenticated", authenticationRequiredMessage, nil)
		return
	}
	user, err := handler.authenticate(request.Context(), token)
	if errors.Is(err, errSessionRequired) {
		writeAPIError(response, http.StatusForbidden, "session_required", "Live candles need a Mini App session, not an API token", nil)
		return
	}
	if err != nil {
		writeAuthenticationError(response, request, handler.logger, err)
		return
	}
	// Limits are checked before the upgrade, so a client at a limit sees a
	// failed handshake instead of an open socket that closes at once.
	switch handler.reserve(user.ID) {
	case reservedUserLimit:
		writeAPIError(response, http.StatusTooManyRequests, "rate_limited", "Live connection limit exceeded", nil)
		return
	case reservedServerLimit:
		writeAPIError(response, http.StatusServiceUnavailable, "unavailable", "Live connection limit reached", nil)
		return
	}
	defer handler.release(user.ID)
	connection, err := handler.upgrader.Upgrade(response, request, nil)
	if err != nil {
		return
	}
	connection.SetReadLimit(maxClientMessage)
	client := newLiveSocketClient(newRequestID(), connection)
	defer client.Close()
	subscriber := newChartClient(client, handler.charts, handler.logger)
	// Chart streams may read storage until they stop, so the handler waits
	// for them before it returns.
	defer func() { handler.service.RemoveClient(client.ID()); subscriber.Close(); subscriber.Wait() }()
	if !handler.service.RegisterClient(subscriber) {
		return
	}
	_ = connection.SetReadDeadline(time.Now().Add(clientPongTimeout))
	connection.SetPongHandler(func(string) error { return connection.SetReadDeadline(time.Now().Add(clientPongTimeout)) })
	go client.writeLoop()
	// A hijacked request's context lives until the handler returns, so the
	// watch gets its own context to cancel an in-flight check on disconnect.
	watchCtx, cancelWatch := context.WithCancel(request.Context())
	sessionWatched := make(chan struct{})
	go func() {
		defer close(sessionWatched)
		handler.watchSession(watchCtx, client, token)
	}()
	// Closing the client unblocks the read loop; cancelling stops the watch.
	defer func() { client.Close(); cancelWatch(); <-sessionWatched }()
	limiter := rate.NewLimiter(rate.Limit(10), 20)
	for {
		if !limiter.Allow() {
			client.writeError("rate_limited", "Too many WebSocket messages")
			return
		}
		message, err := readLiveClientMessage(connection)
		if err != nil {
			return
		}
		if message.Symbol == nil || message.Interval == nil || strings.TrimSpace(*message.Symbol) == "" || len(*message.Symbol) > maxSymbolLength || !market.CandleInterval(*message.Interval).Valid() {
			client.enqueueError("invalid_argument", "A valid symbol and interval are required")
			continue
		}
		key := kline.Key{Symbol: market.NormalizeSymbol(*message.Symbol), Interval: market.CandleInterval(*message.Interval)}
		switch message.Type {
		case Subscribe:
			limit := chart.DefaultRange
			if message.Limit != nil {
				limit = *message.Limit
			}
			if limit < 1 || limit > chart.MaxRange {
				client.enqueueKeyError(key, "invalid_argument", "Invalid chart range")
				continue
			}
			var configs []indicator.Selection
			if message.Indicators != nil {
				for _, item := range *message.Indicators {
					configs = append(configs, indicator.Selection{Type: indicator.Type(item.Type), Parameters: indicator.Parameters(item.Parameters)})
				}
			}
			if handler.charts.Validate(configs) != nil {
				client.enqueueKeyError(key, "invalid_argument", "Invalid indicator selection")
				continue
			}
			// The range is set first, so the snapshot of a new subscription
			// builds the chart once. Subscribing again to the same key only
			// changes the chart range.
			subscriber.setRange(request.Context(), key, limit, configs)
			err := handler.service.Subscribe(request.Context(), subscriber, key.Symbol, key.Interval)
			if err != nil {
				subscriber.forget(key)
			}
			switch {
			case errors.Is(err, marketlive.ErrInactiveSymbol):
				client.enqueueKeyError(key, "symbol_not_found", "Symbol is unknown or inactive")
			case errors.Is(err, marketlive.ErrTooManyClientSubscriptions):
				client.enqueueKeyError(key, "subscription_limit", "Subscription limit exceeded")
			case err != nil:
				handler.logger.ErrorContext(request.Context(), "live candle subscription failed", "module", "httpapi", "operation", "subscribe", "error", err)
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

// liveCandleToken reads the session token the client offers after
// liveCandleProtocol in Sec-WebSocket-Protocol.
func liveCandleToken(request *http.Request) (string, bool) {
	protocols := websocket.Subprotocols(request)
	if len(protocols) != 2 || protocols[0] != liveCandleProtocol || protocols[1] == "" {
		return "", false
	}
	return protocols[1], true
}

// authenticate returns the user of a session token; it fails with
// errSessionRequired for an API token.
func (handler *liveCandleHandler) authenticate(ctx context.Context, token string) (auth.User, error) {
	user, err := handler.sessions.Authenticate(ctx, token)
	if err == nil && user.APIToken {
		return auth.User{}, errSessionRequired
	}
	return user, err
}

// watchSession closes the connection once its session is no longer valid. It
// returns when the client closes or ctx is cancelled.
func (handler *liveCandleHandler) watchSession(ctx context.Context, client *liveSocketClient, token string) {
	ticker := time.NewTicker(sessionCheckPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-client.done:
			return
		case <-ticker.C:
			checkCtx, cancel := context.WithTimeout(ctx, sessionCheckTimeout)
			_, err := handler.authenticate(checkCtx, token)
			cancel()
			if ctx.Err() != nil {
				return
			}
			switch {
			case errors.Is(err, auth.ErrUnauthenticated), errors.Is(err, errSessionRequired):
				client.writeError("unauthenticated", sessionInvalidMessage)
				client.Close()
				return
			case err != nil:
				// A failed check, such as a database outage, is retried on
				// the next tick instead of closing every live chart.
				handler.logger.InfoContext(ctx, "live candle session check failed", "module", "httpapi", "operation", "authenticate", "error", err)
			}
		}
	}
}

type reservation int

const (
	reserved reservation = iota
	reservedUserLimit
	reservedServerLimit
)

// reserve takes a connection slot of the user unless the user or the server
// is at its limit.
func (handler *liveCandleHandler) reserve(userID int64) reservation {
	handler.connectionsMu.Lock()
	defer handler.connectionsMu.Unlock()
	switch {
	case handler.userConnections[userID] >= maxLiveConnectionsPerUser:
		return reservedUserLimit
	case handler.connections >= maxLiveConnections:
		return reservedServerLimit
	}
	handler.connections++
	handler.userConnections[userID]++
	return reserved
}

func (handler *liveCandleHandler) release(userID int64) {
	handler.connectionsMu.Lock()
	defer handler.connectionsMu.Unlock()
	handler.connections--
	if handler.userConnections[userID] <= 1 {
		delete(handler.userConnections, userID)
		return
	}
	handler.userConnections[userID]--
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
func (client *liveSocketClient) enqueueKeyError(key kline.Key, code, message string) {
	client.enqueueWire(keyErrorMessage(key, code, message))
}
func keyErrorMessage(key kline.Key, code, message string) LiveCandleServerMessage {
	c := LiveCandleServerMessageCode(code)
	symbol, interval := key.Symbol, CandleInterval(key.Interval)
	return LiveCandleServerMessage{Type: Error, Code: &c, Message: &message, Symbol: &symbol, Interval: &interval}
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
