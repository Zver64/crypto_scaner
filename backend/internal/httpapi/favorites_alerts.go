package httpapi

import (
	"context"
	"errors"

	"crypto-scanner/internal/alerts"
	authtelegram "crypto-scanner/internal/auth/telegram"
	"crypto-scanner/internal/favorites"
	"crypto-scanner/internal/market"
)

func currentUserID(ctx context.Context) int64 {
	user, _ := authtelegram.UserFromContext(ctx)
	return user.ID
}
func favoriteDTO(v favorites.Favorite) Favorite {
	return Favorite{Symbol: v.Symbol, BaseAsset: v.BaseAsset, QuoteAsset: v.QuoteAsset, Active: v.Active, AlertCount: v.AlertCount, CreatedAt: v.CreatedAt, ClosedIndicators: closedIndicatorsResponse(v.ClosedIndicators)}
}
func alertDTO(v alerts.Alert) PriceAlert {
	return PriceAlert{Id: v.ID, Symbol: v.Symbol, Target: v.Target, Version: v.Version, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}
func (api *api) ListFavorites(ctx context.Context, _ ListFavoritesRequestObject) (ListFavoritesResponseObject, error) {
	items, err := api.favorites.List(ctx, currentUserID(ctx))
	if err != nil {
		return ListFavorites500JSONResponse{InternalErrorJSONResponse{Body: newErrorResponse(ctx, "internal_error", "Internal server error", nil)}}, nil
	}
	result := make([]Favorite, len(items))
	for i, v := range items {
		result[i] = favoriteDTO(v)
	}
	return ListFavorites200JSONResponse{Items: result}, nil
}
func (api *api) AnalyzeFavorites(ctx context.Context, request AnalyzeFavoritesRequestObject) (AnalyzeFavoritesResponseObject, error) {
	body := MarketAnalysisRequest{}
	if request.Body != nil {
		body = *request.Body
	}
	result, err := api.favorites.Analyze(ctx, currentUserID(ctx), marketSearchRequest(body))
	if err != nil {
		errorBody, status := analysisError(ctx, err, "")
		requestID := RequestIdentifier(ctx)
		switch status {
		case 400:
			return AnalyzeFavorites400JSONResponse{BadRequestJSONResponse{Body: errorBody, Headers: BadRequestResponseHeaders{XRequestID: requestID}}}, nil
		case 422:
			return AnalyzeFavorites422JSONResponse{UnprocessableAnalysisJSONResponse: UnprocessableAnalysisJSONResponse{Body: errorBody, Headers: UnprocessableAnalysisResponseHeaders{XRequestID: requestID}}}, nil
		case 503:
			return AnalyzeFavorites503JSONResponse{AnalysisUnavailableJSONResponse: AnalysisUnavailableJSONResponse{Body: errorBody, Headers: AnalysisUnavailableResponseHeaders{XRequestID: requestID}}}, nil
		default:
			return AnalyzeFavorites500JSONResponse{InternalErrorJSONResponse{Body: errorBody, Headers: InternalErrorResponseHeaders{XRequestID: requestID}}}, nil
		}
	}
	return AnalyzeFavorites200JSONResponse(marketAnalysisResponse(result)), nil
}

func (api *api) AddFavorite(ctx context.Context, request AddFavoriteRequestObject) (AddFavoriteResponseObject, error) {
	item, err := api.favorites.Add(ctx, currentUserID(ctx), request.Symbol)
	if errors.Is(err, market.ErrInstrumentNotFound) {
		return AddFavorite404JSONResponse{SymbolNotFoundJSONResponse{Body: newErrorResponse(ctx, "symbol_not_found", "Symbol is unknown or inactive", nil)}}, nil
	}
	if err != nil {
		return AddFavorite500JSONResponse{InternalErrorJSONResponse{Body: newErrorResponse(ctx, "internal_error", "Internal server error", nil)}}, nil
	}
	return AddFavorite200JSONResponse(favoriteDTO(item)), nil
}
func (api *api) RemoveFavorite(ctx context.Context, request RemoveFavoriteRequestObject) (RemoveFavoriteResponseObject, error) {
	confirm := request.Params.ConfirmAlerts != nil && *request.Params.ConfirmAlerts
	count, err := api.favorites.Remove(ctx, currentUserID(ctx), request.Symbol, confirm)
	if errors.Is(err, favorites.ErrAlertsExist) {
		return RemoveFavorite409JSONResponse{FavoriteAlertsConflictJSONResponse: FavoriteAlertsConflictJSONResponse(newErrorResponse(ctx, "favorite_has_alerts", "Confirmation is required to delete existing alerts", map[string]any{"alert_count": count}))}, nil
	}
	if errors.Is(err, favorites.ErrNotFound) {
		return RemoveFavorite404JSONResponse{FavoriteNotFoundJSONResponse: FavoriteNotFoundJSONResponse(newErrorResponse(ctx, "favorite_not_found", "Favorite not found", nil))}, nil
	}
	if err != nil {
		return RemoveFavorite500JSONResponse{InternalErrorJSONResponse{Body: newErrorResponse(ctx, "internal_error", "Internal server error", nil)}}, nil
	}
	return RemoveFavorite204Response{}, nil
}
func (api *api) ListPriceAlerts(ctx context.Context, request ListPriceAlertsRequestObject) (ListPriceAlertsResponseObject, error) {
	items, err := api.alerts.List(ctx, currentUserID(ctx), request.Symbol)
	if err != nil {
		return ListPriceAlerts500JSONResponse{InternalErrorJSONResponse{Body: newErrorResponse(ctx, "internal_error", "Internal server error", nil)}}, nil
	}
	result := make([]PriceAlert, len(items))
	for i, v := range items {
		result[i] = alertDTO(v)
	}
	return ListPriceAlerts200JSONResponse{Items: result, Count: len(result), Limit: alerts.MaxPerInstrument}, nil
}
func (api *api) CreatePriceAlert(ctx context.Context, request CreatePriceAlertRequestObject) (CreatePriceAlertResponseObject, error) {
	item, err := api.alerts.Create(ctx, currentUserID(ctx), request.Symbol, request.Body.Target)
	if response := createAlertError(ctx, err); response != nil {
		return response, nil
	}
	return CreatePriceAlert201JSONResponse(alertDTO(item)), nil
}
func createAlertError(ctx context.Context, err error) CreatePriceAlertResponseObject {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, alerts.ErrInvalidTarget):
		return CreatePriceAlert400JSONResponse{BadRequestJSONResponse{Body: newErrorResponse(ctx, "invalid_argument", "Target must be a positive decimal", nil)}}
	case errors.Is(err, market.ErrInstrumentNotFound):
		return CreatePriceAlert404JSONResponse{SymbolNotFoundJSONResponse{Body: newErrorResponse(ctx, "symbol_not_found", "Symbol is unknown or inactive", nil)}}
	case errors.Is(err, alerts.ErrDuplicate):
		return CreatePriceAlert409JSONResponse{AlertConflictJSONResponse: AlertConflictJSONResponse(newErrorResponse(ctx, "duplicate_target", "An alert with this target already exists", nil))}
	case errors.Is(err, alerts.ErrLimit):
		return CreatePriceAlert409JSONResponse{AlertConflictJSONResponse: AlertConflictJSONResponse(newErrorResponse(ctx, "alert_limit", "At most 10 alerts are allowed per instrument", map[string]any{"limit": alerts.MaxPerInstrument}))}
	default:
		return CreatePriceAlert500JSONResponse{InternalErrorJSONResponse{Body: newErrorResponse(ctx, "internal_error", "Internal server error", nil)}}
	}
}
func (api *api) UpdatePriceAlert(ctx context.Context, request UpdatePriceAlertRequestObject) (UpdatePriceAlertResponseObject, error) {
	item, err := api.alerts.Update(ctx, currentUserID(ctx), request.AlertId, request.Body.Target)
	switch {
	case err == nil:
		return UpdatePriceAlert200JSONResponse(alertDTO(item)), nil
	case errors.Is(err, alerts.ErrInvalidTarget):
		return UpdatePriceAlert400JSONResponse{BadRequestJSONResponse{Body: newErrorResponse(ctx, "invalid_argument", "Target must be a positive decimal", nil)}}, nil
	case errors.Is(err, alerts.ErrNotFound):
		return UpdatePriceAlert404JSONResponse{AlertNotFoundJSONResponse: AlertNotFoundJSONResponse(newErrorResponse(ctx, "alert_not_found", "Price alert not found", nil))}, nil
	case errors.Is(err, alerts.ErrDuplicate):
		return UpdatePriceAlert409JSONResponse{AlertConflictJSONResponse: AlertConflictJSONResponse(newErrorResponse(ctx, "duplicate_target", "An alert with this target already exists", nil))}, nil
	default:
		return UpdatePriceAlert500JSONResponse{InternalErrorJSONResponse{Body: newErrorResponse(ctx, "internal_error", "Internal server error", nil)}}, nil
	}
}
func (api *api) DeletePriceAlert(ctx context.Context, request DeletePriceAlertRequestObject) (DeletePriceAlertResponseObject, error) {
	err := api.alerts.Delete(ctx, currentUserID(ctx), request.AlertId)
	if errors.Is(err, alerts.ErrNotFound) {
		return DeletePriceAlert404JSONResponse{AlertNotFoundJSONResponse: AlertNotFoundJSONResponse(newErrorResponse(ctx, "alert_not_found", "Price alert not found", nil))}, nil
	}
	if err != nil {
		return DeletePriceAlert500JSONResponse{InternalErrorJSONResponse{Body: newErrorResponse(ctx, "internal_error", "Internal server error", nil)}}, nil
	}
	return DeletePriceAlert204Response{}, nil
}
