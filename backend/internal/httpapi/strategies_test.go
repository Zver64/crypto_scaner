package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
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

// emptyStats are the statistics of no trades.
const emptyStats = `{"average_bars":null,"average_loss":null,"average_trade":null,"average_win":null,"exit_rule_exits":0,"profit_factor":null,"stop_loss_exits":0,"take_profit_exits":0,"trade_count":0,"win_rate":null}`

func TestBacktestStrategyIsAdministratorOnlyAndMapsMissingResources(t *testing.T) {
	alert := time.Date(2026, 3, 2, 5, 0, 0, 0, time.UTC)
	strategies := &backtestStrategies{result: strategy.Backtest{
		Interval: market.IntervalHour, Symbol: "BTCUSDT", Direction: strategy.DirectionLong, From: alert.Add(-5 * time.Hour), To: alert.Add(5 * time.Hour),
		Trades: []strategy.Trade{{
			EntryTime: alert.Add(time.Hour), EntryPrice: 100, ExitTime: alert.Add(2 * time.Hour), ExitPrice: 110, Buys: 1, Return: 0.098,
			Fills:      []strategy.Fill{{Signal: alert, Time: alert.Add(time.Hour), Price: 100, Values: map[string]float64{"h_close": 99}}},
			TakeProfit: 110, Reason: strategy.ExitTakeProfit, ExitSignal: alert.Add(2 * time.Hour),
		}},
		Skipped: 2, NetProfit: 0.098, Equity: []strategy.EquityPoint{{Time: alert.Add(2 * time.Hour), Equity: 1.098}},
		Stats:      strategy.TradeStats{Count: 1, WinRate: new(1.0), AverageTrade: new(0.098), AverageWin: new(0.098)},
		BuyAndHold: new(0.05), DCA: new(0.03),
	}}
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
		{name: "too heavy", token: "admin", target: "/api/v1/admin/strategies/5/backtest?symbol=BTCUSDT", status: http.StatusServiceUnavailable, code: "backtest_too_heavy"},
		{name: "busy", token: "admin", target: "/api/v1/admin/strategies/6/backtest?symbol=BTCUSDT", status: http.StatusTooManyRequests, code: "backtest_busy"},
		{name: "administrator", token: "admin", target: "/api/v1/admin/strategies/1/backtest?symbol=btcusdt&from=2026-03-02T03:00:00%2B02:00", status: http.StatusOK},
		{
			name: "empty history", token: "admin", target: "/api/v1/admin/strategies/3/backtest?symbol=BTCUSDT", status: http.StatusOK,
			body: `{"baselines":{"buy_and_hold":null,"dca":null},"direction":"short","equity":[],"fee":0.001,"from":null,"interval":"1h",` +
				`"signal":null,"skipped_alerts":0,"summary":{"max_drawdown":0,"net_profit":0,"stats":` + emptyStats + `},"symbol":"BTCUSDT","to":null,"trades":[]}`,
		},
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
				assertErrorCode(t, response, test.code)
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
			replayed := strategies.result
			if body.Interval != httpapi.CandleInterval(market.IntervalHour) || body.From == nil || !body.From.Equal(replayed.From) || body.To == nil || !body.To.Equal(replayed.To) ||
				strategies.symbol != "BTCUSDT" || body.Fee != strategy.BacktestFee || body.SkippedAlerts != 2 ||
				!strategies.from.Equal(alert.Add(-4*time.Hour)) || strategies.from.Location() != time.UTC || !strategies.to.IsZero() ||
				len(body.Trades) != 1 || body.Trades[0].ExitPrice != 110 || body.Trades[0].Buys != 1 || body.Trades[0].Open || body.Trades[0].NetReturn != 0.098 ||
				*body.Trades[0].TakeProfit != 110 || body.Trades[0].StopLoss != nil || *body.Trades[0].ExitReason != httpapi.BacktestTradeExitReasonTakeProfit ||
				len(body.Trades[0].Fills) != 1 || body.Trades[0].Fills[0].Values["h_close"] != 99 || len(body.Trades[0].ExitValues) != 0 ||
				len(body.Equity) != 1 || body.Equity[0].Equity != 1.098 ||
				body.Summary.NetProfit != 0.098 || body.Summary.Stats.TradeCount != 1 || body.Summary.Stats.AverageLoss != nil || *body.Summary.Stats.WinRate != 1 ||
				*body.Baselines.BuyAndHold != 0.05 || *body.Baselines.Dca != 0.03 {
				t.Fatalf("body = %s, symbol = %s", response.Body.String(), strategies.symbol)
			}
		})
	}
}

