package telegram_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"crypto-scanner/internal/auth"
	"crypto-scanner/internal/auth/telegram"
)

const (
	fixtureBotToken = "123456789:AAExampleBotTokenForDeterministicTests"
	validInitData   = "auth_date=1785902400&query_id=AAHdF6IQAAAAAN0XogcAAAAA&user=%7B%22id%22%3A424242%2C%22first_name%22%3A%22Alice%22%2C%22username%22%3A%22alice%22%7D&hash=3787d0e46c1919cd293ec89f766ac33375446dbd7311acc07e422fecfc07812b"
)

var fixtureNow = time.Date(2026, time.August, 5, 4, 10, 0, 0, time.UTC)

func TestEnabledTelegramUserCanReachProtectedHandler(t *testing.T) {
	want := auth.User{ID: 7, TelegramID: 424242, Username: "alice", DisplayName: "Alice", Enabled: true}
	middleware := telegram.NewWithOptions(
		userStoreStub{find: func(_ context.Context, telegramID int64) (auth.User, error) {
			if telegramID != want.TelegramID {
				t.Fatalf("telegram ID = %d, want %d", telegramID, want.TelegramID)
			}
			return want, nil
		}},
		fixtureBotToken,
		15*time.Minute,
		telegram.Options{Now: func() time.Time { return fixtureNow }},
	)
	handler := middleware.Authenticate(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		got, ok := telegram.UserFromContext(request.Context())
		if !ok || got != want {
			t.Fatalf("authenticated user = %#v, %t; want %#v, true", got, ok, want)
		}
		response.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	request.Header.Set("Authorization", "tma "+validInitData)
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
		{name: "future auth date", authorization: "tma " + signedInitData(replacing(validValues, "auth_date", "1785903001"))},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			middleware := telegram.NewWithOptions(
				userStoreStub{find: func(context.Context, int64) (auth.User, error) {
					t.Fatal("user store called for unauthenticated request")
					return auth.User{}, nil
				}},
				fixtureBotToken,
				10*time.Minute,
				telegram.Options{Now: func() time.Time { return fixtureNow }},
			)
			reached := false
			handler := middleware.Authenticate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true }))
			request := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
			request.Header.Set("Authorization", test.authorization)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			assertErrorResponse(t, response, http.StatusUnauthorized, "unauthenticated")
			if reached {
				t.Fatal("protected handler was reached")
			}
			body := response.Body.String()
			if strings.Contains(body, fixtureBotToken) || strings.Contains(body, validInitData) ||
				test.authorization != "" && strings.Contains(body, test.authorization) {
				t.Fatalf("response exposed authentication material: %q", body)
			}
		})
	}
}

func TestTelegramUserMustBeEnabledInTheStore(t *testing.T) {
	tests := []struct {
		name       string
		storeReply auth.User
		storeError error
		wantStatus int
		wantCode   string
	}{
		{name: "unknown", storeError: auth.ErrUserNotFound, wantStatus: http.StatusForbidden, wantCode: "access_denied"},
		{name: "disabled", storeReply: auth.User{TelegramID: 424242}, wantStatus: http.StatusForbidden, wantCode: "access_denied"},
		{name: "store failure", storeError: errors.New("database unavailable: secret detail"), wantStatus: http.StatusInternalServerError, wantCode: "internal_error"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			middleware := telegram.NewWithOptions(
				userStoreStub{find: func(context.Context, int64) (auth.User, error) { return test.storeReply, test.storeError }},
				fixtureBotToken,
				15*time.Minute,
				telegram.Options{Now: func() time.Time { return fixtureNow }},
			)
			handler := middleware.Authenticate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("protected handler was reached")
			}))
			request := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
			request.Header.Set("Authorization", "tma "+validInitData)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			assertErrorResponse(t, response, test.wantStatus, test.wantCode)
			if strings.Contains(response.Body.String(), "secret detail") {
				t.Fatalf("response exposed store error: %q", response.Body.String())
			}
		})
	}
}

func TestAuthenticationErrorCarriesTheRequestIDWithoutExposingCredentials(t *testing.T) {
	middleware := telegram.New(userStoreStub{find: func(context.Context, int64) (auth.User, error) {
		t.Fatal("user store called for invalid signature")
		return auth.User{}, nil
	}}, fixtureBotToken, 15*time.Minute)
	handler := middleware.Authenticate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("protected handler was reached")
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	request.Header.Set("Authorization", "tma "+strings.Replace(validInitData, "alice", "mallory", 1))
	response := httptest.NewRecorder()
	response.Header().Set("X-Request-ID", "request-11")

	handler.ServeHTTP(response, request)

	var body struct {
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.RequestID != "request-11" {
		t.Errorf("request_id = %q, want request-11", body.RequestID)
	}
	if strings.Contains(response.Body.String(), validInitData) || strings.Contains(response.Body.String(), fixtureBotToken) {
		t.Fatalf("response exposed authentication material: %q", response.Body.String())
	}
}

type userStoreStub struct {
	find func(context.Context, int64) (auth.User, error)
}

func (stub userStoreStub) FindEnabledByTelegramID(ctx context.Context, telegramID int64) (auth.User, error) {
	return stub.find(ctx, telegramID)
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
