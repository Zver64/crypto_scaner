package httpapi_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"crypto-scanner/internal/auth"
	"crypto-scanner/internal/auth/telegram"
	"crypto-scanner/internal/httpapi"
	"crypto-scanner/internal/platform/logging"
)

const (
	validInitData = "auth_date=1785902400&query_id=AAHdF6IQAAAAAN0XogcAAAAA&user=%7B%22id%22%3A424242%2C%22first_name%22%3A%22Alice%22%2C%22username%22%3A%22alice%22%7D&hash=3787d0e46c1919cd293ec89f766ac33375446dbd7311acc07e422fecfc07812b"
)

var fixtureNow = time.Date(2026, time.August, 5, 4, 10, 0, 0, time.UTC)

func newTestSessions(store *memorySessionStore, maxAge time.Duration) *auth.Sessions {
	return auth.NewSessions(store, telegram.New(fixtureBotToken, maxAge, telegram.Options{Now: func() time.Time { return fixtureNow }}),
		0, time.Hour, 24*time.Hour, logging.New(io.Discard, "error", logging.Options{}), auth.SessionOptions{Now: func() time.Time { return fixtureNow }})
}

func newSessionHandler(sessions httpapi.Sessions) http.Handler {
	return httpapi.New(logging.New(io.Discard, "error", logging.Options{}), httpapi.Dependencies{Readiness: readinessStub{}, Sessions: sessions}, httpapi.Options{})
}

func exchange(handler http.Handler, authorization string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/session", nil)
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestExchangedSessionReachesProtectedHandler(t *testing.T) {
	want := auth.User{ID: 7, TelegramID: 424242, Username: "alice", DisplayName: "Alice"}
	store := &memorySessionStore{find: func(_ context.Context, telegramID int64) (auth.User, error) {
		if telegramID != want.TelegramID {
			t.Fatalf("telegram ID = %d, want %d", telegramID, want.TelegramID)
		}
		return want, nil
	}}
	sessions := newTestSessions(store, 15*time.Minute)

	exchanged := exchange(newSessionHandler(sessions), "tma "+validInitData)
	if exchanged.Code != http.StatusCreated {
		t.Fatalf("exchange status = %d, body = %s", exchanged.Code, exchanged.Body.String())
	}
	var session struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(exchanged.Body.Bytes(), &session); err != nil || session.Token == "" {
		t.Fatalf("decode session: %v, %q", err, exchanged.Body.String())
	}
	for hash := range store.sessions {
		if strings.Contains(hash, session.Token) {
			t.Fatal("session store holds the raw token")
		}
	}

	handler := httpapi.RequireSession(sessions, logging.New(io.Discard, "error", logging.Options{}))(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		got, ok := httpapi.UserFromContext(request.Context())
		if !ok || got != want {
			t.Fatalf("authenticated user = %#v, %t; want %#v, true", got, ok, want)
		}
		response.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	request.Header.Set("Authorization", "Bearer "+session.Token)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", response.Code)
	}
}

func TestInvalidTelegramCredentialsAreUnauthenticated(t *testing.T) {
	validValues := url.Values{
		"auth_date": {"1785902400"},
		"query_id":  {"AAHdF6IQAAAAAN0XogcAAAAA"},
		"user":      {`{"id":424242,"first_name":"Alice","username":"alice"}`},
	}
	tests := []struct {
		name          string
		authorization string
	}{
		{name: "missing header"},
		{name: "wrong scheme", authorization: "Bearer " + validInitData},
		{name: "scheme is case sensitive", authorization: "TMA " + validInitData},
		{name: "scheme has one separator", authorization: "tma  " + validInitData},
		{name: "tampered data", authorization: "tma " + strings.Replace(validInitData, "424242", "424243", 1)},
		{name: "missing user", authorization: "tma " + signedInitData(without(validValues, "user"))},
		{name: "malformed user", authorization: "tma " + signedInitData(replacing(validValues, "user", "not-json"))},
		{name: "missing auth date", authorization: "tma " + signedInitData(without(validValues, "auth_date"))},
		{name: "malformed auth date", authorization: "tma " + signedInitData(replacing(validValues, "auth_date", "yesterday"))},
		{name: "expired auth date", authorization: "tma " + signedInitData(replacing(validValues, "auth_date", "1785902399"))},
		{name: "future auth date", authorization: "tma " + signedInitData(replacing(validValues, "auth_date", "1785903031"))},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &memorySessionStore{find: func(context.Context, int64) (auth.User, error) {
				t.Fatal("user store called for unauthenticated request")
				return auth.User{}, nil
			}}

			response := exchange(newSessionHandler(newTestSessions(store, 10*time.Minute)), test.authorization)

			assertErrorResponse(t, response, http.StatusUnauthorized, "unauthenticated")
			if len(store.sessions) != 0 {
				t.Fatal("session was created")
			}
			body := response.Body.String()
			if strings.Contains(body, fixtureBotToken) || strings.Contains(body, validInitData) ||
				test.authorization != "" && strings.Contains(body, test.authorization) {
				t.Fatalf("response exposed authentication material: %q", body)
			}
		})
	}
}

