package httpapi

import (
	"context"
	"errors"
	"net/http"

	chartservice "crypto-scanner/internal/chart"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
)

type ChartService interface {
	Build(context.Context, chartservice.Request) (chartservice.Page, error)
}

func (api *api) GetInstrumentChart(ctx context.Context, request GetInstrumentChartRequestObject) (GetInstrumentChartResponseObject, error) {
	if request.Body == nil || api.chart == nil {
		return api.chartError(ctx, http.StatusInternalServerError, "internal_error", "Internal server error"), nil
	}
	limit := defaultCandlePageSize
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	configs := make([]chartservice.IndicatorConfig, len(request.Body.Indicators))
	for index, config := range request.Body.Indicators {
		configs[index] = chartservice.IndicatorConfig{Type: indicator.Type(config.Type), Parameters: indicator.Parameters(config.Parameters)}
	}
	page, err := api.chart.Build(ctx, chartservice.Request{
		Symbol:     request.Symbol,
		Interval:   market.CandleInterval(request.Params.Interval),
		Before:     request.Params.Before,
		Limit:      limit,
		Indicators: configs,
	})
	if errors.Is(err, chartservice.ErrInvalidRequest) {
		return api.chartError(ctx, http.StatusBadRequest, "invalid_argument", "Invalid chart indicator configuration"), nil
	}
	if errors.Is(err, market.ErrInstrumentNotFound) {
		return api.chartError(ctx, http.StatusNotFound, "symbol_not_found", "Symbol is unknown or inactive"), nil
	}
	if err != nil {
		return api.chartError(ctx, http.StatusInternalServerError, "internal_error", "Internal server error"), nil
	}

	candles := make([]Candle, len(page.Candles))
	for index, candle := range page.Candles {
		candles[index] = candleResponse(candle)
	}
	results := make([]ChartIndicatorResult, len(page.Indicators))
	for resultIndex, result := range page.Indicators {
		series := make([]IndicatorSeries, len(result.Series))
		for seriesIndex, item := range result.Series {
			points := make([]IndicatorPoint, len(item.Points))
			for pointIndex, point := range item.Points {
				points[pointIndex] = IndicatorPoint{Time: point.Time.UTC(), Value: point.Value}
			}
			series[seriesIndex] = IndicatorSeries{Name: item.Name, Points: points}
		}
		results[resultIndex] = ChartIndicatorResult{Type: string(result.Type), Parameters: map[string]interface{}(result.Parameters), Series: series}
	}
	return GetInstrumentChart200JSONResponse{
		Body: ChartPageResponse{
			Symbol: page.Symbol, Interval: CandleInterval(request.Params.Interval), Candles: candles,
			Indicators: results, HasMore: page.HasMore, NextBefore: page.NextBefore,
		},
		Headers: GetInstrumentChart200ResponseHeaders{XRequestID: RequestIdentifier(ctx)},
	}, nil
}

func candleResponse(candle market.Candle) Candle {
	return Candle{
		OpenTime: candle.OpenTime.UTC(), CloseTime: candle.CloseTime.UTC(),
		Open: candle.Open, High: candle.High, Low: candle.Low, Close: candle.Close,
		Volume: candle.Volume, QuoteAssetVolume: candle.QuoteAssetVolume, TradeCount: candle.TradeCount,
	}
}

func (api *api) chartError(ctx context.Context, status int, code, message string) GetInstrumentChartResponseObject {
	body := newErrorResponse(ctx, code, message, nil)
	requestID := body.RequestId
	switch status {
	case http.StatusBadRequest:
		return GetInstrumentChart400JSONResponse{BadRequestJSONResponse: BadRequestJSONResponse{Body: body, Headers: BadRequestResponseHeaders{XRequestID: requestID}}}
	case http.StatusNotFound:
		return GetInstrumentChart404JSONResponse{SymbolNotFoundJSONResponse: SymbolNotFoundJSONResponse{Body: body, Headers: SymbolNotFoundResponseHeaders{XRequestID: requestID}}}
	default:
		return GetInstrumentChart500JSONResponse{InternalErrorJSONResponse: InternalErrorJSONResponse{Body: body, Headers: InternalErrorResponseHeaders{XRequestID: requestID}}}
	}
}
