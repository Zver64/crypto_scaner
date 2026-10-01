package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"crypto-scanner/internal/auth"
)

// Sessions exchanges Telegram init data for session tokens and authenticates
// HTTP and WebSocket requests by them. Exchange fails with
// auth.ErrUnauthenticated or auth.ErrAccessDenied, Authenticate with
// auth.ErrUnauthenticated.
type Sessions interface {
	Exchange(ctx context.Context, rawInitData string) (auth.IssuedSession, error)
	Authenticate(ctx context.Context, token string) (auth.User, error)
	Revoke(ctx context.Context, token string) error
}

type userContextKey struct{}

// credentialContextKey holds the presented credential: the init data of a
// session exchange, or the session token of an authenticated request.
type credentialContextKey struct{}

// UserFromContext returns the application user attached by the authentication middleware.
func UserFromContext(ctx context.Context) (auth.User, bool) {
	user, ok := ctx.Value(userContextKey{}).(auth.User)
	return user, ok
}

func credentialFromContext(ctx context.Context) string {
	credential, _ := ctx.Value(credentialContextKey{}).(string)
	return credential
}

// Authentication failure messages shared by HTTP and WebSocket clients.
const (
	authenticationRequiredMessage = "Authentication is required"
	sessionInvalidMessage         = "Session is invalid or expired"
)

// requireSession protects handlers with "Authorization: Bearer <session token>".
func requireSession(sessions Sessions, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			token, ok := authorizationCredential(request.Header.Get("Authorization"), "Bearer ")
			if !ok {
				writeAPIError(response, http.StatusUnauthorized, "unauthenticated", authenticationRequiredMessage, nil)
				return
			}
			user, err := sessions.Authenticate(request.Context(), token)
			switch {
			case errors.Is(err, auth.ErrUnauthenticated):
				writeAPIError(response, http.StatusUnauthorized, "unauthenticated", sessionInvalidMessage, nil)
				return
			case err != nil:
				logger.ErrorContext(request.Context(), "session authentication failed", "module", "httpapi", "operation", "authenticate",
					"request_id", RequestIdentifier(request.Context()), "error", err)
				writeAPIError(response, http.StatusInternalServerError, "internal_error", "Internal server error", nil)
				return
			}
			ctx := context.WithValue(request.Context(), userContextKey{}, user)
			ctx = context.WithValue(ctx, credentialContextKey{}, token)
			next.ServeHTTP(response, request.WithContext(ctx))
		})
	}
}

// requireInitData passes "Authorization: tma <init data>" to the session
// exchange, which verifies it.
func requireInitData(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		initData, ok := authorizationCredential(request.Header.Get("Authorization"), "tma ")
		if !ok {
			writeAPIError(response, http.StatusUnauthorized, "unauthenticated", authenticationRequiredMessage, nil)
			return
		}
		next.ServeHTTP(response, request.WithContext(context.WithValue(request.Context(), credentialContextKey{}, initData)))
	})
}

// requireAdministrator rejects authenticated users other than the scanner
// administrator. It runs after requireSession.
func requireAdministrator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if user, ok := UserFromContext(request.Context()); !ok || !user.Administrator {
			writeAPIError(response, http.StatusForbidden, "administrator_required", "Only the scanner administrator can change scanner settings", nil)
			return
		}
		next.ServeHTTP(response, request)
	})
}

func authorizationCredential(header, scheme string) (string, bool) {
	raw, ok := strings.CutPrefix(header, scheme)
	return raw, ok && raw != "" && !strings.ContainsAny(raw, " \t\r\n")
}

func (api *api) CreateSession(ctx context.Context, _ CreateSessionRequestObject) (CreateSessionResponseObject, error) {
	issued, err := api.sessions.Exchange(ctx, credentialFromContext(ctx))
	switch {
	case err == nil:
		return CreateSession201JSONResponse{
			Body:    Session{Token: issued.Token, ExpiresAt: issued.ExpiresAt},
			Headers: CreateSession201ResponseHeaders{XRequestID: RequestIdentifier(ctx)},
		}, nil
	case errors.Is(err, auth.ErrUnauthenticated):
		body := newAPIError(ctx, http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error(), nil).body
		return CreateSession401JSONResponse{UnauthenticatedJSONResponse{Body: body, Headers: UnauthenticatedResponseHeaders{XRequestID: body.RequestId}}}, nil
	case errors.Is(err, auth.ErrAccessDenied):
		body := newAPIError(ctx, http.StatusForbidden, "access_denied", auth.ErrAccessDenied.Error(), nil).body
		return CreateSession403JSONResponse{AccessDeniedJSONResponse{Body: body, Headers: AccessDeniedResponseHeaders{XRequestID: body.RequestId}}}, nil
	default:
		return CreateSession500JSONResponse{api.internalError(ctx, "create_session", err)}, nil
	}
}

func (api *api) DeleteSession(ctx context.Context, _ DeleteSessionRequestObject) (DeleteSessionResponseObject, error) {
	if err := api.sessions.Revoke(ctx, credentialFromContext(ctx)); err != nil {
		return DeleteSession500JSONResponse{api.internalError(ctx, "delete_session", err)}, nil
	}
	return DeleteSession204Response{}, nil
}
