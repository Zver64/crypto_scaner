package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"crypto-scanner/internal/apiclient"
	"github.com/gofrs/flock"
)

func TestMultiCoinBacktestsSavedAndDrafts(t *testing.T) {
	for _, args := range [][]string{
		{"backtest", "--strategy", "3"},
		{"strategies", "backtest", "--expr", "h_close > 1"},
		{"signals", "backtest", "--expr", "h_close > 1", "--direction", "long"},
	} {
		t.Run(strings.Join(args[:2], "-"), func(t *testing.T) {
			var home string
			var calls []string
			validations := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/api/v1/admin/strategy-validations" {
					validations++
					io.WriteString(w, `{"errors":[],"missing_indicators":[]}`)
					return
				}
				lock := flock.New(filepath.Join(home, "scanner", "backtest.lock"))
				locked, err := lock.TryLock()
				if err != nil || locked {
					if locked {
						lock.Unlock()
					}
					t.Error("batch did not retain its machine lock")
				}
				symbol := r.URL.Query().Get("symbol")
				if r.Method == http.MethodPost {
					var body apiclient.StrategyBacktestInput
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					symbol = body.Symbol
					if body.Chart != nil && *body.Chart {
						t.Error("draft requested chart")
					}
					if body.Expression != "h_close > 1" {
						t.Errorf("unexpected expression: %s", body.Expression)
					}
				} else if r.URL.Query().Get("chart") != "" {
					t.Error("saved backtest requested chart")
				}
				calls = append(calls, symbol)
				io.WriteString(w, strings.ReplaceAll(backtestResponse, "BTCUSDT", symbol))
			}))
			defer server.Close()
			home = writeProfile(t, server.URL)
			got := runCLI(t, home, "", append(args, "--symbol", " btcusdt ,ETHUSDT", "--symbol", "BTCUSDT", "--json")...)
			if got.code != 0 {
				t.Fatalf("run = %+v", got)
			}
			if !reflect.DeepEqual(calls, []string{"BTCUSDT", "ETHUSDT"}) {
				t.Fatalf("requests = %v", calls)
			}
			var output struct {
				Status string
				Items  []backtestCoinResult
			}
			if err := json.Unmarshal([]byte(got.stdout), &output); err != nil {
				t.Fatal(err)
			}
			if output.Status != "completed" || len(output.Items) != 2 || output.Items[1].Result.Symbol != "ETHUSDT" {
				t.Fatalf("result = %+v", output)
			}
			want := 1
			if args[0] == "backtest" {
				want = 0
			}
			if validations != want {
				t.Fatalf("validated %d times; want %d", validations, want)
			}
		})
	}
}

func TestMultiCoinPartialAndGlobalFailures(t *testing.T) {
	for _, test := range []struct {
		status int
		code   string
		next   bool
	}{
		{404, "symbol_not_found", true},
		{404, "strategy_not_found", false},
		{503, "backtest_too_heavy", true},
		{401, "unauthenticated", false},
		{403, "administrator_required", false},
		{429, "backtest_busy", false},
		{500, "internal_error", false},
	} {
		t.Run(test.code, func(t *testing.T) {
			first := "GET /api/v1/admin/strategies/3/backtest?symbol=BTCUSDT"
			second := "GET /api/v1/admin/strategies/3/backtest?symbol=ETHUSDT"
			api, server := newFakeAPI(t, map[string]string{
				first:  fmt.Sprintf(`{"error":{"code":"%s","message":"Failed","details":{"reason":"test"}},"request_id":"batch-request"}`, test.code),
				second: strings.ReplaceAll(backtestResponse, "BTCUSDT", "ETHUSDT"),
			})
			api.statuses = map[string]int{first: test.status}
			home := writeProfile(t, server.URL)
			got := runCLI(t, home, "", "backtest", "--strategy", "3", "--symbol", "BTCUSDT,ETHUSDT", "--json")
			var output struct {
				Status string
				Items  []backtestCoinResult
			}
			if got.code != 1 || json.Unmarshal([]byte(got.stdout), &output) != nil {
				t.Fatalf("result = %+v", got)
			}
			if output.Items[0].Error.RequestId != "batch-request" || output.Items[0].Error.Error.Details == nil {
				t.Fatalf("lost API error: %+v", output)
			}
			_, called := api.bodies[second]
			if called != test.next {
				t.Fatalf("next coin called = %v, want %v", called, test.next)
			}
			status := "not_run"
			if test.next {
				status = "completed"
			}
			if output.Items[1].Status != status || !strings.Contains(got.stderr, `"code":"backtest_failed"`) {
				t.Fatalf("partial failure = %+v", got)
			}
		})
	}
}

