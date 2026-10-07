package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"crypto-scanner/internal/apiclient"
)

const testToken = "cst_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

// fakeAPI answers the CLI's operations with canned bodies and records the
// requests it receives.
type fakeAPI struct {
	t         *testing.T
	responses map[string]string // "METHOD path" → JSON body
	status    int
	bodies    map[string]string
}

func newFakeAPI(t *testing.T, responses map[string]string) (*fakeAPI, *httptest.Server) {
	api := &fakeAPI{t: t, responses: responses, status: http.StatusOK, bodies: map[string]string{}}
	server := httptest.NewServer(api)
	t.Cleanup(server.Close)
	return api, server
}

func (api *fakeAPI) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Type", "application/json")
	if request.Header.Get("Authorization") != "Bearer "+testToken {
		response.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(response, `{"error":{"code":"unauthenticated","message":"Session is invalid or expired"},"request_id":"r"}`)
		return
	}
	key := request.Method + " " + request.URL.RequestURI()
	raw, _ := io.ReadAll(request.Body)
	api.bodies[key] = string(raw)
	body, ok := api.responses[key]
	if !ok {
		api.t.Errorf("unexpected request %s", key)
		response.WriteHeader(http.StatusNotFound)
		return
	}
	status := api.status
	if status == http.StatusOK && request.Method == http.MethodPost && !strings.HasSuffix(request.URL.Path, "-validations") {
		status = http.StatusCreated
	}
	response.WriteHeader(status)
	_, _ = io.WriteString(response, body)
}

type outcome struct {
	code           int
	stdout, stderr string
}

