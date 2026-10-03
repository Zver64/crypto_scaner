package httpapi

import (
	"context"
	"errors"
	"net/http"

	"crypto-scanner/internal/market"
)

// PriceLimits reads the exchange price limits of an instrument.
type PriceLimits interface {
	Limits(context.Context, string) (market.PriceLimits, error)
}

func (api *api) GetInstrumentPriceLimits(ctx context.Context, request GetInstrumentPriceLimitsRequestObject) (GetInstrumentPriceLimitsResponseObject, error) {
	symbol := market.NormalizeSymbol(request.Symbol)
	if symbol == "" {
		return GetInstrumentPriceLimits400JSONResponse{invalidArgument(ctx, "Symbol is required").badRequest()}, nil
	}
	instrument, err := api.history.GetActiveInstrumentBySymbol(ctx, symbol)
	if errors.Is(err, market.ErrInstrumentNotFound) {
		return GetInstrumentPriceLimits404JSONResponse{symbolNotFound(ctx).symbolNotFound()}, nil
	}
	if err != nil {
		return GetInstrumentPriceLimits500JSONResponse{api.internalError(ctx, "get_instrument", err)}, nil
	}
	limits, err := api.priceLimits.Limits(ctx, instrument.Symbol)
	if errors.Is(err, market.ErrInstrumentNotFound) {
		return GetInstrumentPriceLimits404JSONResponse{symbolNotFound(ctx).symbolNotFound()}, nil
	}
	if err != nil {
		api.logger.WarnContext(ctx, "price limits unavailable", "module", "httpapi", "request_id", RequestIdentifier(ctx), "symbol", instrument.Symbol, "error", err)
		return GetInstrumentPriceLimits503JSONResponse{newAPIError(ctx, http.StatusServiceUnavailable, "market_data_unavailable", "Price limits are unavailable", nil).unavailable()}, nil
	}
	body := PriceLimitsResponse{Symbol: instrument.Symbol, ReferencePrice: limits.ReferencePrice}
	if limits.Range != nil {
		body.BidLimitMultDown = &limits.Range.BidLimitMultDown
		body.AskLimitMultUp = &limits.Range.AskLimitMultUp
	}
	return GetInstrumentPriceLimits200JSONResponse{
		Body:    body,
		Headers: GetInstrumentPriceLimits200ResponseHeaders{XRequestID: RequestIdentifier(ctx)},
	}, nil
}
