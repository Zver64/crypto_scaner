package httpapi_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"crypto-scanner/internal/httpapi"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/platform/logging"
	"crypto-scanner/internal/strategy"
)

func TestSignalOutcomeResponseKeepsNeutralAndUnevaluatedDistinct(t *testing.T) {
	exit := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	result := strategy.Backtest{
		Interval: market.IntervalHour, Symbol: "BTCUSDT", Direction: strategy.DirectionLong,
		Signal: &strategy.SignalReport{
			Judgement: &strategy.SignalJudgement{
				Window: 6, TargetRatio: 2, Evaluated: 3,
				Signals: strategy.SignalStats{Count: 3, Successes: 1, Neutrals: 1},
				All:     strategy.SignalStats{Count: 2, Neutrals: 2},
			},
			Occurrences: []strategy.SignalOccurrence{
				{Evaluation: &strategy.SignalEvaluation{Outcome: strategy.SignalSuccess, ExitTime: exit}},
				{Evaluation: &strategy.SignalEvaluation{Outcome: strategy.SignalFailure, ExitTime: exit}},
				{Evaluation: &strategy.SignalEvaluation{Outcome: strategy.SignalNeutral, ExitTime: exit}},
				{},
			},
		},
	}
	handler := httpapi.New(logging.New(io.Discard, "error", logging.Options{}), httpapi.Dependencies{
		Readiness: readinessStub{}, Analysis: unavailableAnalysis{}, Sessions: roleSessions{}, Strategies: &backtestStrategies{result: result},
	}, httpapi.Options{})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/strategies/"+strategyID(1).String()+"/backtest?symbol=BTCUSDT", nil)
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
		if report.Occurrences[i].Outcome == nil || *report.Occurrences[i].Outcome != want || report.Occurrences[i].ExitTime == nil || !report.Occurrences[i].ExitTime.Equal(exit) {
			t.Fatalf("lost outcome at %d", i)
		}
	}
	if report.Occurrences[3].Outcome != nil || report.Occurrences[3].Counted != nil || report.Occurrences[3].ExitTime != nil {
		t.Fatal("unevaluated outcome is not null")
	}
	evaluation := report.Evaluation
	if evaluation == nil || evaluation.Signals.Neutrals != 1 || evaluation.Signals.SuccessRate == nil || *evaluation.Signals.SuccessRate != 0.5 ||
		evaluation.All.Neutrals != 2 || evaluation.All.SuccessRate != nil {
		t.Fatalf("unexpected signal statistics: %+v", report)
	}
}
