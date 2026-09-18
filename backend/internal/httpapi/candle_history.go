package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"crypto-scanner/internal/market"
)

const (
	defaultCandlePageSize = 200
	maxCandlePageSize     = 500
)

type CandleHistory interface {
	GetActiveInstrumentBySymbol(context.Context, string) (market.Instrument, error)
	ListCandlePage(context.Context, int64, market.CandleInterval, *time.Time, int) (market.CandlePage, error)
}

func (api *api) ListInstrumentCandles(ctx context.Context, request ListInstrumentCandlesRequestObject) (ListInstrumentCandlesResponseObject, error) {
	interval := market.CandleInterval(request.Params.Interval)
	limit := defaultCandlePageSize
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	symbol := strings.ToUpper(strings.TrimSpace(request.Symbol))
	if symbol == "" {
		return api.candleError(ctx, http.StatusBadRequest, "invalid_argument", "Symbol is required"), nil
	}
	instrument, err := api.history.GetActiveInstrumentBySymbol(ctx, symbol)
	if errors.Is(err, market.ErrInstrumentNotFound) {
		return api.candleError(ctx, http.StatusNotFound, "symbol_not_found", "Symbol is unknown or inactive"), nil
	}
	if err != nil {
		return api.candleError(ctx, http.StatusInternalServerError, "internal_error", "Internal server error"), nil
	}
	page, err := api.history.ListCandlePage(ctx, instrument.ID, interval, request.Params.Before, limit)
	if err != nil {
		return api.candleError(ctx, http.StatusInternalServerError, "internal_error", "Internal server error"), nil
	}
	candles := make([]Candle, len(page.Candles))
	for index, candle := range page.Candles {
		candles[index] = candleResponse(candle)
	}
	var nextBefore *time.Time
	if page.HasMore && len(page.Candles) > 0 {
		value := page.Candles[0].OpenTime.UTC()
		nextBefore = &value
	}
	return ListInstrumentCandles200JSONResponse{
		Body:    CandlePageResponse{Symbol: instrument.Symbol, Interval: CandleInterval(interval), Candles: candles, HasMore: page.HasMore, NextBefore: nextBefore},
		Headers: ListInstrumentCandles200ResponseHeaders{XRequestID: RequestIdentifier(ctx)},
	}, nil
}

func (api *api) candleError(ctx context.Context, status int, code, message string) ListInstrumentCandlesResponseObject {
	body := newErrorResponse(ctx, code, message, nil)
	requestID := body.RequestId
	switch status {
	case http.StatusBadRequest:
		return ListInstrumentCandles400JSONResponse{BadRequestJSONResponse: BadRequestJSONResponse{Body: body, Headers: BadRequestResponseHeaders{XRequestID: requestID}}}
	case http.StatusNotFound:
		return ListInstrumentCandles404JSONResponse{SymbolNotFoundJSONResponse: SymbolNotFoundJSONResponse{Body: body, Headers: SymbolNotFoundResponseHeaders{XRequestID: requestID}}}
	default:
		return ListInstrumentCandles500JSONResponse{InternalErrorJSONResponse: InternalErrorJSONResponse{Body: body, Headers: InternalErrorResponseHeaders{XRequestID: requestID}}}
	}
}
