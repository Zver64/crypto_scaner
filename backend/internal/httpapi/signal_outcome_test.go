package httpapi_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"crypto-scanner/internal/httpapi"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/platform/logging"
	"crypto-scanner/internal/strategy"
)

func TestSignalOutcomeResponseKeepsNeutralAndUnevaluatedDistinct(t *testing.T) {
	result := strategy.Backtest{
		Interval: market.IntervalHour, Symbol: "BTCUSDT", Direction: strategy.DirectionLong,
		Signal: &strategy.SignalReport{
			Window: 6, TargetRatio: 2, Evaluated: 3,
			Signals: strategy.SignalStats{Count: 3, Successes: 1, Neutrals: 1},
			All:     strategy.SignalStats{Count: 2, Neutrals: 2},
			Occurrences: []strategy.SignalOccurrence{
				{Evaluation: &strategy.SignalEvaluation{Outcome: strategy.SignalSuccess}},
				{Evaluation: &strategy.SignalEvaluation{Outcome: strategy.SignalFailure}},
				{Evaluation: &strategy.SignalEvaluation{Outcome: strategy.SignalNeutral}},
				{},
			},
		},
	}
	handler := httpapi.New(logging.New(io.Discard, "error", logging.Options{}), httpapi.Dependencies{
		Readiness: readinessStub{}, Analysis: unavailableAnalysis{}, Sessions: roleSessions{}, Strategies: &backtestStrategies{result: result},
	}, httpapi.Options{})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/strategies/1/backtest?symbol=BTCUSDT", nil)
	request.Header.Set("Authorization", "Bearer admin")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("response: %d %s", response.Code, response.Body.String())
	}
	var body httpapi.StrategyBacktest
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Signal == nil {
		t.Fatal("missing signal response")
	}
	report := body.Signal
	for i, want := range []httpapi.BacktestSignalOutcome{httpapi.Success, httpapi.Failure, httpapi.Neutral} {
		if report.Occurrences[i].Outcome == nil || *report.Occurrences[i].Outcome != want {
			t.Fatalf("lost outcome at %d", i)
		}
	}
	if report.Occurrences[3].Outcome != nil || report.Occurrences[3].Counted != nil {
		t.Fatal("unevaluated outcome is not null")
	}
	if report.Signals.Neutrals != 1 || report.Signals.SuccessRate == nil || *report.Signals.SuccessRate != 0.5 ||
		report.All.Neutrals != 2 || report.All.SuccessRate != nil {
		t.Fatalf("unexpected signal statistics: %+v", report)
	}
}
