package httpapi

import (
	"context"
	"errors"
	"net/http"

	"crypto-scanner/internal/auth"
)

// APITokens issues, lists, and revokes the API tokens of a user. Create
// fails with auth.ErrInvalidAPITokenName for a blank name, Delete with
// auth.ErrAPITokenNotFound for tokens of other users.
type APITokens interface {
	CreateAPIToken(ctx context.Context, userID int64, name string) (auth.IssuedAPIToken, error)
	ListAPITokens(ctx context.Context, userID int64) ([]auth.APIToken, error)
	DeleteAPIToken(ctx context.Context, userID, id int64) error
}

func (api *api) ListApiTokens(ctx context.Context, _ ListApiTokensRequestObject) (ListApiTokensResponseObject, error) {
	tokens, err := api.apiTokens.ListAPITokens(ctx, currentUserID(ctx))
	if err != nil {
		return ListApiTokens500JSONResponse{api.internalError(ctx, "list_api_tokens", err)}, nil
	}
	body := ApiTokenList{Items: make([]ApiToken, len(tokens))}
	for i, token := range tokens {
		body.Items[i] = ApiToken{Id: token.ID, Name: token.Name, CreatedAt: token.CreatedAt}
		if !token.LastUsedAt.IsZero() {
			body.Items[i].LastUsedAt = &token.LastUsedAt
		}
	}
	return ListApiTokens200JSONResponse(body), nil
}

func (api *api) CreateApiToken(ctx context.Context, request CreateApiTokenRequestObject) (CreateApiTokenResponseObject, error) {
	issued, err := api.apiTokens.CreateAPIToken(ctx, currentUserID(ctx), request.Body.Name)
	switch {
	case err == nil:
		return CreateApiToken201JSONResponse{Id: issued.ID, Name: issued.Name, CreatedAt: issued.CreatedAt, Token: issued.Token}, nil
	case errors.Is(err, auth.ErrInvalidAPITokenName):
		return CreateApiToken400JSONResponse{invalidArgument(ctx, "API token name must not be blank").badRequest()}, nil
	default:
		return CreateApiToken500JSONResponse{api.internalError(ctx, "create_api_token", err)}, nil
	}
}

func (api *api) DeleteApiToken(ctx context.Context, request DeleteApiTokenRequestObject) (DeleteApiTokenResponseObject, error) {
	err := api.apiTokens.DeleteAPIToken(ctx, currentUserID(ctx), request.TokenId)
	switch {
	case err == nil:
		return DeleteApiToken204Response{}, nil
	case errors.Is(err, auth.ErrAPITokenNotFound):
		return DeleteApiToken404JSONResponse{ApiTokenNotFoundJSONResponse(newAPIError(ctx, http.StatusNotFound, "api_token_not_found", "API token does not exist", nil).body)}, nil
	default:
		return DeleteApiToken500JSONResponse{api.internalError(ctx, "delete_api_token", err)}, nil
	}
}
