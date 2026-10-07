package httpapi

import (
	"context"
	"errors"
	"net/http"

	"crypto-scanner/internal/market"
	"crypto-scanner/internal/market/marketsync"
)

// HistoryLoads runs history load jobs in the background.
type HistoryLoads interface {
	Start(ctx context.Context, symbols []string, intervals []market.CandleInterval, depth int) (marketsync.HistoryJob, error)
	Job() (marketsync.HistoryJob, bool)
}

func (api *api) StartCandleHistoryLoad(ctx context.Context, request StartCandleHistoryLoadRequestObject) (StartCandleHistoryLoadResponseObject, error) {
	// Loads call the exchange for minutes, so only a person starts them.
	if apiTokenUsed(ctx) {
		return StartCandleHistoryLoad403JSONResponse{sessionRequired(ctx, "History loads are started from the Mini App, not with an API token")}, nil
	}
	intervals := make([]market.CandleInterval, len(request.Body.Intervals))
	for i, interval := range request.Body.Intervals {
		intervals[i] = market.CandleInterval(interval)
	}
	job, err := api.historyLoads.Start(ctx, request.Body.Symbols, intervals, request.Body.Depth)
	switch {
	case err == nil:
		return StartCandleHistoryLoad202JSONResponse(historyJobDTO(job)), nil
	case errors.Is(err, marketsync.ErrInvalidHistoryLoad):
		return StartCandleHistoryLoad400JSONResponse{invalidArgument(ctx, err.Error()).badRequest()}, nil
	case errors.Is(err, market.ErrInstrumentNotFound):
		return StartCandleHistoryLoad404JSONResponse{symbolNotFound(ctx).symbolNotFound()}, nil
	case errors.Is(err, marketsync.ErrHistoryLoadRunning):
		body := newAPIError(ctx, http.StatusConflict, "history_load_running", "Another history load is running", nil).body
		return StartCandleHistoryLoad409JSONResponse{HistoryLoadRunningJSONResponse{Body: body, Headers: HistoryLoadRunningResponseHeaders{XRequestID: body.RequestId}}}, nil
	default:
		return StartCandleHistoryLoad500JSONResponse{api.internalError(ctx, "start_candle_history_load", err)}, nil
	}
}

func (api *api) GetCandleHistoryLoad(context.Context, GetCandleHistoryLoadRequestObject) (GetCandleHistoryLoadResponseObject, error) {
	job, ok := api.historyLoads.Job()
	if !ok {
		return GetCandleHistoryLoad204Response{}, nil
	}
	return GetCandleHistoryLoad200JSONResponse(historyJobDTO(job)), nil
}

func historyJobDTO(job marketsync.HistoryJob) CandleHistoryLoadJob {
	dto := CandleHistoryLoadJob{
		Status: CandleHistoryLoadJobStatus(job.Status), Symbols: job.Symbols, Depth: job.Depth, StartedAt: job.StartedAt,
		HistoryChanged: job.HistoryChanged,
		Intervals:      make([]CandleInterval, len(job.Intervals)), Items: make([]CandleHistoryLoad, len(job.Items)),
	}
	for i, interval := range job.Intervals {
		dto.Intervals[i] = CandleInterval(interval)
	}
	for i, load := range job.Items {
		dto.Items[i] = CandleHistoryLoad{Symbol: load.Symbol, Interval: CandleInterval(load.Interval), Count: load.Count, Exhausted: load.Exhausted, NotReady: load.NotReady}
		if load.Count > 0 {
			dto.Items[i].OldestOpenTime = &load.Oldest
		}
	}
	if !job.FinishedAt.IsZero() {
		dto.FinishedAt = &job.FinishedAt
	}
	if job.Error != "" {
		dto.Error = &job.Error
	}
	return dto
}
