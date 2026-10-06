package httpapi

import (
	"context"
	"errors"
	"net/http"

	"crypto-scanner/internal/market"
	"crypto-scanner/internal/market/gridlimits"
)

// GridLimits reads the spot grid bot price limits of an active instrument.
type GridLimits interface {
	Limits(context.Context, string) (gridlimits.Limits, error)
}

func (api *api) GetInstrumentGridLimits(ctx context.Context, request GetInstrumentGridLimitsRequestObject) (GetInstrumentGridLimitsResponseObject, error) {
	symbol := market.NormalizeSymbol(request.Symbol)
	if symbol == "" {
		return GetInstrumentGridLimits400JSONResponse{invalidArgument(ctx, "Symbol is required").badRequest()}, nil
	}
	limits, err := api.gridLimits.Limits(ctx, symbol)
	if errors.Is(err, market.ErrInstrumentNotFound) {
		return GetInstrumentGridLimits404JSONResponse{symbolNotFound(ctx).symbolNotFound()}, nil
	}
	if err != nil {
		api.logger.WarnContext(ctx, "grid limits unavailable", "module", "httpapi", "request_id", RequestIdentifier(ctx), "symbol", symbol, "error", err)
		return GetInstrumentGridLimits503JSONResponse{newAPIError(ctx, http.StatusServiceUnavailable, "market_data_unavailable", "Grid limits are unavailable", nil).unavailable()}, nil
	}
	return GetInstrumentGridLimits200JSONResponse{
		Body: GridLimitsResponse{
			Symbol: limits.Symbol, AveragePrice: limits.AveragePrice,
			BidMultiplierDown: limits.BidMultiplierDown, AskMultiplierUp: limits.AskMultiplierUp,
			MinPrice: limits.MinPrice, MaxPrice: limits.MaxPrice, TickSize: limits.TickSize,
		},
		Headers: GetInstrumentGridLimits200ResponseHeaders{XRequestID: RequestIdentifier(ctx)},
	}, nil
}
