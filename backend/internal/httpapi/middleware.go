package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type requestIDContextKey struct{}

// requestInfoContextKey holds the requestInfo of the current HTTP request.
type requestInfoContextKey struct{}

type requestInfo struct {
	method  string
	path    string
	started time.Time
}

// RequestIdentifier returns the correlation identifier installed by the HTTP
// middleware, or an empty string outside an HTTP request.
func RequestIdentifier(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)
	return requestID
}

// requestAttributes describe the current request in failure logs: its
// identifier, method, path, elapsed time, and user once authenticated.
func requestAttributes(ctx context.Context) []any {
	attributes := []any{"request_id", RequestIdentifier(ctx)}
	if info, ok := ctx.Value(requestInfoContextKey{}).(requestInfo); ok {
		attributes = append(attributes, "method", info.method, "path", info.path, "duration", time.Since(info.started).Round(time.Millisecond))
	}
	if user, ok := UserFromContext(ctx); ok {
		attributes = append(attributes, "telegram_id", user.TelegramID)
	}
	return attributes
}

// clientGone reports that err only reflects the client abandoning the
// request, such as a closed Mini App, rather than a server failure.
func clientGone(ctx context.Context, err error) bool {
	return errors.Is(err, context.Canceled) && errors.Is(ctx.Err(), context.Canceled)
}

func requestMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		started := time.Now()
		requestID := request.Header.Get("X-Request-ID")
		if !validRequestID(requestID) {
			requestID = newRequestID()
		}

		response.Header().Set("X-Request-ID", requestID)
		recorder := &statusRecorder{ResponseWriter: response, status: http.StatusOK}
		ctx := context.WithValue(request.Context(), requestIDContextKey{}, requestID)
		ctx = context.WithValue(ctx, requestInfoContextKey{}, requestInfo{method: request.Method, path: request.URL.Path, started: started})
		request = request.WithContext(ctx)
		next.ServeHTTP(recorder, request)

		logger.InfoContext(request.Context(), "HTTP request completed",
			"request_id", requestID,
			"module", "httpapi",
			"operation", "request",
			"duration", time.Since(started),
			"outcome", outcome(recorder.status),
			"method", request.Method,
			"path", request.URL.Path,
			"status", recorder.status,
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.wroteHeader {
		return
	}
	r.status = status
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(body []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(body)
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func validRequestID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	return strings.IndexFunc(value, func(character rune) bool {
		return !(character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			strings.ContainsRune("-_.:", character))
	}) == -1
}

func newRequestID() string {
	var random [16]byte
	_, _ = rand.Read(random[:]) // never fails since Go 1.24
	return hex.EncodeToString(random[:])
}

func outcome(status int) string {
	switch {
	case status >= http.StatusInternalServerError:
		return "server_error"
	case status >= http.StatusBadRequest:
		return "client_error"
	default:
		return "success"
	}
}
