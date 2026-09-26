package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"crypto-scanner/internal/auth"
)

// InitDataAuthenticator verifies Telegram Mini App init data. It fails with
// auth.ErrUnauthenticated or auth.ErrAccessDenied.
type InitDataAuthenticator interface {
	AuthenticateInitData(context.Context, string) (auth.User, error)
}

type userContextKey struct{}

// UserFromContext returns the enabled user attached by the authentication middleware.
func UserFromContext(ctx context.Context) (auth.User, bool) {
	user, ok := ctx.Value(userContextKey{}).(auth.User)
	return user, ok
}

// Authentication failure messages shared by HTTP and WebSocket clients.
const authenticationRequiredMessage = "Telegram authentication is required"

// requireTelegramUser protects handlers with "Authorization: tma <init data>".
func requireTelegramUser(authenticator InitDataAuthenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			rawInitData, ok := authorizationInitData(request.Header.Get("Authorization"))
			if !ok {
				writeAPIError(response, http.StatusUnauthorized, "unauthenticated", authenticationRequiredMessage, nil)
				return
			}
			user, err := authenticator.AuthenticateInitData(request.Context(), rawInitData)
			switch {
			case errors.Is(err, auth.ErrUnauthenticated):
				writeAPIError(response, http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error(), nil)
				return
			case errors.Is(err, auth.ErrAccessDenied):
				writeAPIError(response, http.StatusForbidden, "access_denied", auth.ErrAccessDenied.Error(), nil)
				return
			case err != nil:
				writeAPIError(response, http.StatusInternalServerError, "internal_error", "Internal server error", nil)
				return
			}
			next.ServeHTTP(response, request.WithContext(context.WithValue(request.Context(), userContextKey{}, user)))
		})
	}
}

func authorizationInitData(header string) (string, bool) {
	raw, ok := strings.CutPrefix(header, "tma ")
	return raw, ok && raw != "" && !strings.ContainsAny(raw, " \t\r\n")
}
