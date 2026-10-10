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

	"github.com/gofrs/flock"
)

const testToken = "cst_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

// fakeAPI answers the CLI's operations with canned bodies and records the
// requests it receives.
type fakeAPI struct {
	t         *testing.T
	responses map[string]string // "METHOD path" → JSON body
	status    int
	// statuses override status for some "METHOD path" keys.
	statuses map[string]int
	bodies   map[string]string
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
	status, ok := api.statuses[key]
	if !ok {
		status = api.status
	}
	if status == http.StatusOK && request.Method == http.MethodPost && !strings.HasSuffix(request.URL.Path, "-validations") && !strings.HasSuffix(request.URL.Path, "-backtests") {
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
	want := `{"current":"prod","profiles":[{"name":"dev","server":"` + server.URL + `"},{"name":"prod","server":"` + server.URL + `"}]}` + "\n"
	if got := runCLI(t, home, "", "profiles", "--json"); got.code != 0 || got.stdout != want {
		t.Fatalf("profiles --json = %+v, want %s", got, want)
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
		"GET /api/v1/admin/strategies/3/backtest?symbol=BTCUSDT":                                                           backtestResponse,
		"GET /api/v1/admin/strategies/3/backtest?symbol=BTCUSDT&from=2024-01-01T00%3A00%3A00Z&to=2024-06-30T23%3A59%3A59Z": backtestResponse,
	})
	home := writeProfile(t, server.URL)

	if got := runCLI(t, home, "", "backtest", "--strategy", "3", "--symbol", "BTCUSDT"); got.code != 0 || got.stdout != backtestText {
		t.Errorf("backtest = %+v\nwant:\n%s", got, backtestText)
	}
	if raw := runCLI(t, home, "", "--json", "backtest", "--strategy", "3", "--symbol", "BTCUSDT"); raw.stdout != backtestResponse+"\n" {
		t.Errorf("--json output = %q", raw.stdout)
	}
	if got := runCLI(t, home, "", "backtest", "--strategy", "3", "--symbol", "BTCUSDT", "--from", "2024-01-01", "--to", "2024-06-30"); got.code != 0 {
		t.Errorf("backtest of a period = %+v", got)
	}
	if got := runCLI(t, home, "", "backtest", "--strategy", "3", "--symbol", "BTCUSDT", "--from", "January"); got.code != 1 || !strings.Contains(got.stderr, "--from") {
		t.Errorf("backtest of an invalid period = %+v", got)
	}
}

// Drafts send their rules as they are, with the period, and print the
// backtest like a saved strategy's.
func TestBacktestDraftsSendTheirRules(t *testing.T) {
	api, server := newFakeAPI(t, map[string]string{
		"POST /api/v1/admin/strategy-validations": `{"errors":[],"missing_indicators":[]}`,
		"POST /api/v1/admin/strategy-backtests":   backtestResponse,
	})
	home := writeProfile(t, server.URL)

	got := runCLI(t, home, "", "strategies", "backtest", "--expr", "h_close > 1", "--direction", "short", "--take-profit", "h_close * 0.9", "--stop-loss", "h_close * 1.1", "--symbol", "BTCUSDT", "--from", "2024-01-01")
	if got.code != 0 || got.stdout != backtestText {
		t.Errorf("strategies backtest = %+v\nwant:\n%s", got, backtestText)
	}
	want := `{"direction":"short","exit_expression":"","expression":"h_close \u003e 1","from":"2024-01-01T00:00:00Z","signal":false,` +
		`"stop_loss_expression":"h_close * 1.1","symbol":"BTCUSDT","take_profit_expression":"h_close * 0.9","target_ratio":null,"window":null}`
	if body := api.bodies["POST /api/v1/admin/strategy-backtests"]; body != want {
		t.Errorf("strategy draft = %s\nwant %s", body, want)
	}
	if got := runCLI(t, home, "", "signals", "backtest", "--expr", "h_close > 1", "--direction", "sideways", "--window", "12", "--symbol", "BTCUSDT"); got.code != 0 {
		t.Errorf("signals backtest = %+v", got)
	}
	want = `{"direction":"sideways","exit_expression":"","expression":"h_close \u003e 1","signal":true,` +
		`"stop_loss_expression":"","symbol":"BTCUSDT","take_profit_expression":"","target_ratio":2,"window":12}`
	if body := api.bodies["POST /api/v1/admin/strategy-backtests"]; body != want {
		t.Errorf("signal draft = %s\nwant %s", body, want)
	}
}

