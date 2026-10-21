package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"crypto-scanner/internal/auth"
	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/indicator"
	indicatortalib "crypto-scanner/internal/indicator/talib"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/market/kline"
	marketlive "crypto-scanner/internal/market/live"

	"github.com/gorilla/websocket"
)

func TestSameWebSocketOrigin(t *testing.T) {
	tests := []struct {
		name   string
		host   string
		origin string
		want   bool
	}{
		{name: "same HTTPS origin", host: "scanner.example", origin: "https://scanner.example", want: true},
		{name: "same development origin", host: "127.0.0.1:3000", origin: "http://127.0.0.1:3000", want: true},
		{name: "foreign origin", host: "scanner.example", origin: "https://attacker.example", want: false},
		{name: "missing origin", host: "scanner.example", want: false},
		{name: "non HTTP scheme", host: "scanner.example", origin: "file://scanner.example", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest("GET", "http://"+test.host+"/api/v1/live/candles", nil)
			request.Host = test.host
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			if got := sameWebSocketOrigin(request); got != test.want {
				t.Fatalf("sameWebSocketOrigin() = %t, want %t", got, test.want)
			}
		})
	}
}

type liveTestSessions struct{}

func (liveTestSessions) Exchange(context.Context, string) (auth.IssuedSession, error) {
	return auth.IssuedSession{}, errors.New("unused")
}

func (liveTestSessions) Authenticate(_ context.Context, token string) (auth.User, error) {
	switch token {
	case "valid":
		return auth.User{ID: 1}, nil
	case "cst_valid":
		return auth.User{ID: 1, APIToken: true}, nil
	}
	return auth.User{}, auth.ErrUnauthenticated
}

func (liveTestSessions) Revoke(context.Context, string) error { return nil }

type liveTestService struct{}

func (liveTestService) RegisterClient(marketlive.Client) bool { return true }
func (liveTestService) Subscribe(context.Context, marketlive.Client, string, market.CandleInterval) error {
	return nil
}
func (liveTestService) Unsubscribe(string, kline.Key) {}
func (liveTestService) RemoveClient(string)           {}

// The handshake authenticates the session token offered after the live
// protocol, so no unauthenticated or API token socket is ever upgraded.
func TestLiveCandleHandshakeAuthenticatesSession(t *testing.T) {
	registry, err := indicator.NewRegistry(indicatortalib.New()...)
	if err != nil {
		t.Fatal(err)
	}
	charts, err := chart.NewService(newChartHistoryStub(), registry, emptyChartCatalog{}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(newLiveCandleHandler(liveTestSessions{}, liveTestService{}, charts, slog.New(slog.DiscardHandler)))
	defer server.Close()
	url := "ws" + strings.TrimPrefix(server.URL, "http")
	tests := []struct {
		name      string
		protocols []string
		status    int
	}{
		{name: "session", protocols: []string{liveCandleProtocol, "valid"}, status: http.StatusSwitchingProtocols},
		{name: "missing token", protocols: []string{liveCandleProtocol}, status: http.StatusUnauthorized},
		{name: "unknown protocol", protocols: []string{"other", "valid"}, status: http.StatusUnauthorized},
		{name: "invalid token", protocols: []string{liveCandleProtocol, "expired"}, status: http.StatusUnauthorized},
		{name: "API token", protocols: []string{liveCandleProtocol, "cst_valid"}, status: http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dialer := websocket.Dialer{Subprotocols: test.protocols, HandshakeTimeout: 2 * time.Second}
			connection, response, err := dialer.Dial(url, http.Header{"Origin": {server.URL}})
			if response == nil {
				t.Fatalf("Dial() error = %v", err)
			}
			if response.StatusCode != test.status {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.status)
			}
			if connection != nil {
				defer connection.Close()
				if connection.Subprotocol() != liveCandleProtocol {
					t.Fatalf("subprotocol = %q, want %q", connection.Subprotocol(), liveCandleProtocol)
				}
			}
		})
	}
}

// Connection limits reject the handshake, so a client at a limit never sees
// an open socket, and a rejected or failed upgrade keeps no slot.
func TestLiveCandleHandshakeEnforcesConnectionLimits(t *testing.T) {
	registry, err := indicator.NewRegistry(indicatortalib.New()...)
	if err != nil {
		t.Fatal(err)
	}
	charts, err := chart.NewService(newChartHistoryStub(), registry, emptyChartCatalog{}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name        string
		connections int
		user        int
		origin      bool
		status      int
	}{
		{name: "user limit", connections: maxLiveConnectionsPerUser, user: maxLiveConnectionsPerUser, origin: true, status: http.StatusTooManyRequests},
		{name: "server limit", connections: maxLiveConnections, origin: true, status: http.StatusServiceUnavailable},
		{name: "failed upgrade", status: http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := newLiveCandleHandler(liveTestSessions{}, liveTestService{}, charts, slog.New(slog.DiscardHandler)).(*liveCandleHandler)
			handler.connections = test.connections
			if test.user > 0 {
				handler.userConnections[1] = test.user
			}
			server := httptest.NewServer(handler)
			defer server.Close()
			header := http.Header{}
			if test.origin {
				header.Set("Origin", server.URL)
			}
			dialer := websocket.Dialer{Subprotocols: []string{liveCandleProtocol, "valid"}, HandshakeTimeout: 2 * time.Second}
			connection, response, err := dialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), header)
			if connection != nil {
				connection.Close()
			}
			if response == nil {
				t.Fatalf("Dial() error = %v", err)
			}
			if response.StatusCode != test.status {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.status)
			}
			// Close waits for the handler, which releases its slot on return.
			server.Close()
			handler.connectionsMu.Lock()
			defer handler.connectionsMu.Unlock()
			if handler.connections != test.connections || handler.userConnections[1] != test.user {
				t.Fatalf("slots = %d (user %d), want %d (user %d)", handler.connections, handler.userConnections[1], test.connections, test.user)
			}
		})
	}
}