// A draft is backtested from its body as it is, also with an API token.
func TestBacktestStrategyDraftPassesTheBodyAndMapsFailures(t *testing.T) {
	strategies := &backtestStrategies{result: strategy.Backtest{Interval: market.IntervalHour, Symbol: "BTCUSDT", Direction: strategy.DirectionSideways}}
	handler := httpapi.New(logging.New(io.Discard, "error", logging.Options{}), httpapi.Dependencies{
		Readiness: readinessStub{}, Analysis: unavailableAnalysis{}, Sessions: roleSessions{}, Strategies: strategies,
	}, httpapi.Options{})
	signal := `"signal":true,"direction":"sideways","exit_expression":"","take_profit_expression":"","stop_loss_expression":"","target_ratio":3,"window":12,"symbol":"btcusdt"`
	for _, test := range []struct {
		name   string
		token  string
		body   string
		status int
		code   string
	}{
		{name: "user", token: "user", body: `{"expression":"h_close > 1",` + signal + `}`, status: http.StatusForbidden, code: "administrator_required"},
		{name: "unknown field", token: "admin", body: `{"expression":"h_close > 1","name":"Draft",` + signal + `}`, status: http.StatusBadRequest, code: "invalid_argument"},
		{name: "invalid", token: "admin", body: `{"expression":"invalid",` + signal + `}`, status: http.StatusBadRequest, code: "invalid_argument"},
		{name: "busy", token: "admin", body: `{"expression":"busy",` + signal + `}`, status: http.StatusTooManyRequests, code: "backtest_busy"},
		{name: "too heavy", token: "admin", body: `{"expression":"heavy",` + signal + `}`, status: http.StatusServiceUnavailable, code: "backtest_too_heavy"},
		{name: "api token", token: "program", body: `{"expression":"h_close > 1","to":"2026-03-02T03:00:00+02:00",` + signal + `}`, status: http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/strategy-backtests", strings.NewReader(test.body))
			request.Header.Set("Authorization", "Bearer "+test.token)
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			if test.code != "" {
				assertErrorCode(t, response, test.code)
				return
			}
			want := strategy.Strategy{Signal: true, Direction: strategy.DirectionSideways, Expression: "h_close > 1", TargetRatio: 3, Window: 12}
			if strategies.draft != want || strategies.symbol != "BTCUSDT" || !strategies.from.IsZero() ||
				!strategies.to.Equal(time.Date(2026, 3, 2, 1, 0, 0, 0, time.UTC)) || strategies.to.Location() != time.UTC {
				t.Fatalf("draft = %+v, symbol %s, from %v, to %v", strategies.draft, strategies.symbol, strategies.from, strategies.to)
			}
		})
	}
}

// backtestStrategies knows strategy 1 and the coin BTCUSDT; strategy 3 has no
// history, strategy 5 runs out of time, and strategy 6 finds no free slot.
// Drafts fail by their expression: invalid, heavy, or busy.
type backtestStrategies struct {
	httpapi.Strategies
	result   strategy.Backtest
	draft    strategy.Strategy
	symbol   string
	from, to time.Time
}