// An invalid draft reports its rules like create and is not backtested.
func TestBacktestDraftWithAnInvalidRuleSendsNothing(t *testing.T) {
	_, server := newFakeAPI(t, map[string]string{"POST /api/v1/admin/strategy-validations": `{"errors":["undeclared reference to 'x'"],"missing_indicators":[]}`})
	home := writeProfile(t, server.URL)

	got := runCLI(t, home, "", "strategies", "backtest", "--expr", "x > 1", "--exit", "x < 1", "--symbol", "BTCUSDT")
	if want := "scanner: invalid rules; nothing was sent\n  the entry rule: undeclared reference to 'x'\n  the exit rule: undeclared reference to 'x'\n"; got.code != 1 || got.stderr != want {
		t.Errorf("strategies backtest = %+v, want %q", got, want)
	}
}

// While a backtest runs on this machine, another fails before its backtest
// request.
func TestBacktestRefusesWhileAnotherRuns(t *testing.T) {
	_, server := newFakeAPI(t, map[string]string{
		"GET /api/v1/admin/strategies/3/backtest?symbol=BTCUSDT": backtestResponse,
		"POST /api/v1/admin/strategy-validations":                `{"errors":[],"missing_indicators":[]}`,
	})
	home := writeProfile(t, server.URL)
	running := flock.New(filepath.Join(home, "scanner", "backtest.lock"))
	if locked, err := running.TryLock(); err != nil || !locked {
		t.Fatalf("TryLock() = %v, %v", locked, err)
	}

	for _, args := range [][]string{
		{"backtest", "--strategy", "3", "--symbol", "BTCUSDT"},
		{"strategies", "backtest", "--expr", "h_close > 1", "--symbol", "BTCUSDT"},
		{"signals", "backtest", "--expr", "h_close > 1", "--direction", "long", "--symbol", "BTCUSDT"},
	} {
		if got := runCLI(t, home, "", args...); got.code != 1 || got.stderr != "scanner: another backtest runs on this machine; retry once it ends\n" {
			t.Errorf("%v while another runs = %+v", args, got)
		}
	}
	if err := running.Unlock(); err != nil {
		t.Fatal(err)
	}
	if got := runCLI(t, home, "", "backtest", "--strategy", "3", "--symbol", "BTCUSDT"); got.code != 0 {
		t.Errorf("backtest after the other ended = %+v", got)
	}
}

const backtestResponse = `{"baselines":{"buy_and_hold":0.25,"dca":0.18},"direction":"long","duration_ms":1234.5,` +
	`"equity":[{"equity":1.03,"time":"2024-02-01T07:00:00Z"},{"equity":1.0094,"time":"2024-06-30T23:00:00Z"}],"fee":0.001,"from":"2024-01-01T00:00:00Z","interval":"1h",` +
	`"skipped_alerts":1,"summary":{"max_drawdown":0.02,"net_profit":0.0094,"stats":{"average_bars":2,"average_loss":null,"average_trade":0.03,"average_win":0.03,` +
	`"exit_rule_exits":0,"profit_factor":null,"stop_loss_exits":0,"take_profit_exits":1,"trade_count":1,"win_rate":1}},` +
	`"symbol":"BTCUSDT","to":"2024-06-30T23:00:00Z","trades":[` +
	`{"buys":3,"entry_price":42000.123456789,"entry_time":"2024-02-01T06:00:00Z","exit_price":43303.15,"exit_reason":"take_profit",` +
	`"exit_signal_time":"2024-02-01T07:00:00Z","exit_time":"2024-02-01T07:00:00Z","exit_values":{},"fills":[],"net_return":0.03,"open":false,"stop_loss":null,"take_profit":43303.15},` +
	`{"buys":1,"entry_price":0.00001234,"entry_time":"2024-06-30T21:00:00Z","exit_price":0.0000121,"exit_signal_time":null,"exit_time":"2024-06-30T23:00:00Z",` +
	`"exit_values":{},"fills":[],"net_return":-0.02,"open":true,"stop_loss":null,"take_profit":null}]}`