func TestTelegramUserMustExistInTheStore(t *testing.T) {
	tests := []struct {
		name       string
		storeReply auth.User
		storeError error
		wantStatus int
		wantCode   string
	}{
		{name: "unknown", storeError: auth.ErrUserNotFound, wantStatus: http.StatusForbidden, wantCode: "access_denied"},
		{name: "store failure", storeError: errors.New("database unavailable: secret detail"), wantStatus: http.StatusInternalServerError, wantCode: "internal_error"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &memorySessionStore{find: func(context.Context, int64) (auth.User, error) { return test.storeReply, test.storeError }}

			response := exchange(newSessionHandler(newTestSessions(store, 15*time.Minute)), "tma "+validInitData)

			assertErrorResponse(t, response, test.wantStatus, test.wantCode)
			if strings.Contains(response.Body.String(), "secret detail") {
				t.Fatalf("response exposed store error: %q", response.Body.String())
			}
		})
	}
}

func TestAuthenticationErrorCarriesTheRequestIDWithoutExposingCredentials(t *testing.T) {
	store := &memorySessionStore{find: func(context.Context, int64) (auth.User, error) {
		t.Fatal("user store called for an unknown session")
		return auth.User{}, nil
	}}
	middleware := httpapi.RequireSession(newTestSessions(store, 15*time.Minute), logging.New(io.Discard, "error", logging.Options{}))
	handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("protected handler was reached")
	}))
	const unknownToken = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	request := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	request.Header.Set("Authorization", "Bearer "+unknownToken)
	response := httptest.NewRecorder()
	response.Header().Set("X-Request-ID", "request-11")

	handler.ServeHTTP(response, request)

	var body struct {
		RequestID string `json:"request_id"`
		Error     struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Code != http.StatusUnauthorized || body.Error.Code != "unauthenticated" {
		t.Errorf("status = %d, code = %q; want 401 unauthenticated", response.Code, body.Error.Code)
	}
	if body.RequestID != "request-11" {
		t.Errorf("request_id = %q, want request-11", body.RequestID)
	}
	if strings.Contains(response.Body.String(), unknownToken) {
		t.Fatalf("response exposed authentication material: %q", response.Body.String())
	}
}

// memorySessionStore keeps sessions and API tokens in memory, keyed by token
// hash.
type memorySessionStore struct {
	find      func(context.Context, int64) (auth.User, error)
	mu        sync.Mutex
	sessions  map[string]auth.NewSession
	users     map[int64]auth.User
	apiTokens map[string]memoryAPIToken
	lastID    int64
	touches   int
	// touchErr fails recording the last use of API tokens.
	touchErr error
}

type memoryAPIToken struct {
	auth.APIToken
	userID int64
}

func (store *memorySessionStore) FindByTelegramID(ctx context.Context, telegramID int64) (auth.User, error) {
	user, err := store.find(ctx, telegramID)
	if err == nil {
		store.mu.Lock()
		defer store.mu.Unlock()
		if store.users == nil {
			store.users = make(map[int64]auth.User)
		}
		store.users[user.ID] = user
	}
	return user, err
}

func (store *memorySessionStore) CreateSession(_ context.Context, session auth.NewSession) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.sessions == nil {
		store.sessions = make(map[string]auth.NewSession)
	}
	store.sessions[string(session.TokenHash)] = session
	return nil
}

