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
	if token != "valid" {
		return auth.User{}, auth.ErrUnauthenticated
	}
	return auth.User{ID: 1}, nil
}

func (liveTestSessions) Revoke(context.Context, string) error { return nil }

type liveTestService struct{}

func (liveTestService) RegisterClient(marketlive.Client) bool { return true }
func (liveTestService) Subscribe(context.Context, marketlive.Client, string, market.CandleInterval) error {
	return nil
}
func (liveTestService) Unsubscribe(string, kline.Key) {}
func (liveTestService) RemoveClient(string)           {}

// A socket that has not authenticated never takes a live connection slot, so
// it cannot keep users out of their charts.
func TestUnauthenticatedSocketsDoNotTakeLiveConnectionSlots(t *testing.T) {
	registry, err := indicator.NewRegistry(indicatortalib.New()...)
	if err != nil {
		t.Fatal(err)
	}
	charts, err := chart.NewService(newChartHistoryStub(), registry, emptyChartCatalog{}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	handler := newLiveCandleHandler(liveTestSessions{}, liveTestService{}, charts, slog.New(slog.DiscardHandler)).(*liveCandleHandler)
	handler.connections = make(chan struct{}, 1)
	server := httptest.NewServer(handler)
	defer server.Close()
	dial := func() *websocket.Conn {
		t.Helper()
		url := "ws" + strings.TrimPrefix(server.URL, "http")
		connection, _, err := websocket.DefaultDialer.Dial(url, http.Header{"Origin": {server.URL}})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = connection.Close() })
		return connection
	}

	// The silent socket waits for authentication while a user connects.
	dial()
	connection := dial()
	if err := connection.WriteJSON(map[string]any{"type": "authenticate", "token": "valid"}); err != nil {
		t.Fatal(err)
	}
	_ = connection.SetReadDeadline(time.Now().Add(2 * time.Second))
	var message LiveCandleServerMessage
	if err := connection.ReadJSON(&message); err != nil || message.Type != Authenticated {
		t.Fatalf("first message = %+v, error %v; want authenticated", message, err)
	}
}