const backtestText = `Execution time: 1.23 s
┌─────────────────┬──────────────────────────────────────────────────────────────────┐
│ Coin            │ BTCUSDT                                                          │
│ Direction       │ long                                                             │
│ Period          │ 2024-01-01 → 2024-06-30 (1h candles)                             │
│ Fee             │ 0.1% per buy and per sell                                        │
│ Skipped signals │ 1 (bought nothing: a trade was open, or TP/SL on the wrong side) │
└─────────────────┴──────────────────────────────────────────────────────────────────┘
┌──────────────────────┬───────────┬───────────────────────────────┐
│ METRIC               │ STRATEGY  │ COMPARED WITH                 │
├──────────────────────┼───────────┼───────────────────────────────┤
│ Net profit %         │     +0.94 │ Buy & Hold +25.00, DCA +18.00 │
│ Max drawdown %       │      2.00 │ -                             │
│ Closed trades        │         1 │ -                             │
│ Win rate %           │       100 │ -                             │
│ Profit factor        │         - │ -                             │
│ Avg trade %          │     +3.00 │ -                             │
│ Avg win %            │     +3.00 │ -                             │
│ Avg loss %           │         - │ -                             │
│ Avg candles held     │      2.00 │ -                             │
│ Exits TP / SL / rule │ 1 / 0 / 0 │ -                             │
└──────────────────────┴───────────┴───────────────────────────────┘
Trades: buys and exit rule sells fill at the open after their signal, TP and SL on the candle reaching them; an open trade is valued at the last close; returns are after fees.
┌───┬──────────────────┬────────────┬──────┬──────────────────┬───────────┬────┬───────┐
│   │ ENTRY            │ AVG PRICE  │ BUYS │ EXIT             │ PRICE     │ BY │ NET % │
├───┼──────────────────┼────────────┼──────┼──────────────────┼───────────┼────┼───────┤
│ 1 │ 2024-06-30 21:00 │ 0.00001234 │    1 │ open             │ 0.0000121 │ -  │ -2.00 │
│ 2 │ 2024-02-01 06:00 │  42000.123 │    3 │ 2024-02-01 07:00 │  43303.15 │ TP │ +3.00 │
└───┴──────────────────┴────────────┴──────┴──────────────────┴───────────┴────┴───────┘
`

// Text output retains the newest trades and reports omitted older ones.
func TestBacktestCapsTheTradeList(t *testing.T) {
	trades := make([]apiclient.BacktestTrade, maxTradeRows+3)
	for i := range trades {
		trades[i].EntryPrice = float64(i)
	}
	var output strings.Builder

	renderTrades(&output, apiclient.N1d, trades, tradeWords(apiclient.Long))

	lines := strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n")
	if len(lines) != maxTradeRows+6 || !strings.Contains(lines[4], fmt.Sprintf(" %d │", len(trades)-1)) || !strings.Contains(lines[len(lines)-3], " 3 │") || lines[len(lines)-1] != "… 3 more (use --json)" {
		t.Fatalf("output = %q", output.String())
	}
}

