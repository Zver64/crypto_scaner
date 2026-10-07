package httpapi_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"crypto-scanner/internal/httpapi"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/market/marketsync"
	"crypto-scanner/internal/platform/logging"
)

func TestCandleHistoryLoadsStartAJobAndReportIt(t *testing.T) {
	started := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	oldest := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	loads := &historyLoads{}
	handler := httpapi.New(logging.New(io.Discard, "error", logging.Options{}), httpapi.Dependencies{
		Readiness: readinessStub{}, Analysis: unavailableAnalysis{}, Sessions: roleSessions{}, HistoryLoads: loads,
	}, httpapi.Options{})
	serve := func(method, token, body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, "/api/v1/admin/candle-history-loads", strings.NewReader(body))
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}

	if response := serve(http.MethodGet, "program", ""); response.Code != http.StatusNoContent {
		t.Fatalf("GET before any job: status = %d, body = %s", response.Code, response.Body.String())
	}
	for _, test := range []struct {
		name, token, body, code string
		status                  int
	}{
		{name: "user", token: "user", body: `{"symbols":["BTCUSDT"],"intervals":["1h"],"depth":20000}`, status: http.StatusForbidden, code: "administrator_required"},
		{name: "API token", token: "program", body: `{"symbols":["BTCUSDT"],"intervals":["1h"],"depth":20000}`, status: http.StatusForbidden, code: "session_required"},
		{name: "invalid load", token: "admin", body: `{"symbols":["BTCUSDT","btcusdt"],"intervals":["1h"],"depth":20000}`, status: http.StatusBadRequest, code: "invalid_argument"},
		{name: "unknown symbol", token: "admin", body: `{"symbols":["ETHUSDT"],"intervals":["1h"],"depth":20000}`, status: http.StatusNotFound, code: "symbol_not_found"},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := serve(http.MethodPost, test.token, test.body)
			if response.Code != test.status {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			assertErrorCode(t, response, test.code)
		})
	}

	loads.job = marketsync.HistoryJob{Status: marketsync.HistoryJobRunning, Symbols: []string{"BTCUSDT"}, Intervals: []market.CandleInterval{market.IntervalHour, market.IntervalDay}, Depth: 20000, StartedAt: started}
	response := serve(http.MethodPost, "admin", `{"symbols":["BTCUSDT"],"intervals":["1h","1d"],"depth":20000}`)
	if response.Code != http.StatusAccepted || loads.depth != 20000 || !slices.Equal(loads.intervals, loads.job.Intervals) {
		t.Fatalf("start: status = %d, body = %s, depth = %d", response.Code, response.Body.String(), loads.depth)
	}
	if got, want := strings.TrimSpace(response.Body.String()), `{"depth":20000,"error":null,"finished_at":null,"history_changed":false,"intervals":["1h","1d"],"items":[],"started_at":"2026-10-07T12:00:00Z","status":"running","symbols":["BTCUSDT"]}`; got != want {
		t.Fatalf("started job = %s, want %s", got, want)
	}
	if response := serve(http.MethodPost, "admin", `{"symbols":["BTCUSDT"],"intervals":["1h"],"depth":20000}`); response.Code != http.StatusConflict {
		t.Fatalf("second start: status = %d, body = %s", response.Code, response.Body.String())
	} else {
		assertErrorCode(t, response, "history_load_running")
	}

	loads.job.Status, loads.job.FinishedAt, loads.job.Error = marketsync.HistoryJobFailed, started.Add(time.Minute), "Binance is down"
	loads.job.HistoryChanged = true
	loads.job.Items = []marketsync.HistoryLoad{{Symbol: "BTCUSDT", Interval: market.IntervalHour, Count: 20000, Oldest: oldest}}
	response = serve(http.MethodGet, "admin", "")
	var job httpapi.CandleHistoryLoadJob
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &job) != nil {
		t.Fatalf("GET: status = %d, body = %s", response.Code, response.Body.String())
	}
	if job.Status != httpapi.Failed || !job.HistoryChanged || job.Error == nil || *job.Error != "Binance is down" || job.FinishedAt == nil ||
		len(job.Items) != 1 || job.Items[0].OldestOpenTime == nil || !job.Items[0].OldestOpenTime.Equal(oldest) || job.Items[0].Count != 20000 {
		t.Fatalf("GET body = %s", response.Body.String())
	}
}

// historyLoads knows only BTCUSDT, rejects two symbols as invalid, and holds
// the job a test sets; a job that is running rejects a start.
type historyLoads struct {
	job       marketsync.HistoryJob
	intervals []market.CandleInterval
	depth     int
	started   bool
}

func (loads *historyLoads) Start(_ context.Context, symbols []string, intervals []market.CandleInterval, depth int) (marketsync.HistoryJob, error) {
	switch {
	case len(symbols) == 2:
		return marketsync.HistoryJob{}, marketsync.ErrInvalidHistoryLoad
	case !slices.Equal(symbols, []string{"BTCUSDT"}):
		return marketsync.HistoryJob{}, market.ErrInstrumentNotFound
	case loads.started:
		return marketsync.HistoryJob{}, marketsync.ErrHistoryLoadRunning
	}
	loads.started, loads.intervals, loads.depth = true, intervals, depth
	return loads.job, nil
}

func (loads *historyLoads) Job() (marketsync.HistoryJob, bool) {
	return loads.job, loads.job.Status != ""
}