func (strategies *backtestStrategies) BacktestDraft(_ context.Context, item strategy.Strategy, symbol string, from, to time.Time) (strategy.Backtest, error) {
	strategies.draft, strategies.symbol, strategies.from, strategies.to = item, market.NormalizeSymbol(symbol), from, to
	switch item.Expression {
	case "invalid":
		return strategy.Backtest{}, fmt.Errorf("%w: unknown variable", strategy.ErrInvalidArgument)
	case "heavy":
		return strategy.Backtest{}, fmt.Errorf("replay: %w", context.DeadlineExceeded)
	case "busy":
		return strategy.Backtest{}, strategy.ErrBacktestBusy
	}
	return strategies.result, nil
}

func (strategies *backtestStrategies) Backtest(_ context.Context, id int64, symbol string, from, to time.Time) (strategy.Backtest, error) {
	symbol = market.NormalizeSymbol(symbol)
	strategies.symbol, strategies.from, strategies.to = symbol, from, to
	switch {
	case id == 5:
		return strategy.Backtest{}, fmt.Errorf("replay: %w", context.DeadlineExceeded)
	case id == 6:
		return strategy.Backtest{}, strategy.ErrBacktestBusy
	case id == 3:
		return strategy.Backtest{Interval: market.IntervalHour, Symbol: symbol, Direction: strategy.DirectionShort}, nil
	case id != 1:
		return strategy.Backtest{}, strategy.ErrNotFound
	case symbol != "BTCUSDT":
		return strategy.Backtest{}, market.ErrInstrumentNotFound
	}
	return strategies.result, nil
}

// roleSessions authenticates the token "admin" as the administrator, "user"
// as another user, and "program" as the administrator's API token.
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
	case "program":
		return auth.User{ID: 1, TelegramID: 1, Administrator: true, APIToken: true}, nil
	}
	return auth.User{}, auth.ErrUnauthenticated
}

func (roleSessions) Revoke(context.Context, string) error { return nil }

func assertErrorCode(t *testing.T, response *httptest.ResponseRecorder, code string) {
	t.Helper()
	var body httpapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || string(body.Error.Code) != code {
		t.Fatalf("error = %s, want %s", response.Body.String(), code)
	}
}

// A request the contract rejects names the parameter or body field and why,
// without kin-openapi's schema and value dumps.
func TestContractViolationsNameTheirField(t *testing.T) {
	handler := httpapi.New(logging.New(io.Discard, "error", logging.Options{}), httpapi.Dependencies{
		Readiness: readinessStub{}, Analysis: unavailableAnalysis{}, Sessions: roleSessions{}, Strategies: &backtestStrategies{},
	}, httpapi.Options{})
	for _, test := range []struct {
		method, target, body, want string
	}{
		{method: http.MethodGet, target: "/api/v1/admin/strategies/0/backtest?symbol=BTCUSDT", want: `Invalid path parameter "strategy_id": minimum: got 0, want 1`},
		{method: http.MethodGet, target: "/api/v1/admin/strategies/abc/backtest?symbol=BTCUSDT", want: `Invalid path parameter "strategy_id": an invalid integer`},
		{method: http.MethodGet, target: "/api/v1/admin/strategies/" + strings.Repeat("x", 1000) + "/backtest?symbol=BTCUSDT", want: `Invalid path parameter "strategy_id": an invalid integer`},
		{method: http.MethodGet, target: "/api/v1/admin/strategies/1/backtest?symbol=BTC%2FUSDT", want: `Invalid query parameter "symbol": 'BTC/USDT' does not match pattern '^[A-Za-z0-9]+$'`},
		{method: http.MethodPost, target: "/api/v1/admin/strategies", body: `{"name":"","expression":"h_close > 1","message":""}`, want: `Invalid request body field "name": minimum string length is 1`},
	} {
		t.Run(test.target, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.target, strings.NewReader(test.body))
			request.Header.Set("Authorization", "Bearer admin")
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			var body httpapi.ErrorResponse
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || response.Code != http.StatusBadRequest || body.Error.Code != "invalid_argument" ||
				body.Error.Message != test.want {
				t.Fatalf("status = %d, body = %s, want message %q", response.Code, response.Body.String(), test.want)
			}
		})
	}
}