func TestMultiCoinTruncatedResponseStopsQueueAndRetainsResults(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		symbol := r.URL.Query().Get("symbol")
		calls = append(calls, symbol)
		w.Header().Set("Content-Type", "application/json")
		if symbol == "ETHUSDT" {
			// Returning fewer bytes than advertised makes the generated
			// client's io.ReadAll fail with io.ErrUnexpectedEOF.
			w.Header().Set("Content-Length", "1000")
			io.WriteString(w, `{"symbol":"ETHUSDT"`)
			return
		}
		io.WriteString(w, strings.ReplaceAll(backtestResponse, "BTCUSDT", symbol))
	}))
	defer server.Close()
	home := writeProfile(t, server.URL)
	got := runCLI(t, home, "", "backtest", "--strategy", "3", "--symbol", "BTCUSDT,ETHUSDT,ARBUSDT", "--json")
	var output struct {
		Status string
		Items  []backtestCoinResult
	}
	if got.code != 1 || json.Unmarshal([]byte(got.stdout), &output) != nil {
		t.Fatalf("run = %+v", got)
	}
	if !reflect.DeepEqual(calls, []string{"BTCUSDT", "ETHUSDT"}) {
		t.Fatalf("requests after truncated response = %v", calls)
	}
	if output.Status != "failed" || len(output.Items) != 3 ||
		output.Items[0].Status != "completed" || output.Items[0].Result == nil ||
		output.Items[0].Result.Symbol != "BTCUSDT" ||
		output.Items[1].Status != "failed" || output.Items[1].Error == nil ||
		output.Items[1].Error.Error.Code != "unexpected_response" ||
		!strings.Contains(output.Items[1].Error.Error.Message, io.ErrUnexpectedEOF.Error()) ||
		output.Items[2].Status != "not_run" {
		t.Fatalf("partial result = %+v", output)
	}
}

func TestMultiCoinSummaryOmitsDetailsAndCountsNeutrals(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	result := apiclient.StrategyBacktest{
		Symbol: "BTCUSDT", Interval: apiclient.N1h, Direction: apiclient.Long, From: &from, To: &from,
		Signal: &apiclient.BacktestSignal{
			Evaluation: &apiclient.BacktestSignalEvaluation{
				Evaluated: 4, Signals: apiclient.BacktestSignalStats{Count: 4, Successes: 0, Neutrals: 1, SuccessRate: new(float64(0))},
				All: apiclient.BacktestSignalStats{Count: 1254, Successes: 243, Neutrals: 255, SuccessRate: new(float64(243.0 / 999))},
			},
			Occurrences: []apiclient.BacktestSignalOccurrence{{Close: 987654321}},
		},
	}
	var output bytes.Buffer
	renderBacktestSummary(&output, result, false)
	text := output.String()
	if !strings.Contains(text, "0/3 (0%)") || !strings.Contains(text, "243/999") || strings.Contains(text, "987654321") || strings.Contains(text, "newest first") {
		t.Fatalf("signal summary = %s", text)
	}
	output.Reset()
	result.Signal = nil
	result.Direction = apiclient.Short
	result.Trades = []apiclient.BacktestTrade{{Open: true, EntryPrice: 987654321}}
	renderBacktestSummary(&output, result, false)
	text = output.String()
	if strings.Contains(text, "Buy & Hold") || strings.Contains(text, "987654321") || !strings.Contains(text, "Open trades") {
		t.Fatalf("short summary = %s", text)
	}
}

func TestMultiCoinCancellationRetainsCompletedResults(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := &cli{}
	command := c.command()
	c.json = true
	command.SetContext(ctx)
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(io.Discard)
	calls := 0
	err := c.runBacktests(command, backtestFlags{symbols: []string{"BTCUSDT", "ETHUSDT", "ARBUSDT"}}, func(symbol string) ([]byte, *apiclient.StrategyBacktest, error) {
		calls++
		if calls == 2 {
			cancel()
			return nil, nil, ctx.Err()
		}
		return nil, &apiclient.StrategyBacktest{Symbol: symbol}, nil
	})
	var result struct {
		Status string
		Items  []backtestCoinResult
	}
	if err != context.Canceled || json.Unmarshal(output.Bytes(), &result) != nil {
		t.Fatalf("cancel = %v, %s", err, output.String())
	}
	if calls != 2 || result.Status != "cancelled" || result.Items[0].Status != "completed" || result.Items[1].Status != "cancelled" || result.Items[2].Status != "not_run" {
		t.Fatalf("cancel result = %+v", result)
	}
}
