package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"crypto-scanner/internal/apiclient"
)

func TestUpdateStrategyPreservesUnspecifiedFields(t *testing.T) {
	for _, test := range []struct {
		name  string
		flags []string
		want  apiclient.StrategyUpdate
		added bool
	}{
		{"expression", []string{"--expr", "w_rsi < 50"}, apiclient.StrategyUpdate{Name: "Test", Expression: "w_rsi < 50", Message: "custom"}, false},
		{"name", []string{"--name", "New"}, apiclient.StrategyUpdate{Name: "New", Expression: "m_rsi < 40", Message: "custom"}, false},
		{"clear message", []string{"--message", ""}, apiclient.StrategyUpdate{Name: "Test", Expression: "m_rsi < 40", Message: ""}, false},
		{"add indicators", []string{"--add-indicators"}, apiclient.StrategyUpdate{Name: "Test", Expression: "m_rsi < 40", Message: "custom"}, true},
		{
			"exits", []string{"--exit", "pnl > 5 || w_rsi > 70", "--take-profit", "h_close * 1.1", "--stop-loss", "w_low", "--add-indicators"},
			apiclient.StrategyUpdate{
				Name: "Test", Expression: "m_rsi < 40", ExitExpression: "pnl > 5 || w_rsi > 70", TakeProfitExpression: "h_close * 1.1", StopLossExpression: "w_low", Message: "custom",
			}, true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := `{"id":6,"name":"Test","expression":"w_rsi < 50","message":"custom","enabled":false,"valid":true}`
			api, server := newFakeAPI(t, map[string]string{
				"GET /api/v1/admin/strategies":                 `{"items":[{"id":6,"name":"Test","expression":"m_rsi < 40","message":"custom","enabled":false,"valid":true}]}`,
				"POST /api/v1/admin/strategy-validations":      `{"errors":[],"missing_indicators":[{"interval":"1w","type":"rsi","parameters":{"period":14},"title":"w-rsi"}]}`,
				"POST /api/v1/admin/scanner-indicator-batches": `{"items":[]}`,
				"PUT /api/v1/admin/strategies/6":               response,
			})
			got := runCLI(t, writeProfile(t, server.URL), "", append([]string{"strategies", "update", "6", "--json"}, test.flags...)...)
			if got.code != 0 || got.stdout != response+"\n" {
				t.Fatalf("update = %+v", got)
			}
			var body apiclient.StrategyUpdate
			raw := api.bodies["PUT /api/v1/admin/strategies/6"]
			if err := json.Unmarshal([]byte(raw), &body); err != nil || !reflect.DeepEqual(body, test.want) {
				t.Fatalf("body = %s, want %+v", raw, test.want)
			}
			if strings.Contains(raw, "enabled") {
				t.Fatal("update must not send enabled state")
			}
			_, added := api.bodies["POST /api/v1/admin/scanner-indicator-batches"]
			if added != test.added {
				t.Fatalf("added indicators = %v", added)
			}
		})
	}
}

// --add-indicators alone sends no update when nothing is missing.
func TestUpdateStrategyWithNothingToAddSendsNothing(t *testing.T) {
	_, server := newFakeAPI(t, map[string]string{
		"GET /api/v1/admin/strategies":            `{"items":[{"id":6,"name":"Test","expression":"m_rsi < 40","message":"","enabled":false,"valid":true}]}`,
		"POST /api/v1/admin/strategy-validations": `{"errors":[],"missing_indicators":[]}`,
	})
	if got := runCLI(t, writeProfile(t, server.URL), "", "strategies", "update", "6", "--add-indicators"); got.code != 0 || got.stdout != "no indicators to add\n" {
		t.Fatalf("update = %+v", got)
	}
}

func TestUpdateStrategyRejectsInvalidInputBeforeWrites(t *testing.T) {
	for _, test := range []struct {
		name             string
		args             []string
		list, validation string
		stderr           string
	}{
		{name: "bad ID", args: []string{"0", "--name", "New"}},
		{name: "overflow ID", args: []string{"9223372036854775808", "--name", "New"}},
		{name: "no fields", args: []string{"6"}},
		{name: "missing strategy", args: []string{"6", "--name", "New"}, list: `{"items":[]}`},
		{name: "invalid expression", args: []string{"6", "--expr", "bad"}, list: `{"items":[{"id":6,"name":"Test","expression":"m_rsi < 40","message":"","enabled":false,"valid":true}]}`, validation: `{"errors":["invalid expression"],"missing_indicators":[]}`},
		{name: "enabled strategy", args: []string{"6", "--expr", "w_rsi < 50"}, list: `{"items":[{"id":6,"name":"Test","expression":"m_rsi < 40","message":"","enabled":true,"valid":true}]}`, stderr: "scanner: strategy 6 is enabled; edit it in the Mini App\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			api, server := newFakeAPI(t, map[string]string{"GET /api/v1/admin/strategies": test.list, "POST /api/v1/admin/strategy-validations": test.validation})
			got := runCLI(t, writeProfile(t, server.URL), "", append([]string{"strategies", "update"}, test.args...)...)
			if got.code != 1 || got.stderr == "" || test.stderr != "" && got.stderr != test.stderr {
				t.Fatalf("update = %+v", got)
			}
			for key := range api.bodies {
				if strings.HasPrefix(key, "PUT ") || strings.HasPrefix(key, "POST /api/v1/admin/scanner-indicator-batches") || test.stderr != "" && key != "GET /api/v1/admin/strategies" {
					t.Fatalf("unexpected write %s", key)
				}
			}
		})
	}
}
