package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"crypto-scanner/internal/auth"
	"crypto-scanner/internal/auth/telegram"
	"crypto-scanner/internal/httpapi"
	"crypto-scanner/internal/platform/logging"
)

const administratorTelegramID = 424242

func newAPITokenHandler(store *memorySessionStore) (http.Handler, *auth.Sessions) {
	logger := logging.New(io.Discard, "error", logging.Options{})
	sessions := auth.NewSessions(store, telegram.New(fixtureBotToken, 15*time.Minute, telegram.Options{Now: func() time.Time { return fixtureNow }}),
		administratorTelegramID, time.Hour, 24*time.Hour, logger, auth.SessionOptions{Now: func() time.Time { return fixtureNow }})
	return httpapi.New(logger, httpapi.Dependencies{Readiness: readinessStub{}, Sessions: sessions, APITokens: sessions}, httpapi.Options{}), sessions
}

func call(t *testing.T, handler http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestAPITokenAuthenticatesAsItsAdministratorUntilDeleted(t *testing.T) {
	administrator := auth.User{ID: 7, TelegramID: administratorTelegramID, Username: "alice"}
	store := &memorySessionStore{find: func(context.Context, int64) (auth.User, error) { return administrator, nil }}
	handler, _ := newAPITokenHandler(store)
	exchanged := exchange(handler, "tma "+validInitData)
	var session struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(exchanged.Body.Bytes(), &session); err != nil || session.Token == "" {
		t.Fatalf("exchange = %d %s", exchanged.Code, exchanged.Body.String())
	}

	created := call(t, handler, http.MethodPost, "/api/v1/admin/api-tokens", session.Token, `{"name":"laptop CLI"}`)
	var issued struct {
		ID    int64  `json:"id"`
		Name  string `json:"name"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &issued); err != nil || created.Code != http.StatusCreated || issued.Name != "laptop CLI" || !strings.HasPrefix(issued.Token, auth.APITokenPrefix) {
		t.Fatalf("create = %d %s", created.Code, created.Body.String())
	}

	me := call(t, handler, http.MethodGet, "/api/v1/me", issued.Token, "")
	if me.Code != http.StatusOK || !strings.Contains(me.Body.String(), `"administrator":true`) {
		t.Fatalf("me with API token = %d %s", me.Code, me.Body.String())
	}
	// Only a session manages API tokens, so a leaked token cannot issue
	// more or revoke others.
	assertErrorResponse(t, call(t, handler, http.MethodGet, "/api/v1/admin/api-tokens", issued.Token, ""), http.StatusForbidden, "session_required")
	assertErrorResponse(t, call(t, handler, http.MethodPost, "/api/v1/admin/api-tokens", issued.Token, `{"name":"more"}`), http.StatusForbidden, "session_required")
	assertErrorResponse(t, call(t, handler, http.MethodDelete, "/api/v1/admin/api-tokens/"+strconv.FormatInt(issued.ID, 10), issued.Token, ""), http.StatusForbidden, "session_required")
	listed := call(t, handler, http.MethodGet, "/api/v1/admin/api-tokens", session.Token, "")
	var list struct {
		Items []struct {
			ID         int64      `json:"id"`
			LastUsedAt *time.Time `json:"last_used_at"`
			Token      *string    `json:"token"`
		} `json:"items"`
	}
	if err := json.Unmarshal(listed.Body.Bytes(), &list); err != nil || listed.Code != http.StatusOK || len(list.Items) != 1 {
		t.Fatalf("list = %d %s", listed.Code, listed.Body.String())
	}
	if item := list.Items[0]; item.ID != issued.ID || item.Token != nil || item.LastUsedAt == nil || !item.LastUsedAt.Equal(fixtureNow) {
		t.Fatalf("listed token = %+v, want id %d last used at %s without the secret", item, issued.ID, fixtureNow)
	}
	// Only a person starts history loads.
	assertErrorResponse(t, call(t, handler, http.MethodPost, "/api/v1/admin/candle-history-loads", issued.Token, `{"symbols":["BTCUSDT"],"intervals":["1h"],"depth":20000}`), http.StatusForbidden, "session_required")
	if store.touches != 1 {
		t.Errorf("last use recorded %d times within a minute, want 1", store.touches)
	}

	path := "/api/v1/admin/api-tokens/" + strconv.FormatInt(issued.ID, 10)
	if deleted := call(t, handler, http.MethodDelete, path, session.Token, ""); deleted.Code != http.StatusNoContent {
		t.Fatalf("delete = %d %s", deleted.Code, deleted.Body.String())
	}
	assertErrorResponse(t, call(t, handler, http.MethodGet, "/api/v1/me", issued.Token, ""), http.StatusUnauthorized, "unauthenticated")
	assertErrorResponse(t, call(t, handler, http.MethodDelete, path, session.Token, ""), http.StatusNotFound, "api_token_not_found")
	assertErrorResponse(t, call(t, handler, http.MethodGet, "/api/v1/me", auth.APITokenPrefix+"malformed", ""), http.StatusUnauthorized, "unauthenticated")
	if me := call(t, handler, http.MethodGet, "/api/v1/me", session.Token, ""); me.Code != http.StatusOK {
		t.Fatalf("session after API token use = %d %s", me.Code, me.Body.String())
	}
}

func TestAPITokenOfAnotherUserIsNotAnAdministrator(t *testing.T) {
	user := auth.User{ID: 8, TelegramID: 515151, Username: "bob"}
	store := &memorySessionStore{users: map[int64]auth.User{user.ID: user}}
	handler, sessions := newAPITokenHandler(store)
	issued, err := sessions.CreateAPIToken(context.Background(), user.ID, "bot")
	if err != nil {
		t.Fatalf("CreateAPIToken() error = %v", err)
	}

	if me := call(t, handler, http.MethodGet, "/api/v1/me", issued.Token, ""); me.Code != http.StatusOK || !strings.Contains(me.Body.String(), `"administrator":false`) {
		t.Fatalf("me = %d %s", me.Code, me.Body.String())
	}
	assertErrorResponse(t, call(t, handler, http.MethodGet, "/api/v1/admin/api-tokens", issued.Token, ""), http.StatusForbidden, "administrator_required")
}

// Recording the last use is informational: a failed write never refuses a
// valid token.
func TestAPITokenAuthenticatesWhenItsLastUseCannotBeRecorded(t *testing.T) {
	administrator := auth.User{ID: 7, TelegramID: administratorTelegramID}
	store := &memorySessionStore{users: map[int64]auth.User{administrator.ID: administrator}, touchErr: errors.New("database is read-only")}
	handler, sessions := newAPITokenHandler(store)
	issued, err := sessions.CreateAPIToken(context.Background(), administrator.ID, "cli")
	if err != nil {
		t.Fatal(err)
	}

	if me := call(t, handler, http.MethodGet, "/api/v1/me", issued.Token, ""); me.Code != http.StatusOK {
		t.Fatalf("me = %d %s", me.Code, me.Body.String())
	}
}

func TestCreateAPITokenTrimsTheNameAndRejectsABlankOne(t *testing.T) {
	administrator := auth.User{ID: 7, TelegramID: administratorTelegramID}
	store := &memorySessionStore{find: func(context.Context, int64) (auth.User, error) { return administrator, nil }}
	handler, sessions := newAPITokenHandler(store)
	var session struct {
		Token string `json:"token"`
	}
	if exchanged := exchange(handler, "tma "+validInitData); json.Unmarshal(exchanged.Body.Bytes(), &session) != nil || session.Token == "" {
		t.Fatalf("exchange = %d %s", exchanged.Code, exchanged.Body.String())
	}
	issued, err := sessions.CreateAPIToken(context.Background(), administrator.ID, " cli\t")
	if err != nil || issued.Name != "cli" {
		t.Fatalf("CreateAPIToken() = %q, %v; want the trimmed name", issued.Name, err)
	}

	// An ideographic space is blank to the service although the contract's
	// length bounds accept it.
	assertErrorResponse(t, call(t, handler, http.MethodPost, "/api/v1/admin/api-tokens", session.Token, `{"name":"\u3000"}`), http.StatusBadRequest, "invalid_argument")
	if tokens, _ := store.ListAPITokens(context.Background(), administrator.ID); len(tokens) != 1 || tokens[0].Name != "cli" {
		t.Fatalf("stored tokens = %+v, want only the trimmed first one", tokens)
	}
}
