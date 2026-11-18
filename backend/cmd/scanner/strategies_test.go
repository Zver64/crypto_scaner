package main

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"crypto-scanner/internal/apiclient"
)

func TestUpdateStrategyPreservesUnspecifiedFields(t *testing.T) {
	coins := apiclient.StrategyCoins{Source: apiclient.CoinSourceSelected, Symbols: []string{"BTCUSDT"}}
	rules := func(expression, exit, takeProfit, stopLoss string) apiclient.StrategyRules {
		return apiclient.StrategyRules{Direction: apiclient.Long, Expression: expression, ExitExpression: exit, TakeProfitExpression: takeProfit, StopLossExpression: stopLoss}
	}
	for _, test := range []struct {
		name  string
		flags []string
		want  apiclient.StrategyUpdate
	}{
		{"expression", []string{"--expr", "w_rsi < 50"}, apiclient.StrategyUpdate{Name: "Test", Rules: rules("w_rsi < 50", "", "", ""), Message: "custom", Coins: coins}},
		{"name", []string{"--name", "New"}, apiclient.StrategyUpdate{Name: "New", Rules: rules("m_rsi < 40", "", "", ""), Message: "custom", Coins: coins}},
		{"clear message", []string{"--message", ""}, apiclient.StrategyUpdate{Name: "Test", Rules: rules("m_rsi < 40", "", "", ""), Message: "", Coins: coins}},
		{
			"exits", []string{"--exit", "pnl > 5 || w_rsi > 70", "--take-profit", "h_close * 1.1", "--stop-loss", "w_low"},
			apiclient.StrategyUpdate{Name: "Test", Rules: rules("m_rsi < 40", "pnl > 5 || w_rsi > 70", "h_close * 1.1", "w_low"), Message: "custom", Coins: coins},
		},
		{
			// Chosen coins replace the current ones; only active favorites count.
			"coins", []string{"--coin", "solusdt,", "--favorites"},
			apiclient.StrategyUpdate{Name: "Test", Rules: rules("m_rsi < 40", "", "", ""), Message: "custom", Coins: apiclient.StrategyCoins{Source: apiclient.CoinSourceSelected, Symbols: []string{"ETHUSDT", "SOLUSDT"}}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := `{"id":"00000000-0000-7000-8000-000000000006","name":"Test","rules":{"direction":"long","expression":"w_rsi < 50"},"coins":{"source":"selected","symbols":["BTCUSDT"]},"message":"custom","enabled":false,"valid":true}`
			api, server := newFakeAPI(t, map[string]string{
				"GET /api/v1/admin/strategies/00000000-0000-7000-8000-000000000006": `{"id":"00000000-0000-7000-8000-000000000006","name":"Test","rules":{"direction":"long","expression":"m_rsi < 40"},"coins":{"symbols":["BTCUSDT"],"source":"selected"},"message":"custom","enabled":false,"valid":true}`,
				"GET /api/v1/favorites": `{"items":[{"symbol":"ETHUSDT","active":true},{"symbol":"OLDUSDT","active":false}]}`,
				"PUT /api/v1/admin/strategies/00000000-0000-7000-8000-000000000006": response,
			})
			got := runCLI(t, writeProfile(t, server.URL), "", append([]string{"strategies", "update", "00000000-0000-7000-8000-000000000006", "--json"}, test.flags...)...)
			if got.code != 0 || got.stdout != response+"\n" {
				t.Fatalf("update = %+v", got)
			}
			var body apiclient.StrategyUpdate
			raw := api.bodies["PUT /api/v1/admin/strategies/00000000-0000-7000-8000-000000000006"]
			if err := json.Unmarshal([]byte(raw), &body); err != nil || !reflect.DeepEqual(body, test.want) {
				t.Fatalf("body = %s, want %+v", raw, test.want)
			}
			if strings.Contains(raw, "enabled") {
				t.Fatal("update must not send enabled state")
			}
		})
	}
}

func TestUpdateStrategyRejectsInvalidInputBeforeWrites(t *testing.T) {
	for _, test := range []struct {
		name   string
		args   []string
		get    string
		status int
		stderr string
	}{
		{name: "bad ID", args: []string{"00000000-0000-7000-8000-000000000000", "--name", "New"}},
		{name: "overflow ID", args: []string{"00000000-0000-7000-8000-9223372036854775808", "--name", "New"}},
		{name: "no fields", args: []string{"6"}},
		{name: "missing strategy", args: []string{"00000000-0000-7000-8000-000000000006", "--name", "New"}, status: http.StatusNotFound, get: `{"error":{"code":"strategy_not_found","message":"Strategy does not exist"},"request_id":"r"}`},
		{name: "enabled strategy", args: []string{"00000000-0000-7000-8000-000000000006", "--expr", "w_rsi < 50"}, get: `{"id":"00000000-0000-7000-8000-000000000006","name":"Test","rules":{"expression":"m_rsi < 40"},"message":"","enabled":true,"valid":true}`, stderr: "scanner: strategy 00000000-0000-7000-8000-000000000006 is enabled; edit it in the Mini App\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			get := "GET /api/v1/admin/strategies/" + test.args[0]
			api, server := newFakeAPI(t, map[string]string{get: test.get})
			if test.status != 0 {
				api.status = test.status
			}
			got := runCLI(t, writeProfile(t, server.URL), "", append([]string{"strategies", "update"}, test.args...)...)
			if got.code != 1 || got.stderr == "" || test.stderr != "" && got.stderr != test.stderr {
				t.Fatalf("update = %+v", got)
			}
			for key := range api.bodies {
				if key != get {
					t.Fatalf("unexpected request %s", key)
				}
			}
		})
	}
}

