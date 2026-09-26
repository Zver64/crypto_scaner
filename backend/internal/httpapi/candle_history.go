package httpapi

import (
	"context"
	"errors"
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
	symbol := market.NormalizeSymbol(request.Symbol)
	if symbol == "" {
		return ListInstrumentCandles400JSONResponse{invalidArgument(ctx, "Symbol is required").badRequest()}, nil
	}
	instrument, err := api.history.GetActiveInstrumentBySymbol(ctx, symbol)
	if errors.Is(err, market.ErrInstrumentNotFound) {
		return ListInstrumentCandles404JSONResponse{symbolNotFound(ctx).symbolNotFound()}, nil
	}
	if err != nil {
		return ListInstrumentCandles500JSONResponse{api.internalError(ctx, "get_instrument", err)}, nil
	}
	page, err := api.history.ListCandlePage(ctx, instrument.ID, interval, request.Params.Before, limit)
	if err != nil {
		return ListInstrumentCandles500JSONResponse{api.internalError(ctx, "list_candles", err)}, nil
	}
	candles := make([]Candle, len(page.Candles))
	for index, candle := range page.Candles {
		candles[index] = candleResponse(candle)
	}
	return ListInstrumentCandles200JSONResponse{
		Body:    CandlePageResponse{Symbol: instrument.Symbol, Interval: CandleInterval(interval), Candles: candles, HasMore: page.HasMore, NextBefore: page.NextBefore()},
		Headers: ListInstrumentCandles200ResponseHeaders{XRequestID: RequestIdentifier(ctx)},
	}, nil
}

func candleResponse(candle market.Candle) Candle {
	return Candle{
		OpenTime: candle.OpenTime.UTC(), CloseTime: candle.CloseTime.UTC(),
		Open: candle.Open, High: candle.High, Low: candle.Low, Close: candle.Close,
		Volume: candle.Volume, QuoteAssetVolume: candle.QuoteAssetVolume, TradeCount: candle.TradeCount,
	}
}