// A strategy is created disabled and reads the indicators that are not
// configured without adding them; --add-indicators adds them first.
func TestStrategiesCreateAddsMissingIndicatorsOnlyWhenAsked(t *testing.T) {
	for _, test := range []struct {
		args []string
		add  bool
	}{{}, {args: []string{"--add-indicators"}, add: true}} {
		api, server := newFakeAPI(t, map[string]string{
			"POST /api/v1/admin/strategy-validations":      `{"errors":[],"missing_indicators":[{"interval":"1h","type":"atr","parameters":{"period":100},"title":"h-atr-100"}]}`,
			"POST /api/v1/admin/scanner-indicator-batches": `{"items":[]}`,
			"POST /api/v1/admin/strategies":                `{"id":7,"name":"ATR","direction":"long","expression":"h_atr_100 > 1","message":"","enabled":false,"valid":true,"missing_indicators":[{"interval":"1h","type":"atr","parameters":{"period":100},"title":"h-atr-100"}]}`,
		})
		home := writeProfile(t, server.URL)

		got := runCLI(t, home, "", append([]string{"strategies", "create", "ATR", "--expr", "h_atr_100 > 1"}, test.args...)...)

		want := `┌────┬───────┬───────────────────────────┬───────────┬───────────────┬──────┬──────────────┬────────────┐
│ ID │ STATE │ NAME                      │ DIRECTION │ ENTRY         │ EXIT │ BUYS         │ MARKET CAP │
├────┼───────┼───────────────────────────┼───────────┼───────────────┼──────┼──────────────┼────────────┤
│  7 │ off   │ ATR                       │ long      │ h_atr_100 > 1 │ -    │ every signal │ -          │
│    │       │ not configured: h-atr-100 │           │               │      │              │            │
└────┴───────┴───────────────────────────┴───────────┴───────────────┴──────┴──────────────┴────────────┘
`
		wantNote := ""
		if test.add {
			wantNote = "added indicators: h-atr-100\n"
		}
		if got.code != 0 || got.stdout != want || got.stderr != wantNote {
			t.Fatalf("strategies create %v = %+v", test.args, got)
		}
		wants := map[string]string{
			"POST /api/v1/admin/strategies": `{"enabled":false,"exit_expression":"","expression":"h_atr_100 \u003e 1","max_market_cap_usd":null,"message":"","min_market_cap_usd":null,"name":"ATR","signal":false,"direction":"long","stop_loss_expression":"","take_profit_expression":"","target_ratio":null,"window":null}`,
		}
		if test.add {
			wants["POST /api/v1/admin/scanner-indicator-batches"] = `{"items":[{"interval":"1h","parameters":{"period":100},"type":"atr"}]}`
		} else if body, sent := api.bodies["POST /api/v1/admin/scanner-indicator-batches"]; sent {
			t.Errorf("indicators added without --add-indicators: %s", body)
		}
		for key, want := range wants {
			var sent, expected any
			if json.Unmarshal([]byte(api.bodies[key]), &sent) != nil || json.Unmarshal([]byte(want), &expected) != nil || !reflect.DeepEqual(sent, expected) {
				t.Errorf("%s body = %s, want %s", key, api.bodies[key], want)
			}
		}
	}
}

// Every rule is validated before any indicator is added, so an invalid
// rule adds nothing, and every invalid rule is reported, on stderr only.
func TestStrategiesCreateWithAnInvalidRuleAddsNothing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/api/v1/admin/strategy-validations" {
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		if strings.Contains(string(raw), "bad(") {
			fmt.Fprint(w, `{"errors":["syntax error"],"missing_indicators":[]}`)
			return
		}
		fmt.Fprint(w, `{"errors":[],"missing_indicators":[{"interval":"1h","type":"ema","parameters":{"period":3},"title":"h-ema-3"}]}`)
	}))
	t.Cleanup(server.Close)

	home := writeProfile(t, server.URL)
	args := []string{"strategies", "create", "X", "--expr", "h_ema_3 > 0", "--exit", "bad(", "--stop-loss", "bad(", "--add-indicators"}

	if got := runCLI(t, home, "", args...); got.code != 1 || got.stdout != "" ||
		got.stderr != "scanner: invalid rules; nothing was sent\n  the exit rule: syntax error\n  the stop loss: syntax error\n" {
		t.Fatalf("create = %+v", got)
	}
	want := `{"error":{"code":"invalid_expression","details":["the exit rule: syntax error","the stop loss: syntax error"],"message":"invalid rules; nothing was sent"},"request_id":""}` + "\n"
	if got := runCLI(t, home, "", append(args, "--json")...); got.code != 1 || got.stdout != "" || got.stderr != want {
		t.Fatalf("create --json = %+v, want stderr %s", got, want)
	}
}