// Each group edits and deletes only its own: a signal is refused by the
// strategies commands and a strategy by the signals ones, before any write.
func TestGroupsRefuseTheOtherKind(t *testing.T) {
	_, server := newFakeAPI(t, map[string]string{
		"GET /api/v1/admin/strategies/00000000-0000-7000-8000-000000000005": `{"id":"00000000-0000-7000-8000-000000000005","name":"Trade","rules":{"direction":"long","expression":"h_close > 1"},"valid":true}`,
		"GET /api/v1/admin/strategies/00000000-0000-7000-8000-000000000006": `{"id":"00000000-0000-7000-8000-000000000006","name":"Crash","rules":{"signal":true,"direction":"short","expression":"h_close > 1"},"valid":true}`,
	})
	home := writeProfile(t, server.URL)
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"strategies", "update", "00000000-0000-7000-8000-000000000006", "--name", "X"}, "scanner: 00000000-0000-7000-8000-000000000006 is a signal; use scanner signals\n"},
		{[]string{"strategies", "delete", "00000000-0000-7000-8000-000000000006"}, "scanner: 00000000-0000-7000-8000-000000000006 is a signal; use scanner signals\n"},
		{[]string{"signals", "update", "00000000-0000-7000-8000-000000000005", "--name", "X"}, "scanner: 00000000-0000-7000-8000-000000000005 is a strategy; use scanner strategies\n"},
		{[]string{"signals", "delete", "00000000-0000-7000-8000-000000000005"}, "scanner: 00000000-0000-7000-8000-000000000005 is a strategy; use scanner strategies\n"},
	} {
		if got := runCLI(t, home, "", test.args...); got.code != 1 || got.stderr != test.want {
			t.Errorf("scanner %v = %+v", test.args, got)
		}
	}
}

// A signal update keeps its fields, coin source, market cap range, and target
// ratio included, changing only the flags given.
func TestUpdateSignalPreservesUnspecifiedFields(t *testing.T) {
	response := `{"id":"00000000-0000-7000-8000-000000000006","name":"Crash Soon","rules":{"signal":true,"direction":"short","expression":"h_close > 1"},"coins":{"source":"favorites"},"message":"m","enabled":false,"valid":true}`
	api, server := newFakeAPI(t, map[string]string{
		"GET /api/v1/admin/strategies/00000000-0000-7000-8000-000000000006": `{"id":"00000000-0000-7000-8000-000000000006","name":"Crash","rules":{"signal":true,"direction":"short","expression":"h_close > 1","target_ratio":3,"window":12},` +
			`"coins":{"source":"favorites","min_market_cap_usd":10000000,"max_market_cap_usd":2000000000},"message":"m","enabled":false,"valid":true}`,
		"PUT /api/v1/admin/strategies/00000000-0000-7000-8000-000000000006": response,
	})
	got := runCLI(t, writeProfile(t, server.URL), "", "signals", "update", "00000000-0000-7000-8000-000000000006", "--name", "Crash Soon", "--max-market-cap", "1B", "--window", "24", "--json")
	if got.code != 0 || got.stdout != response+"\n" {
		t.Fatalf("update = %+v", got)
	}
	var body apiclient.StrategyUpdate
	raw := api.bodies["PUT /api/v1/admin/strategies/00000000-0000-7000-8000-000000000006"]
	minimum, maximum := 10e6, 1e9
	ratio, window := apiclient.SignalTargetRatioN3, apiclient.SignalWindowN24
	want := apiclient.StrategyUpdate{
		Name:    "Crash Soon",
		Rules:   apiclient.StrategyRules{Signal: true, Direction: apiclient.Short, Expression: "h_close > 1", TargetRatio: &ratio, Window: &window},
		Coins:   apiclient.StrategyCoins{Source: apiclient.CoinSourceFavorites, MinMarketCapUsd: &minimum, MaxMarketCapUsd: &maximum},
		Message: "m",
	}
	if err := json.Unmarshal([]byte(raw), &body); err != nil || !reflect.DeepEqual(body, want) {
		t.Fatalf("body = %s, want %+v", raw, want)
	}
}
