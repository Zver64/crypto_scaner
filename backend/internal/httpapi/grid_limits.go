package httpapi

import (
	"context"
	"errors"
	"net/http"

	"crypto-scanner/internal/market"
	"crypto-scanner/internal/market/gridlimits"
)

// GridLimits reads the grid bot limits of an active instrument.
type GridLimits interface {
	Limits(context.Context, string) (gridlimits.Limits, error)
}

func (api *api) GetInstrumentGridLimits(ctx context.Context, request GetInstrumentGridLimitsRequestObject) (GetInstrumentGridLimitsResponseObject, error) {
	symbol := market.NormalizeSymbol(request.Symbol)
	if symbol == "" {
		return GetInstrumentGridLimits400JSONResponse{invalidArgument(ctx, "Symbol is required").badRequest()}, nil
	}
	limits, err := api.GridLimits.Limits(ctx, symbol)
	if errors.Is(err, market.ErrInstrumentNotFound) {
		return GetInstrumentGridLimits404JSONResponse{symbolNotFound(ctx).symbolNotFound()}, nil
	}
	if err != nil {
		attributes := []any{"module", "httpapi", "request_id", RequestIdentifier(ctx), "symbol", symbol, "error", err}
		if clientGone(ctx, err) {
			api.logger.InfoContext(ctx, "grid limits request canceled by the client", attributes...)
		} else {
			api.logger.WarnContext(ctx, "grid limits unavailable", attributes...)
		}
		return GetInstrumentGridLimits503JSONResponse{newAPIError(ctx, http.StatusServiceUnavailable, "market_data_unavailable", "Grid limits are unavailable", nil).unavailable()}, nil
	}
	return GetInstrumentGridLimits200JSONResponse(gridLimitsResponse(limits)), nil
}

func gridLimitsResponse(limits gridlimits.Limits) GridLimitsResponse {
	spot := limits.Spot
	response := GridLimitsResponse{
		Symbol: limits.Symbol,
		Spot: SpotGridLimits{
			AveragePrice: spot.AveragePrice, BidMultiplierDown: spot.BidMultiplierDown, AskMultiplierUp: spot.AskMultiplierUp,
			MinPrice: spot.MinPrice, MaxPrice: spot.MaxPrice, TickSize: spot.TickSize,
			MinQty: spot.MinQty, StepSize: spot.StepSize, MinNotional: spot.MinNotional,
		},
	}
	if usdm := limits.Usdm; usdm != nil {
		response.Usdm = &UsdmGridLimits{
			Symbol: usdm.Symbol, MarkPrice: usdm.MarkPrice, TickSize: usdm.TickSize,
			MinQty: usdm.MinQty, StepSize: usdm.StepSize, MinNotional: usdm.MinNotional,
		}
	}
	if coinm := limits.Coinm; coinm != nil {
		response.Coinm = &CoinmGridLimits{
			Symbol: coinm.Symbol, MarkPrice: coinm.MarkPrice, TickSize: coinm.TickSize,
			MinQty: coinm.MinQty, StepSize: coinm.StepSize,
			ContractSize: coinm.ContractSize, EqualQtyPrecision: coinm.EqualQtyPrecision,
		}
	}
	return response
}