func (store *memorySessionStore) FindSession(_ context.Context, tokenHash []byte) (auth.Session, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	session, ok := store.sessions[string(tokenHash)]
	if !ok {
		return auth.Session{}, auth.ErrSessionNotFound
	}
	return auth.Session{User: store.users[session.UserID], IdleExpiresAt: session.IdleExpiresAt, AbsoluteExpiresAt: session.AbsoluteExpiresAt}, nil
}

func (store *memorySessionStore) ExtendSession(_ context.Context, tokenHash []byte, idleExpiresAt time.Time, minimumExtension time.Duration) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if session, ok := store.sessions[string(tokenHash)]; ok && !session.IdleExpiresAt.After(idleExpiresAt.Add(-minimumExtension)) {
		session.IdleExpiresAt = idleExpiresAt
		store.sessions[string(tokenHash)] = session
	}
	return nil
}

func (store *memorySessionStore) DeleteSession(_ context.Context, tokenHash []byte) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	delete(store.sessions, string(tokenHash))
	return nil
}

func (store *memorySessionStore) DeleteExpiredSessions(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func (store *memorySessionStore) CreateAPIToken(_ context.Context, token auth.NewAPIToken) (int64, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.apiTokens == nil {
		store.apiTokens = make(map[string]memoryAPIToken)
	}
	store.lastID++
	store.apiTokens[string(token.TokenHash)] = memoryAPIToken{APIToken: auth.APIToken{ID: store.lastID, Name: token.Name, CreatedAt: token.CreatedAt}, userID: token.UserID}
	return store.lastID, nil
}

func (store *memorySessionStore) ListAPITokens(_ context.Context, userID int64) ([]auth.APIToken, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	var tokens []auth.APIToken
	for _, token := range store.apiTokens {
		if token.userID == userID {
			tokens = append(tokens, token.APIToken)
		}
	}
	return tokens, nil
}

func (store *memorySessionStore) DeleteAPIToken(_ context.Context, userID, id int64) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	for hash, token := range store.apiTokens {
		if token.ID == id && token.userID == userID {
			delete(store.apiTokens, hash)
			return nil
		}
	}
	return auth.ErrAPITokenNotFound
}

func (store *memorySessionStore) FindAPIToken(_ context.Context, tokenHash []byte) (auth.StoredAPIToken, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	token, ok := store.apiTokens[string(tokenHash)]
	if !ok {
		return auth.StoredAPIToken{}, auth.ErrAPITokenNotFound
	}
	return auth.StoredAPIToken{ID: token.ID, User: store.users[token.userID], LastUsedAt: token.LastUsedAt}, nil
}

func (store *memorySessionStore) TouchAPIToken(_ context.Context, id int64, usedAt time.Time, minimumInterval time.Duration) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.touchErr != nil {
		return store.touchErr
	}
	for hash, token := range store.apiTokens {
		if token.ID == id && !token.LastUsedAt.After(usedAt.Add(-minimumInterval)) {
			token.LastUsedAt = usedAt
			store.apiTokens[hash] = token
			store.touches++
		}
	}
	return nil
}

func assertErrorResponse(t *testing.T, response *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()
	if response.Code != wantStatus {
		t.Errorf("status = %d, want %d", response.Code, wantStatus)
	}
	if response.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", response.Header().Get("Content-Type"))
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != wantCode {
		t.Errorf("error code = %q, want %q", body.Error.Code, wantCode)
	}
}

func signedInitData(values url.Values) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+values.Get(key))
	}
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	_, _ = secret.Write([]byte(fixtureBotToken))
	signature := hmac.New(sha256.New, secret.Sum(nil))
	_, _ = signature.Write([]byte(strings.Join(parts, "\n")))
	result := replacing(values, "hash", hex.EncodeToString(signature.Sum(nil)))
	return result.Encode()
}

func replacing(values url.Values, key, value string) url.Values {
	result := cloneValues(values)
	result.Set(key, value)
	return result
}

func without(values url.Values, key string) url.Values {
	result := cloneValues(values)
	result.Del(key)
	return result
}

func cloneValues(values url.Values) url.Values {
	result := make(url.Values, len(values))
	for key, items := range values {
		result[key] = append([]string(nil), items...)
	}
	return result
}