func runCLI(t *testing.T, configHome, stdin string, args ...string) outcome {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("SCANNER_PROFILE", "")
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), args, strings.NewReader(stdin), &stdout, &stderr)
	if strings.Contains(stdout.String()+stderr.String(), testToken) {
		t.Fatalf("scanner %v printed the token", args)
	}
	return outcome{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// writeProfile saves a current profile for server without logging in.
func writeProfile(t *testing.T, server string) string {
	t.Helper()
	home := t.TempDir()
	if err := saveConfig(filepath.Join(home, "scanner", "config.json"), config{Current: "dev", Profiles: map[string]profile{"dev": {Server: server, Token: testToken}}}); err != nil {
		t.Fatal(err)
	}
	return home
}

func TestLoginSavesPrivateProfilesAndUseSwitchesThem(t *testing.T) {
	_, server := newFakeAPI(t, map[string]string{"GET /api/v1/me": `{"administrator":true}`})
	home := t.TempDir()

	for _, name := range []string{"dev", "prod"} {
		if got := runCLI(t, home, testToken+"\n", "login", name, "--server", server.URL+"/"); got.code != 0 {
			t.Fatalf("login %s = %+v", name, got)
		}
	}

	path := filepath.Join(home, "scanner", "config.json")
	for file, want := range map[string]os.FileMode{path: 0o600, filepath.Dir(path): 0o700} {
		if info, err := os.Stat(file); err != nil || info.Mode().Perm() != want {
			t.Fatalf("mode of %s = %v, %v; want %v", file, info.Mode().Perm(), err, want)
		}
	}
	if got := runCLI(t, home, "", "profiles"); got.stdout != "* dev "+server.URL+"\n  prod "+server.URL+"\n" {
		t.Fatalf("profiles = %q; the first login stays current", got.stdout)
	}
	if got := runCLI(t, home, "", "use", "prod"); got.code != 0 {
		t.Fatalf("use = %+v", got)
	}
	if got := runCLI(t, home, "", "use", "staging"); got.code != 1 {
		t.Fatalf("use of an unknown profile = %+v", got)
	}
	if got := runCLI(t, home, "", "profiles"); !strings.HasPrefix(got.stdout, "  dev") || !strings.Contains(got.stdout, "* prod") {
		t.Fatalf("profiles after use = %q", got.stdout)
	}
}

func TestLoginRejectsTokensThatCannotServeTheCLI(t *testing.T) {
	for _, test := range []struct {
		name, stdin, me string
	}{
		{name: "not an API token", stdin: "session-token", me: `{"administrator":true}`},
		{name: "not the administrator", stdin: testToken, me: `{"administrator":false}`},
		{name: "unknown token", stdin: "cst_other", me: `{"administrator":true}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, server := newFakeAPI(t, map[string]string{"GET /api/v1/me": test.me})
			home := t.TempDir()

			got := runCLI(t, home, test.stdin, "login", "dev", "--server", server.URL)

			if got.code != 1 || got.stderr == "" {
				t.Fatalf("login = %+v, want a failure", got)
			}
			if _, err := os.Stat(filepath.Join(home, "scanner", "config.json")); !os.IsNotExist(err) {
				t.Fatalf("profile saved after a failed login: %v", err)
			}
		})
	}
}

func TestBacktestRendersTheTradesOfASavedStrategy(t *testing.T) {
	_, server := newFakeAPI(t, map[string]string{
		"GET /api/v1/admin/strategies/3/backtest?symbol=BTCUSDT&hold=2": backtestResponse,
	})
	home := writeProfile(t, server.URL)

	if got := runCLI(t, home, "", "backtest", "--strategy", "3", "--symbol", "BTCUSDT", "--hold", "2"); got.code != 0 || got.stdout != backtestText {
		t.Errorf("backtest = %+v\nwant:\n%s", got, backtestText)
	}
	if raw := runCLI(t, home, "", "--json", "backtest", "--strategy", "3", "--symbol", "BTCUSDT", "--hold", "2"); raw.stdout != backtestResponse+"\n" {
		t.Errorf("--json output = %q", raw.stdout)
	}
}

const backtestResponse = `{"baselines":{"buy_and_hold":0.25,"every_candle":{"average_loss":-0.011,"average_trade":0.0004,"average_win":0.012,"profit_factor":1.04,"trade_count":4000,"win_rate":0.5123}},` +
	`"equity":[{"equity":1.03,"time":"2024-02-01T07:00:00Z"},{"equity":1.0094,"time":"2024-06-30T22:00:00Z"}],"fee":0.001,"from":"2024-01-01T00:00:00Z","hold":2,"interval":"1h",` +
	`"skipped_alerts":1,"summary":{"max_drawdown":0.02,"net_profit":0.0094,"stats":{"average_loss":-0.02,"average_trade":0.005,"average_win":0.03,"profit_factor":1.5,"trade_count":2,"win_rate":0.5}},` +
	`"symbol":"BTCUSDT","to":"2024-06-30T23:00:00Z","trades":[` +
	`{"entry_price":42000.5,"entry_time":"2024-02-01T06:00:00Z","exit_price":43303.15,"exit_time":"2024-02-01T07:00:00Z","net_return":0.03},` +
	`{"entry_price":0.00001234,"entry_time":"2024-06-30T21:00:00Z","exit_price":0.0000121,"exit_time":"2024-06-30T22:00:00Z","net_return":-0.02}],"unfinished_trades":1}`

const backtestText = `┌────────────────┬────────────────────────────────────────────────┐
│ Coin           │ BTCUSDT                                        │
│ Period         │ 2024-01-01 → 2024-06-30 (1h candles)           │
│ Hold           │ 2 candles                                      │
│ Fee            │ 0.1% per buy and per sell                      │
│ Skipped alerts │ 1 (a trade was open)                           │
│ Unfinished     │ 1 (history ended or had a gap before the exit) │
└────────────────┴────────────────────────────────────────────────┘
┌────────────────┬──────────┬────────────────────┐
│ METRIC         │ STRATEGY │ COMPARED WITH      │
├────────────────┼──────────┼────────────────────┤
│ Net profit %   │    +0.94 │ Buy & Hold +25.00  │
│ Trades         │        2 │ Every candle 4000  │
│ Win rate %     │       50 │ Every candle 51.23 │
│ Profit factor  │     1.50 │ Every candle 1.04  │
│ Max drawdown % │     2.00 │ -                  │
└────────────────┴──────────┴────────────────────┘
┌─────────────┬──────────┬──────────────┐
│ METRIC      │ STRATEGY │ EVERY CANDLE │
├─────────────┼──────────┼──────────────┤
│ Avg trade % │    +0.50 │        +0.04 │
│ Avg win %   │    +3.00 │        +1.20 │
│ Avg loss %  │    -2.00 │        -1.10 │
└─────────────┴──────────┴──────────────┘
Trades: buy at the open of the candle after an alert, sell at the close of the last held candle; returns are after fees.
┌───┬──────────────────┬────────────┬──────────────────┬───────────┬───────┐
│   │ ENTRY            │ PRICE      │ EXIT             │ PRICE     │ NET % │
├───┼──────────────────┼────────────┼──────────────────┼───────────┼───────┤
│ 1 │ 2024-06-30 21:00 │ 0.00001234 │ 2024-06-30 22:00 │ 0.0000121 │ -2.00 │
│ 2 │ 2024-02-01 06:00 │    42000.5 │ 2024-02-01 07:00 │  43303.15 │ +3.00 │
└───┴──────────────────┴────────────┴──────────────────┴───────────┴───────┘
`

// The trade list shows the newest maxTradeRows trades, newest first.
func TestBacktestCapsTheTradeList(t *testing.T) {
	trades := make([]apiclient.BacktestTrade, maxTradeRows+3)
	for i := range trades {
		trades[i].EntryPrice = float64(i)
	}
	var output strings.Builder

	renderTrades(&output, apiclient.N1d, trades)

	lines := strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n")
	if len(lines) != maxTradeRows+6 || !strings.Contains(lines[4], fmt.Sprintf(" %d │", maxTradeRows+2)) || lines[len(lines)-1] != "… 3 more (use --json)" {
		t.Fatalf("output = %q", output.String())
	}
}

// A strategy is created disabled, after the indicators it reads that are not
// configured are added.
func TestStrategiesCreateAddsMissingIndicatorsAndSavesADisabledStrategy(t *testing.T) {
	api, server := newFakeAPI(t, map[string]string{
		"POST /api/v1/admin/strategy-validations":      `{"errors":[],"missing_indicators":[{"interval":"1h","type":"atr","parameters":{"period":100},"title":"h-atr-100"}]}`,
		"POST /api/v1/admin/scanner-indicator-batches": `{"items":[]}`,
		"POST /api/v1/admin/strategies":                `{"id":7,"name":"ATR","expression":"h_atr_100 > 1","message":"","enabled":false,"valid":true}`,
	})
	home := writeProfile(t, server.URL)

	got := runCLI(t, home, "", "strategies", "create", "ATR", "--expr", "h_atr_100 > 1")

	want := `┌────┬───────┬──────┬───────────────┐
│ ID │ STATE │ NAME │ EXPRESSION    │
├────┼───────┼──────┼───────────────┤
│  7 │ off   │ ATR  │ h_atr_100 > 1 │
└────┴───────┴──────┴───────────────┘
`
	if got.code != 0 || got.stdout != want {
		t.Fatalf("strategies create = %+v", got)
	}
	for key, want := range map[string]string{
		"POST /api/v1/admin/scanner-indicator-batches": `{"items":[{"interval":"1h","parameters":{"period":100},"type":"atr"}]}`,
		"POST /api/v1/admin/strategies":                `{"enabled":false,"expression":"h_atr_100 \u003e 1","message":"","name":"ATR"}`,
	} {
		var sent, expected any
		if json.Unmarshal([]byte(api.bodies[key]), &sent) != nil || json.Unmarshal([]byte(want), &expected) != nil || !reflect.DeepEqual(sent, expected) {
			t.Errorf("%s body = %s, want %s", key, api.bodies[key], want)
		}
	}
}

func TestCommandsRenderCompactText(t *testing.T) {
	_, server := newFakeAPI(t, map[string]string{
		"GET /api/v1/admin/strategy-variables":    `{"items":[{"name":"d_rsi","label":"d-rsi","interval":"1d","indicator_id":1},{"name":"h_close","label":"h-close","interval":"1h"}]}`,
		"POST /api/v1/admin/strategy-validations": `{"errors":[],"missing_indicators":[{"interval":"1h","type":"atr","parameters":{"period":100},"title":"h-atr-100"}]}`,
		"GET /api/v1/favorites":                   `{"items":[{"symbol":"BTCUSDT","base_asset":"BTC","quote_asset":"USDT","active":true,"alert_count":0,"created_at":"2024-01-01T00:00:00Z"},{"symbol":"OLDUSDT","base_asset":"OLD","quote_asset":"USDT","active":false,"alert_count":0,"created_at":"2024-01-01T00:00:00Z"}]}`,
		"GET /api/v1/admin/strategies":            `{"items":[{"id":3,"name":"Dip buy","expression":"d_rsi < 30 &&\n  h_close > 1","message":"","enabled":true,"valid":true},{"id":4,"name":"Old","expression":"x","message":"","enabled":false,"valid":false,"problem":"undeclared"}]}`,
	})
	home := writeProfile(t, server.URL)

	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"vars", "--filter", "RSI"}, want: "d_rsi\n"},
		{args: []string{"validate", "h_atr_100 > 1"}, want: "ok; indicators will be added when saving: h-atr-100\n"},
		{args: []string{"favorites"}, want: "BTCUSDT OLDUSDT(inactive)\n"},
		{args: []string{"strategies"}, want: `┌────┬─────────┬─────────┬───────────────────────────┐
│ ID │ STATE   │ NAME    │ EXPRESSION                │
├────┼─────────┼─────────┼───────────────────────────┤
│  3 │ on      │ Dip buy │ d_rsi < 30 && h_close > 1 │
│  4 │ invalid │ Old     │ x                         │
└────┴─────────┴─────────┴───────────────────────────┘
`},
	} {
		if got := runCLI(t, home, "", test.args...); got.code != 0 || got.stdout != test.want {
			t.Errorf("scanner %v = %+v, want %q", test.args, got, test.want)
		}
	}
}

func TestErrorsExitWithoutPrintingTheToken(t *testing.T) {
	api, server := newFakeAPI(t, map[string]string{
		"POST /api/v1/admin/strategy-validations":             `{"errors":["undeclared reference to 'x'"],"missing_indicators":[]}`,
		"GET /api/v1/admin/strategies/3/backtest?symbol=NOPE": `{"error":{"code":"symbol_not_found","message":"Symbol is unknown"},"request_id":"r"}`,
	})
	home := writeProfile(t, server.URL)

	if got := runCLI(t, home, "", "validate", "x > 1"); got.code != 1 || got.stdout != "error: undeclared reference to 'x'\n" {
		t.Errorf("invalid expression = %+v", got)
	}
	api.status = http.StatusNotFound
	if got := runCLI(t, home, "", "backtest", "--strategy", "3", "--symbol", "NOPE"); got.code != 1 || got.stderr != "scanner: symbol_not_found: Symbol is unknown\n" {
		t.Errorf("API error = %+v", got)
	}
	if err := saveConfig(filepath.Join(home, "scanner", "config.json"), config{Current: "dev", Profiles: map[string]profile{"dev": {Server: server.URL, Token: "cst_revoked"}}}); err != nil {
		t.Fatal(err)
	}
	if got := runCLI(t, home, "", "favorites"); got.code != 1 || !strings.Contains(got.stderr, "unauthenticated") || strings.Contains(got.stderr, "cst_revoked") {
		t.Errorf("revoked token = %+v", got)
	}
}

// A proxy's HTML success page and an unexpected 2xx must be ordinary
// errors, not nil success DTO dereferences (also with --json).
func TestCommandsRejectUnexpectedSuccessResponses(t *testing.T) {
	for _, test := range []struct {
		name, target string
		args         []string
		status       int
	}{
		{name: "login", target: "/api/v1/me", args: []string{"login", "dev"}, status: http.StatusOK},
		{name: "vars", target: "/api/v1/admin/strategy-variables", args: []string{"vars"}, status: http.StatusOK},
		{name: "validate", target: "/api/v1/admin/strategy-validations", args: []string{"validate", "h_close > 1"}, status: http.StatusOK},
		{name: "favorites", target: "/api/v1/favorites", args: []string{"favorites"}, status: http.StatusOK},
		{name: "strategies", target: "/api/v1/admin/strategies", args: []string{"strategies"}, status: http.StatusOK},
		{name: "add indicators", target: "/api/v1/admin/scanner-indicator-batches", args: []string{"strategies", "create", "Test", "--expr", "h_atr_100 > 1"}, status: http.StatusCreated},
		{name: "create", target: "/api/v1/admin/strategies", args: []string{"strategies", "create", "Test", "--expr", "h_close > 1"}, status: http.StatusCreated},
		{name: "backtest", target: "/api/v1/admin/strategies/3/backtest", args: []string{"backtest", "--strategy", "3", "--symbol", "BTCUSDT"}, status: http.StatusOK},
		{name: "delete", target: "/api/v1/admin/strategies/3", args: []string{"strategies", "delete", "3"}, status: http.StatusOK},
	} {
		for _, html := range []bool{true, false} {
			t.Run(test.name+"/html="+strconv.FormatBool(html), func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					if r.URL.Path != test.target {
						// The create command validates before its writes.
						if r.URL.Path != "/api/v1/admin/strategy-validations" {
							t.Errorf("unexpected request %s", r.URL.Path)
						}
						missing := "[]"
						if test.name == "add indicators" {
							missing = `[{"interval":"1h","type":"atr","parameters":{"period":100},"title":"h-atr-100"}]`
						}
						fmt.Fprintf(w, `{"errors":[],"missing_indicators":%s}`, missing)
						return
					}
					if html {
						w.Header().Set("Content-Type", "text/html")
						w.WriteHeader(test.status)
						fmt.Fprint(w, "<html>proxy page</html>")
					} else {
						w.WriteHeader(http.StatusAccepted)
						fmt.Fprint(w, "{}")
					}
				}))
				defer server.Close()
				home := writeProfile(t, server.URL)
				args := append([]string{"--json"}, test.args...)
				if test.name == "login" {
					args = append(args, "--server", server.URL)
				}
				got := runCLI(t, home, testToken, args...)
				if got.code != 1 || got.stdout != "" || !strings.HasPrefix(got.stderr, "scanner: ") || !strings.Contains(got.stderr, "unexpected API response") {
					t.Fatalf("unexpected success response = %+v", got)
				}
			})
		}
	}
}

func TestDeleteStrategy(t *testing.T) {
	api, server := newFakeAPI(t, map[string]string{"DELETE /api/v1/admin/strategies/9223372036854775807": ""})
	api.status = http.StatusNoContent
	home := writeProfile(t, server.URL)

	for _, jsonOutput := range []bool{false, true} {
		clear(api.bodies)
		args := []string{"strategies", "delete", "9223372036854775807"}
		want := "deleted strategy 9223372036854775807\n"
		if jsonOutput {
			args = append(args, "--json")
			want = ""
		}
		got := runCLI(t, home, "", args...)
		if got.code != 0 || got.stdout != want || got.stderr != "" {
			t.Fatalf("delete = %+v, want stdout %q", got, want)
		}
		if body, ok := api.bodies["DELETE /api/v1/admin/strategies/9223372036854775807"]; !ok || body != "" || len(api.bodies) != 1 {
			t.Fatalf("delete requests = %v", api.bodies)
		}
	}
}

func TestDeleteStrategyReportsAPIErrors(t *testing.T) {
	for _, test := range []struct {
		status     int
		body, want string
	}{
		{http.StatusBadRequest, `{"error":{"code":"invalid_request","message":"Invalid ID"}}`, "HTTP 400"},
		{http.StatusNotFound, `{"error":{"code":"strategy_not_found","message":"Strategy not found"}}`, "strategy_not_found: Strategy not found"},
		{http.StatusUnauthorized, `{"error":{"code":"unauthenticated","message":"Session is invalid or expired"}}`, "unauthenticated: Session is invalid or expired (revoked or wrong token? run scanner login)"},
		{http.StatusForbidden, `{"error":{"code":"administrator_required","message":"Administrator access required"}}`, "administrator_required: Administrator access required"},
		{http.StatusInternalServerError, `{"error":{"code":"internal_error","message":"Internal server error"}}`, "internal_error: Internal server error"},
	} {
		t.Run(strconv.Itoa(test.status), func(t *testing.T) {
			api, server := newFakeAPI(t, map[string]string{"DELETE /api/v1/admin/strategies/3": test.body})
			api.status = test.status
			got := runCLI(t, writeProfile(t, server.URL), "", "strategies", "delete", "3", "--json")
			if got.code != 1 || got.stdout != "" || got.stderr != "scanner: "+test.want+"\n" {
				t.Fatalf("delete API error = %+v", got)
			}
		})
	}
}

func TestLoginRefusesCleartextToOtherHosts(t *testing.T) {
	got := runCLI(t, t.TempDir(), testToken, "login", "prod", "--server", "http://scanner.example")
	if got.code != 1 || !strings.HasPrefix(got.stderr, "scanner: ") {
		t.Errorf("login = %+v, want a failure", got)
	}
}

func TestCompletionOffersProfileNames(t *testing.T) {
	home := t.TempDir()
	if err := saveConfig(filepath.Join(home, "scanner", "config.json"), config{Current: "dev", Profiles: map[string]profile{"dev": {}, "prod": {}}}); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{{"use", ""}, {"favorites", "--profile", ""}} {
		got := runCLI(t, home, "", append([]string{"__complete"}, args...)...)
		if lines := strings.Split(got.stdout, "\n"); len(lines) < 3 || lines[0] != "dev" || lines[1] != "prod" {
			t.Errorf("completion of %v = %q", args, got.stdout)
		}
	}
}

// Without closed trades only the header and the Mini App's empty state are
// printed.
func TestBacktestWithoutTradesSaysWhy(t *testing.T) {
	from := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	backtest := apiclient.StrategyBacktest{Interval: "1d", Symbol: "UNIUSDT", Hold: 7, Fee: 0.001, From: &from, To: &to, UnfinishedTrades: 1}
	var output strings.Builder

	renderBacktest(&output, backtest)

	if output.String() != noTradesText {
		t.Fatalf("output:\n%s\nwant:\n%s", output.String(), noTradesText)
	}
	output.Reset()
	backtest.UnfinishedTrades = 0
	renderBacktest(&output, backtest)
	if !strings.HasSuffix(output.String(), "\nNo trades: the strategy did not alert on this coin in the stored history.\n") {
		t.Fatalf("output without unfinished trades:\n%s", output.String())
	}
}

const noTradesText = `┌────────────────┬────────────────────────────────────────────────┐
│ Coin           │ UNIUSDT                                        │
│ Period         │ 2026-07-20 → 2026-10-07 (1d candles)           │
│ Hold           │ 7 candles                                      │
│ Fee            │ 0.1% per buy and per sell                      │
│ Skipped alerts │ 0 (a trade was open)                           │
│ Unfinished     │ 1 (history ended or had a gap before the exit) │
└────────────────┴────────────────────────────────────────────────┘
No trades: every trade is still unfinished (1): the stored history ends or has a gap before its exit.
`

func TestBacktestWithoutEvaluatedCandlesSaysWhy(t *testing.T) {
	var output strings.Builder

	renderBacktest(&output, apiclient.StrategyBacktest{Interval: "1h", Symbol: "ETHUSDT"})

	if output.String() != "ETHUSDT: no stored candles of this interval yet; synchronization fills them first.\n" {
		t.Fatalf("output = %q", output.String())
	}
}
