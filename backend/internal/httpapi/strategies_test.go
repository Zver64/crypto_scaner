package httpapi_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"crypto-scanner/internal/auth"
	"crypto-scanner/internal/httpapi"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/platform/logging"
	"crypto-scanner/internal/strategy"
)

func TestBacktestStrategyIsAdministratorOnlyAndMapsMissingResources(t *testing.T) {
	alert := time.Date(2026, 3, 2, 5, 0, 0, 0, time.UTC)
	strategies := &backtestStrategies{result: strategy.Backtest{Interval: market.IntervalHour, From: alert.Add(-5 * time.Hour), To: alert.Add(5 * time.Hour), Alerts: []time.Time{alert}}}
	handler := httpapi.New(logging.New(io.Discard, "error", logging.Options{}), httpapi.Dependencies{
		Readiness: readinessStub{}, Analysis: unavailableAnalysis{}, Sessions: roleSessions{}, Strategies: strategies,
	}, httpapi.Options{})
	for _, test := range []struct {
		name   string
		token  string
		target string
		status int
		code   string
		// body is the exact response of an empty history.
		body string
	}{
		{name: "user", token: "user", target: "/api/v1/admin/strategies/1/backtest?symbol=btcusdt", status: http.StatusForbidden, code: "administrator_required"},
		{name: "unknown strategy", token: "admin", target: "/api/v1/admin/strategies/2/backtest?symbol=BTCUSDT", status: http.StatusNotFound, code: "strategy_not_found"},
		{name: "unknown symbol", token: "admin", target: "/api/v1/admin/strategies/1/backtest?symbol=ETHUSDT", status: http.StatusNotFound, code: "symbol_not_found"},
		{name: "administrator", token: "admin", target: "/api/v1/admin/strategies/1/backtest?symbol=btcusdt", status: http.StatusOK},
		{name: "empty history", token: "admin", target: "/api/v1/admin/strategies/3/backtest?symbol=BTCUSDT", status: http.StatusOK, body: `{"alerts":[],"from":null,"interval":"1h","to":null}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.target, nil)
			request.Header.Set("Authorization", "Bearer "+test.token)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			if test.code != "" {
				var envelope struct {
					Error struct {
						Code string `json:"code"`
					} `json:"error"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil || envelope.Error.Code != test.code {
					t.Fatalf("body = %s, want code %s", response.Body.String(), test.code)
				}
				if test.status == http.StatusNotFound && response.Header().Get("X-Request-ID") == "" {
					t.Fatal("not found response lacks X-Request-ID")
				}
				return
			}
			if test.body != "" {
				if got := strings.TrimSpace(response.Body.String()); got != test.body {
					t.Fatalf("body = %s, want %s", got, test.body)
				}
				return
			}
			var body httpapi.StrategyBacktest
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Interval != httpapi.CandleInterval(market.IntervalHour) || body.From == nil || !body.From.Equal(strategies.result.From) || body.To == nil || !body.To.Equal(strategies.result.To) ||
				len(body.Alerts) != 1 || !body.Alerts[0].OpenTime.Equal(alert) || strategies.symbol != "BTCUSDT" {
				t.Fatalf("body = %s, symbol = %s", response.Body.String(), strategies.symbol)
			}
		})
	}
}

// backtestStrategies knows strategy 1 and the coin BTCUSDT; strategy 3 has no
// history.
type backtestStrategies struct {
	httpapi.Strategies
	result strategy.Backtest
	symbol string
}

func (strategies *backtestStrategies) Backtest(_ context.Context, id int64, symbol string) (strategy.Backtest, error) {
	strategies.symbol = symbol
	switch {
	case id == 3:
		return strategy.Backtest{Interval: market.IntervalHour}, nil
	case id != 1:
		return strategy.Backtest{}, strategy.ErrNotFound
	case symbol != "BTCUSDT":
		return strategy.Backtest{}, market.ErrInstrumentNotFound
	}
	return strategies.result, nil
}

// roleSessions authenticates the token "admin" as the administrator and
// "user" as another user.
type roleSessions struct{}

func (roleSessions) Exchange(context.Context, string) (auth.IssuedSession, error) {
	return auth.IssuedSession{}, auth.ErrUnauthenticated
}

func (roleSessions) Authenticate(_ context.Context, token string) (auth.User, error) {
	switch token {
	case "admin":
		return auth.User{ID: 1, TelegramID: 1, Administrator: true}, nil
	case "user":
		return auth.User{ID: 2, TelegramID: 2}, nil
	}
	return auth.User{}, auth.ErrUnauthenticated
}

func (roleSessions) Revoke(context.Context, string) error { return nil }
