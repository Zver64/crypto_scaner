package httpapi

import (
	"context"
	"errors"
	"net/http"

	"crypto-scanner/internal/auth"
	"crypto-scanner/internal/users"
)

// Users lists, updates, and deletes application users for the administrator.
type Users interface {
	List(context.Context) ([]users.User, error)
	Delete(context.Context, int64) error
	SetStrategyAlerts(ctx context.Context, telegramID int64, enabled bool) error
}

func (api *api) ListUsers(ctx context.Context, _ ListUsersRequestObject) (ListUsersResponseObject, error) {
	items, err := api.users.List(ctx)
	if err != nil {
		return ListUsers500JSONResponse{api.internalError(ctx, "list_users", err)}, nil
	}
	body := UserList{Items: make([]User, len(items))}
	for i, user := range items {
		body.Items[i] = User{TelegramId: user.TelegramID, Username: optionalString(user.Username), DisplayName: optionalString(user.DisplayName), Administrator: user.Administrator, StrategyAlerts: user.StrategyAlerts}
	}
	return ListUsers200JSONResponse(body), nil
}

func (api *api) DeleteUser(ctx context.Context, request DeleteUserRequestObject) (DeleteUserResponseObject, error) {
	err := api.users.Delete(ctx, request.TelegramId)
	switch {
	case err == nil:
		return DeleteUser204Response{}, nil
	case errors.Is(err, auth.ErrUserNotFound):
		return DeleteUser404JSONResponse{UserNotFoundJSONResponse(newAPIError(ctx, http.StatusNotFound, "user_not_found", "User does not exist", nil).body)}, nil
	case errors.Is(err, users.ErrAdministratorProtected):
		return DeleteUser409JSONResponse{AdministratorProtectedJSONResponse(newAPIError(ctx, http.StatusConflict, "administrator_protected", "The administrator cannot be deleted", nil).body)}, nil
	default:
		return DeleteUser500JSONResponse{api.internalError(ctx, "delete_user", err)}, nil
	}
}

func (api *api) UpdateUser(ctx context.Context, request UpdateUserRequestObject) (UpdateUserResponseObject, error) {
	err := api.users.SetStrategyAlerts(ctx, request.TelegramId, request.Body.StrategyAlerts)
	switch {
	case err == nil:
		return UpdateUser204Response{}, nil
	case errors.Is(err, auth.ErrUserNotFound):
		return UpdateUser404JSONResponse{UserNotFoundJSONResponse(newAPIError(ctx, http.StatusNotFound, "user_not_found", "User does not exist", nil).body)}, nil
	case errors.Is(err, users.ErrAdministratorProtected):
		return UpdateUser409JSONResponse{AdministratorProtectedJSONResponse(newAPIError(ctx, http.StatusConflict, "administrator_protected", "The administrator always receives strategy alerts", nil).body)}, nil
	default:
		return UpdateUser500JSONResponse{api.internalError(ctx, "update_user", err)}, nil
	}
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