func TestCommandsRenderCompactText(t *testing.T) {
	_, server := newFakeAPI(t, map[string]string{
		"GET /api/v1/admin/strategy-variables":    `{"items":[{"name":"d_rsi","label":"d-rsi","interval":"1d","indicator_id":1},{"name":"h_close","label":"h-close","interval":"1h"},{"name":"pnl","label":"pnl","position":true}]}`,
		"POST /api/v1/admin/strategy-validations": `{"errors":[],"missing_indicators":[{"interval":"1h","type":"atr","parameters":{"period":100},"title":"h-atr-100"}]}`,
		"GET /api/v1/favorites":                   `{"items":[{"symbol":"BTCUSDT","base_asset":"BTC","quote_asset":"USDT","active":true,"alert_count":0,"created_at":"2024-01-01T00:00:00Z"},{"symbol":"OLDUSDT","base_asset":"OLD","quote_asset":"USDT","active":false,"alert_count":0,"created_at":"2024-01-01T00:00:00Z"}]}`,
		"GET /api/v1/admin/strategies":            `{"items":[{"id":3,"name":"Dip buy","direction":"long","expression":"d_rsi < 30 &&\n  h_close > 1","exit_expression":"pnl > 5","take_profit_expression":"h_close * 1.1","stop_loss_expression":"","min_market_cap_usd":10000000,"max_market_cap_usd":1500000000,"message":"Dip!","enabled":true,"valid":true},{"id":4,"name":"Old","direction":"short","expression":"x","message":"","enabled":false,"valid":false,"problem":"undeclared"}]}`,
	})
	home := writeProfile(t, server.URL)

	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"vars", "--filter", "RSI"}, want: "d_rsi\n"},
		{args: []string{"vars", "--filter", "pnl"}, want: "pnl (exit rule only)\n"},
		{args: []string{"vars", "--filter", "RSI", "--json"}, want: `{"items":[{"indicator_id":1,"interval":"1d","label":"d-rsi","name":"d_rsi","position":false}]}` + "\n"},
		{args: []string{"validate", "h_atr_100 > 1"}, want: "ok; reads indicators that are not configured: h-atr-100\n"},
		{args: []string{"favorites"}, want: "BTCUSDT OLDUSDT(inactive)\n"},
		{args: []string{"strategies"}, want: `┌────┬──────────────┬─────────────────────┬───────────┬───────────────────────────┬──────────────────┬─────────────────────┬──────────────┐
│ ID │ STATE        │ NAME                │ DIRECTION │ ENTRY                     │ EXIT             │ BUYS                │ MARKET CAP   │
├────┼──────────────┼─────────────────────┼───────────┼───────────────────────────┼──────────────────┼─────────────────────┼──────────────┤
│  3 │ on           │ Dip buy             │ long      │ d_rsi < 30 && h_close > 1 │ pnl > 5          │ one per trade       │ $10M – $1.5B │
│    │              │ message: Dip!       │           │                           │ TP h_close * 1.1 │                     │              │
│  4 │ off, invalid │ Old                 │ short     │ x                         │ -                │ one short per trade │ -            │
│    │              │ problem: undeclared │           │                           │                  │                     │              │
└────┴──────────────┴─────────────────────┴───────────┴───────────────────────────┴──────────────────┴─────────────────────┴──────────────┘
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

	if got := runCLI(t, home, "", "validate", "x > 1"); got.code != 1 || got.stdout != "" || got.stderr != "scanner: the expression is invalid\n  undeclared reference to 'x'\n" {
		t.Errorf("invalid expression = %+v", got)
	}
	api.status = http.StatusNotFound
	if got := runCLI(t, home, "", "backtest", "--strategy", "3", "--symbol", "NOPE"); got.code != 1 || got.stderr != "scanner: symbol_not_found: Symbol is unknown\n" {
		t.Errorf("API error = %+v", got)
	}
	if err := saveConfig(filepath.Join(home, "scanner", "config.json"), config{Current: "dev", Profiles: map[string]profile{"dev": {Server: server.URL, Token: "cst_revoked"}}}); err != nil {
		t.Fatal(err)
	}
	if got := runCLI(t, home, "", "favorites"); got.code != 1 || !strings.Contains(got.stderr, "unauthenticated") || strings.Contains(got.stderr, "cst_revoked") ||
		!strings.Contains(got.stderr, "run scanner login dev --server "+server.URL) {
		t.Errorf("revoked token = %+v", got)
	}
	if got := runCLI(t, t.TempDir(), "", "favorites"); got.code != 1 || got.stderr != "scanner: no profile selected; run scanner login NAME --server URL, or pass --profile\n" {
		t.Errorf("no profile = %+v", got)
	}
	// Login checks the token it is given, so it does not suggest logging in.
	if got := runCLI(t, t.TempDir(), "cst_revoked", "login", "dev", "--server", server.URL); got.code != 1 || !strings.Contains(got.stderr, "create one in the Mini App settings") {
		t.Errorf("login with a revoked token = %+v", got)
	}
}

// Unreachable servers and unknown commands are errors in the --json error
// shape too, and an invalid strategy ID sends nothing.
func TestEarlyErrorsRespectJSONFlag(t *testing.T) {
	for _, test := range []struct {
		args []string
		json bool
	}{
		{[]string{"typo", "--json=true"}, true},
		{[]string{"profiles", "---oops", "--json"}, true},
		{[]string{"profiles", "---oops", "--=bad", "--json"}, true},
		{[]string{"profiles", "---oops", "--json", "--json=false"}, false},
		{[]string{"profiles", "---oops", "--", "--json"}, false},
		{[]string{"strategies", "create", "Test", "--expr", "---oops", "---oops", "--json"}, true},
		{[]string{"strategies", "create", "Test", "--expr", "---oops", "---oops", "--message", "--json"}, false},
		{[]string{"validate", "x", "--exit=oops", "--json"}, true},
		{[]string{"favorites", "--json=false", "--unknown", "--json"}, true},
		{[]string{"validate", "x", "--exit=oops", "--json", "--json=false"}, false},
		{[]string{"validate", "x", "--exit=oops", "--", "--json"}, false},
		{[]string{"strategies", "create", "Test", "--expr", "--json"}, false},
		{[]string{"typo", "--json=false"}, false},
		{[]string{"typo", "--json", "--json=false"}, false},
		{[]string{"--unknown", "--json=true"}, true},
		{[]string{"typo", "--", "--json"}, false},
	} {
		got := runCLI(t, t.TempDir(), "", test.args...)
		var body apiclient.ErrorResponse
		isJSON := json.Unmarshal([]byte(got.stderr), &body) == nil
		if got.code != 1 || got.stdout != "" || isJSON != test.json || isJSON && body.Error.Code != "usage" {
			t.Errorf("scanner %v = %+v", test.args, got)
		}
	}
}

func TestLocalErrors(t *testing.T) {
	home := writeProfile(t, "http://127.0.0.1:1")
	for _, test := range []struct {
		args         []string
		code, prefix string
	}{
		{args: []string{"favorites"}, code: "unreachable", prefix: "cannot reach http://127.0.0.1:1 (profile dev): "},
		{args: []string{"indicators", "typo"}, code: "usage", prefix: `unknown command "typo" for "scanner indicators"`},
		{args: []string{"typo"}, code: "usage", prefix: `unknown command "typo" for "scanner"`},
		{args: []string{"backtest", "--strategy", "0", "--symbol", "BTCUSDT"}, code: "usage", prefix: `strategy ID "0" must be a positive int64`},
		{args: []string{"backtest", "--strategy", "abc", "--symbol", "BTCUSDT"}, code: "usage", prefix: `strategy ID "abc" must be a positive int64`},
	} {
		got := runCLI(t, home, "", test.args...)
		if got.code != 1 || got.stdout != "" || !strings.HasPrefix(got.stderr, "scanner: "+test.prefix) {
			t.Errorf("scanner %v = %+v", test.args, got)
		}
		got = runCLI(t, home, "", append(test.args, "--json")...)
		var body apiclient.ErrorResponse
		if err := json.Unmarshal([]byte(got.stderr), &body); err != nil || got.code != 1 || got.stdout != "" ||
			string(body.Error.Code) != test.code || !strings.HasPrefix(body.Error.Message, test.prefix) {
			t.Errorf("scanner %v --json = %+v", test.args, got)
		}
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
		{name: "add indicators", target: "/api/v1/admin/scanner-indicator-batches", args: []string{"strategies", "create", "Test", "--expr", "h_atr_100 > 1", "--add-indicators"}, status: http.StatusCreated},
		{name: "create", target: "/api/v1/admin/strategies", args: []string{"strategies", "create", "Test", "--expr", "h_close > 1"}, status: http.StatusCreated},
		{name: "backtest", target: "/api/v1/admin/strategies/3/backtest", args: []string{"backtest", "--strategy", "3", "--symbol", "BTCUSDT"}, status: http.StatusOK},
		{name: "delete", target: "/api/v1/admin/strategies/3", args: []string{"strategies", "delete", "3"}, status: http.StatusOK},
	} {
		for _, html := range []bool{true, false} {
			t.Run(test.name+"/html="+strconv.FormatBool(html), func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					if r.URL.Path == "/api/v1/admin/strategies" && test.name == "delete" {
						// Delete checks that the ID is a strategy first.
						fmt.Fprint(w, `{"items":[{"id":3,"name":"Test","expression":"h_close > 1","valid":true}]}`)
						return
					}
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
				var body apiclient.ErrorResponse
				if err := json.Unmarshal([]byte(got.stderr), &body); err != nil || got.code != 1 || got.stdout != "" ||
					body.Error.Code != "unexpected_response" || !strings.Contains(body.Error.Message, "unexpected API response") {
					t.Fatalf("unexpected success response = %+v", got)
				}
			})
		}
	}
}

func TestDeleteStrategy(t *testing.T) {
	api, server := newFakeAPI(t, map[string]string{
		"GET /api/v1/admin/strategies":                        `{"items":[{"id":9223372036854775807,"name":"Max","expression":"h_close > 1","valid":true}]}`,
		"DELETE /api/v1/admin/strategies/9223372036854775807": "",
	})
	api.statuses = map[string]int{"DELETE /api/v1/admin/strategies/9223372036854775807": http.StatusNoContent}
	home := writeProfile(t, server.URL)

	for _, jsonOutput := range []bool{false, true} {
		clear(api.bodies)
		args := []string{"strategies", "delete", "9223372036854775807"}
		want := "deleted strategy 9223372036854775807\n"
		if jsonOutput {
			args = append(args, "--json")
			want = `{"deleted":1}` + "\n"
		}
		got := runCLI(t, home, "", args...)
		if got.code != 0 || got.stdout != want || got.stderr != "" {
			t.Fatalf("delete = %+v, want stdout %q", got, want)
		}
		if body, ok := api.bodies["DELETE /api/v1/admin/strategies/9223372036854775807"]; !ok || body != "" || len(api.bodies) != 2 {
			t.Fatalf("delete requests = %v", api.bodies)
		}
	}
}

// With --json, errors keep the API error shape, request ID and details
// included; SERVER stands for the server's URL.
func TestDeleteStrategyReportsAPIErrors(t *testing.T) {
	for _, test := range []struct {
		status           int
		body, text, json string
	}{
		{
			http.StatusBadRequest, `{"error":{"code":"invalid_request","message":"Invalid ID"},"request_id":"r"}`,
			"HTTP 400 from SERVER; check the profile's server URL",
			`{"error":{"code":"unexpected_response","message":"HTTP 400 from SERVER; check the profile's server URL"},"request_id":""}`,
		},
		{
			http.StatusNotFound, `{"error":{"code":"strategy_not_found","message":"Strategy not found","details":{"id":3}},"request_id":"r"}`,
			"strategy_not_found: Strategy not found",
			`{"error":{"code":"strategy_not_found","details":{"id":3},"message":"Strategy not found"},"request_id":"r"}`,
		},
		{
			http.StatusUnauthorized, `{"error":{"code":"unauthenticated","message":"Session is invalid or expired"},"request_id":"r"}`,
			"unauthenticated: Session is invalid or expired (revoked or wrong token? run scanner login dev --server SERVER)",
			`{"error":{"code":"unauthenticated","message":"Session is invalid or expired (revoked or wrong token? run scanner login dev --server SERVER)"},"request_id":"r"}`,
		},
		{
			http.StatusInternalServerError, `{"error":{"code":"internal_error","message":"Internal server error"},"request_id":"r"}`,
			"internal_error: Internal server error",
			`{"error":{"code":"internal_error","message":"Internal server error"},"request_id":"r"}`,
		},
	} {
		t.Run(strconv.Itoa(test.status), func(t *testing.T) {
			api, server := newFakeAPI(t, map[string]string{
				"GET /api/v1/admin/strategies":      `{"items":[{"id":3,"name":"Test","expression":"h_close > 1","valid":true}]}`,
				"DELETE /api/v1/admin/strategies/3": test.body,
			})
			api.statuses = map[string]int{"DELETE /api/v1/admin/strategies/3": test.status}
			home := writeProfile(t, server.URL)
			if got := runCLI(t, home, "", "strategies", "delete", "3"); got.code != 1 || got.stdout != "" || got.stderr != "scanner: "+strings.ReplaceAll(test.text, "SERVER", server.URL)+"\n" {
				t.Errorf("delete API error = %+v", got)
			}
			if got := runCLI(t, home, "", "strategies", "delete", "3", "--json"); got.code != 1 || got.stdout != "" || got.stderr != strings.ReplaceAll(test.json, "SERVER", server.URL)+"\n" {
				t.Errorf("delete --json API error = %+v", got)
			}
		})
	}
}

// Login refuses cleartext to other hosts, and URLs with a path, which would
// only lead to 404s.
func TestLoginRefusesUnsuitableServers(t *testing.T) {
	for _, server := range []string{"http://scanner.example", "https://scanner.example/api", "http://localhost:8080/?x=1"} {
		got := runCLI(t, t.TempDir(), testToken, "login", "prod", "--server", server)
		if got.code != 1 || !strings.HasPrefix(got.stderr, "scanner: --server ") {
			t.Errorf("login --server %s = %+v, want a failure", server, got)
		}
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

func TestCompletionOffersStrategiesAndFavorites(t *testing.T) {
	_, server := newFakeAPI(t, map[string]string{
		"GET /api/v1/admin/strategies": `{"items":[{"id":79,"name":"Dip buy","expression":"x","message":"","enabled":false,"valid":true}]}`,
		"GET /api/v1/favorites":        `{"items":[{"symbol":"BTCUSDT","base_asset":"BTC","quote_asset":"USDT","active":true,"alert_count":0,"created_at":"2024-01-01T00:00:00Z"}]}`,
	})
	home := writeProfile(t, server.URL)

	for args, want := range map[[3]string]string{
		{"strategies", "delete", ""}:   "79\tDip buy",
		{"strategies", "update", ""}:   "79\tDip buy",
		{"backtest", "--strategy", ""}: "79\tDip buy",
		{"backtest", "--symbol", ""}:   "BTCUSDT",
	} {
		got := runCLI(t, home, "", append([]string{"__complete"}, args[:]...)...)
		if first, _, _ := strings.Cut(got.stdout, "\n"); first != want {
			t.Errorf("completion of %v = %q", args, got.stdout)
		}
	}
}

// Without trades only the header and the Mini App's empty state are
// printed.
func TestBacktestWithoutTradesSaysWhy(t *testing.T) {
	from := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	backtest := apiclient.StrategyBacktest{Interval: "1d", Symbol: "UNIUSDT", Direction: apiclient.Short, Fee: 0.001, From: &from, To: &to}
	var output strings.Builder

	renderBacktest(&output, backtest, false, false)

	if output.String() != noTradesText {
		t.Fatalf("output:\n%s\nwant:\n%s", output.String(), noTradesText)
	}
}

const noTradesText = `Execution time: 0.00 s
┌─────────────────┬───────────────────────────────────────────────────────────────────┐
│ Coin            │ UNIUSDT                                                           │
│ Direction       │ short                                                             │
│ Period          │ 2026-07-20 → 2026-10-07 (1d candles)                              │
│ Fee             │ 0.1% per short and per cover                                      │
│ Skipped signals │ 0 (shorted nothing: a trade was open, or TP/SL on the wrong side) │
└─────────────────┴───────────────────────────────────────────────────────────────────┘
No trades: the strategy did not short on this coin in the stored history.
`

func TestBacktestWithoutEvaluatedCandlesSaysWhy(t *testing.T) {
	var output strings.Builder

	renderBacktest(&output, apiclient.StrategyBacktest{Interval: "1h", Symbol: "ETHUSDT"}, false, false)
	renderBacktest(&output, apiclient.StrategyBacktest{Interval: "1h", Symbol: "ETHUSDT"}, true, false)

	if output.String() != "Execution time: 0.00 s\nETHUSDT: no stored candles of this interval yet; synchronization fills them first.\n"+
		"Execution time: 0.00 s\nETHUSDT: no stored candles of this interval in the requested period.\n" {
		t.Fatalf("output = %q", output.String())
	}
}
