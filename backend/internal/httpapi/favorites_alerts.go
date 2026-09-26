package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"crypto-scanner/internal/alerts"
	"crypto-scanner/internal/favorites"
	"crypto-scanner/internal/market"
)

const invalidTargetMessage = "Target must be a positive decimal"

func duplicateTarget(ctx context.Context) apiError {
	return newAPIError(ctx, http.StatusConflict, "duplicate_target", "An alert with this target already exists", nil)
}

func alertNotFound(ctx context.Context) apiError {
	return newAPIError(ctx, http.StatusNotFound, "alert_not_found", "Price alert not found", nil)
}

func currentUserID(ctx context.Context) int64 {
	user, _ := UserFromContext(ctx)
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
		return ListFavorites500JSONResponse{api.internalError(ctx, "list_favorites", err)}, nil
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
		mapped, ok := analysisError(ctx, err, "")
		switch {
		case ok && mapped.status == http.StatusBadRequest:
			return AnalyzeFavorites400JSONResponse{mapped.badRequest()}, nil
		case ok && mapped.status == http.StatusUnprocessableEntity:
			return AnalyzeFavorites422JSONResponse{mapped.unprocessable()}, nil
		case ok && mapped.status == http.StatusServiceUnavailable:
			return AnalyzeFavorites503JSONResponse{mapped.unavailable()}, nil
		default:
			return AnalyzeFavorites500JSONResponse{api.internalError(ctx, "analyze_favorites", err)}, nil
		}
	}
	return AnalyzeFavorites200JSONResponse(marketAnalysisResponse(result)), nil
}

func (api *api) AddFavorite(ctx context.Context, request AddFavoriteRequestObject) (AddFavoriteResponseObject, error) {
	item, err := api.favorites.Add(ctx, currentUserID(ctx), request.Symbol)
	if errors.Is(err, market.ErrInstrumentNotFound) {
		return AddFavorite404JSONResponse{symbolNotFound(ctx).symbolNotFound()}, nil
	}
	if err != nil {
		return AddFavorite500JSONResponse{api.internalError(ctx, "add_favorite", err)}, nil
	}
	return AddFavorite200JSONResponse(favoriteDTO(item)), nil
}
func (api *api) RemoveFavorite(ctx context.Context, request RemoveFavoriteRequestObject) (RemoveFavoriteResponseObject, error) {
	confirm := request.Params.ConfirmAlerts != nil && *request.Params.ConfirmAlerts
	count, err := api.favorites.Remove(ctx, currentUserID(ctx), request.Symbol, confirm)
	if errors.Is(err, favorites.ErrAlertsExist) {
		return RemoveFavorite409JSONResponse{FavoriteAlertsConflictJSONResponse: FavoriteAlertsConflictJSONResponse(newAPIError(ctx, http.StatusConflict, "favorite_has_alerts", "Confirmation is required to delete existing alerts", map[string]any{"alert_count": count}).body)}, nil
	}
	if errors.Is(err, favorites.ErrNotFound) {
		return RemoveFavorite404JSONResponse{FavoriteNotFoundJSONResponse: FavoriteNotFoundJSONResponse(newAPIError(ctx, http.StatusNotFound, "favorite_not_found", "Favorite not found", nil).body)}, nil
	}
	if err != nil {
		return RemoveFavorite500JSONResponse{api.internalError(ctx, "remove_favorite", err)}, nil
	}
	return RemoveFavorite204Response{}, nil
}
func (api *api) ListPriceAlerts(ctx context.Context, request ListPriceAlertsRequestObject) (ListPriceAlertsResponseObject, error) {
	items, err := api.alerts.List(ctx, currentUserID(ctx), request.Symbol)
	if err != nil {
		return ListPriceAlerts500JSONResponse{api.internalError(ctx, "list_price_alerts", err)}, nil
	}
	result := make([]PriceAlert, len(items))
	for i, v := range items {
		result[i] = alertDTO(v)
	}
	return ListPriceAlerts200JSONResponse{Items: result, Count: len(result), Limit: alerts.MaxPerInstrument}, nil
}
func (api *api) CreatePriceAlert(ctx context.Context, request CreatePriceAlertRequestObject) (CreatePriceAlertResponseObject, error) {
	item, err := api.alerts.Create(ctx, currentUserID(ctx), request.Symbol, request.Body.Target)
	if response := api.createAlertError(ctx, err); response != nil {
		return response, nil
	}
	return CreatePriceAlert201JSONResponse(alertDTO(item)), nil
}
func (api *api) createAlertError(ctx context.Context, err error) CreatePriceAlertResponseObject {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, alerts.ErrInvalidTarget):
		return CreatePriceAlert400JSONResponse{invalidArgument(ctx, invalidTargetMessage).badRequest()}
	case errors.Is(err, market.ErrInstrumentNotFound):
		return CreatePriceAlert404JSONResponse{symbolNotFound(ctx).symbolNotFound()}
	case errors.Is(err, alerts.ErrDuplicate):
		return CreatePriceAlert409JSONResponse{AlertConflictJSONResponse: AlertConflictJSONResponse(duplicateTarget(ctx).body)}
	case errors.Is(err, alerts.ErrLimit):
		return CreatePriceAlert409JSONResponse{AlertConflictJSONResponse: AlertConflictJSONResponse(newAPIError(ctx, http.StatusConflict, "alert_limit", fmt.Sprintf("At most %d alerts are allowed per instrument", alerts.MaxPerInstrument), map[string]any{"limit": alerts.MaxPerInstrument}).body)}
	default:
		return CreatePriceAlert500JSONResponse{api.internalError(ctx, "create_price_alert", err)}
	}
}
func (api *api) UpdatePriceAlert(ctx context.Context, request UpdatePriceAlertRequestObject) (UpdatePriceAlertResponseObject, error) {
	item, err := api.alerts.Update(ctx, currentUserID(ctx), request.AlertId, request.Body.Target)
	switch {
	case err == nil:
		return UpdatePriceAlert200JSONResponse(alertDTO(item)), nil
	case errors.Is(err, alerts.ErrInvalidTarget):
		return UpdatePriceAlert400JSONResponse{invalidArgument(ctx, invalidTargetMessage).badRequest()}, nil
	case errors.Is(err, alerts.ErrNotFound):
		return UpdatePriceAlert404JSONResponse{AlertNotFoundJSONResponse: AlertNotFoundJSONResponse(alertNotFound(ctx).body)}, nil
	case errors.Is(err, alerts.ErrDuplicate):
		return UpdatePriceAlert409JSONResponse{AlertConflictJSONResponse: AlertConflictJSONResponse(duplicateTarget(ctx).body)}, nil
	default:
		return UpdatePriceAlert500JSONResponse{api.internalError(ctx, "update_price_alert", err)}, nil
	}
}
func (api *api) DeletePriceAlert(ctx context.Context, request DeletePriceAlertRequestObject) (DeletePriceAlertResponseObject, error) {
	err := api.alerts.Delete(ctx, currentUserID(ctx), request.AlertId)
	if errors.Is(err, alerts.ErrNotFound) {
		return DeletePriceAlert404JSONResponse{AlertNotFoundJSONResponse: AlertNotFoundJSONResponse(alertNotFound(ctx).body)}, nil
	}
	if err != nil {
		return DeletePriceAlert500JSONResponse{api.internalError(ctx, "delete_price_alert", err)}, nil
	}
	return DeletePriceAlert204Response{}, nil
}
