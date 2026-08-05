package telegram

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"crypto-scanner/internal/auth"
)

type userContextKey struct{}

// Options exposes the time boundary used to validate init-data age.
type Options struct {
	Now func() time.Time
}

// Middleware authenticates Telegram Mini App init data and authorizes the
// Telegram identity against the application user store.
type Middleware struct {
	store    auth.UserStore
	botToken string
	maxAge   time.Duration
	now      func() time.Time
}

// New creates Telegram Mini App authentication middleware using the system clock.
func New(store auth.UserStore, botToken string, maxAge time.Duration) *Middleware {
	return NewWithOptions(store, botToken, maxAge, Options{})
}

// NewWithOptions creates middleware with explicit system-boundary options.
func NewWithOptions(store auth.UserStore, botToken string, maxAge time.Duration, options Options) *Middleware {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Middleware{store: store, botToken: botToken, maxAge: maxAge, now: now}
}

// Authenticate protects an HTTP handler with Telegram identity verification.
func (middleware *Middleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		rawInitData, ok := authorizationInitData(request.Header.Get("Authorization"))
		if !ok {
			writeError(response, http.StatusUnauthorized, "unauthenticated", "Telegram authentication is required")
			return
		}
		telegramID, ok := middleware.validate(rawInitData)
		if !ok {
			writeError(response, http.StatusUnauthorized, "unauthenticated", "Telegram authentication is invalid or expired")
			return
		}
		user, err := middleware.store.FindEnabledByTelegramID(request.Context(), telegramID)
		if errors.Is(err, auth.ErrUserNotFound) || err == nil && !user.Enabled {
			writeError(response, http.StatusForbidden, "access_denied", "Telegram user is not allowed")
			return
		}
		if err != nil {
			writeError(response, http.StatusInternalServerError, "internal_error", "Internal server error")
			return
		}
		ctx := context.WithValue(request.Context(), userContextKey{}, user)
		next.ServeHTTP(response, request.WithContext(ctx))
	})
}

// UserFromContext returns the enabled user attached by Authenticate.
func UserFromContext(ctx context.Context) (auth.User, bool) {
	user, ok := ctx.Value(userContextKey{}).(auth.User)
	return user, ok
}

func authorizationInitData(header string) (string, bool) {
	if !strings.HasPrefix(header, "tma ") {
		return "", false
	}
	raw := strings.TrimPrefix(header, "tma ")
	return raw, raw != "" && !strings.ContainsAny(raw, " \t\r\n")
}

func (middleware *Middleware) validate(raw string) (int64, bool) {
	values, err := url.ParseQuery(raw)
	if err != nil || !singleValues(values) {
		return 0, false
	}
	receivedHash, err := hex.DecodeString(values.Get("hash"))
	if err != nil || len(receivedHash) != sha256.Size {
		return 0, false
	}
	dataCheckString := makeDataCheckString(values)
	secretMAC := hmac.New(sha256.New, []byte("WebAppData"))
	_, _ = secretMAC.Write([]byte(middleware.botToken))
	dataMAC := hmac.New(sha256.New, secretMAC.Sum(nil))
	_, _ = dataMAC.Write([]byte(dataCheckString))
	if !hmac.Equal(receivedHash, dataMAC.Sum(nil)) {
		return 0, false
	}
	authUnix, err := strconv.ParseInt(values.Get("auth_date"), 10, 64)
	if err != nil {
		return 0, false
	}
	authTime := time.Unix(authUnix, 0)
	age := middleware.now().Sub(authTime)
	if age < 0 || age > middleware.maxAge {
		return 0, false
	}
	var telegramUser struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal([]byte(values.Get("user")), &telegramUser); err != nil || telegramUser.ID <= 0 {
		return 0, false
	}
	return telegramUser.ID, true
}

func singleValues(values url.Values) bool {
	if len(values) == 0 {
		return false
	}
	for key, items := range values {
		if key == "" || len(items) != 1 {
			return false
		}
	}
	return true
}

func makeDataCheckString(values url.Values) string {
	keys := make([]string, 0, len(values)-1)
	for key := range values {
		if key != "hash" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+values.Get(key))
	}
	return strings.Join(parts, "\n")
}

func writeError(response http.ResponseWriter, status int, code, message string) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	type errorBody struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	_ = json.NewEncoder(response).Encode(struct {
		Error     errorBody `json:"error"`
		RequestID string    `json:"request_id"`
	}{
		Error:     errorBody{Code: code, Message: message},
		RequestID: response.Header().Get("X-Request-ID"),
	})
}
