package telegram_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	telegrambot "crypto-scanner/internal/bot/telegram"
)

func TestWebhookRejectsWrongSecretBeforeReadingBody(t *testing.T) {
	body := &readProbe{Reader: strings.NewReader(`{"message":`)}
	client := &outboundStub{}
	handler := newWebhook(client)
	request := httptest.NewRequest(http.MethodPost, "/telegram/webhook", body)
	request.Header.Set("X-Telegram-Bot-Api-Secret-Token", "wrong-secret")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
	if body.read {
		t.Fatal("request body was read before the secret was accepted")
	}
	if client.calls != 0 {
		t.Fatalf("outbound calls = %d, want 0", client.calls)
	}
}

func TestWebhookDecodesExactlyOneUpdate(t *testing.T) {
	client := &outboundStub{}
	handler := newWebhook(client)
	request := httptest.NewRequest(http.MethodPost, "/telegram/webhook", strings.NewReader(`{"update_id":1}{"update_id":2}`))
	request.Header.Set("X-Telegram-Bot-Api-Secret-Token", "correct-secret")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
	if client.calls != 0 {
		t.Fatalf("outbound calls = %d, want 0", client.calls)
	}
}

func TestWebhookBoundsUpdateBody(t *testing.T) {
	handler := newWebhook(&outboundStub{})
	request := httptest.NewRequest(http.MethodPost, "/telegram/webhook", strings.NewReader(`{"padding":"`+strings.Repeat("x", 1<<20)+`"}`))
	request.Header.Set("X-Telegram-Bot-Api-Secret-Token", "correct-secret")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", response.Code)
	}
}

type readProbe struct {
	*strings.Reader
	read bool
}

func (reader *readProbe) Read(buffer []byte) (int, error) {
	reader.read = true
	return reader.Reader.Read(buffer)
}

type outboundStub struct {
	calls      int
	chatID     int64
	miniAppURL string
}

func (client *outboundStub) SendMiniAppLaunch(_ context.Context, chatID int64, miniAppURL string) error {
	client.calls++
	client.chatID = chatID
	client.miniAppURL = miniAppURL
	return nil
}

func newWebhook(client *outboundStub) http.Handler {
	processor := telegrambot.NewUpdateHandler("https://scanner.example/app", client)
	return telegrambot.NewWebhookHandler("correct-secret", processor)
}

func TestStartSendsConfiguredMiniAppLaunch(t *testing.T) {
	client := &outboundStub{}
	handler := newWebhook(client)
	request := httptest.NewRequest(http.MethodPost, "/telegram/webhook", strings.NewReader(`{
		"update_id": 42,
		"message": {"text": "/start", "chat": {"id": 987654321}}
	}`))
	request.Header.Set("X-Telegram-Bot-Api-Secret-Token", "correct-secret")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if client.calls != 1 || client.chatID != 987654321 || client.miniAppURL != "https://scanner.example/app" {
		t.Fatalf("outbound call = count:%d chat:%d URL:%q", client.calls, client.chatID, client.miniAppURL)
	}
}

func TestUnknownUpdateIsAcknowledgedWithoutSideEffects(t *testing.T) {
	client := &outboundStub{}
	handler := newWebhook(client)
	request := httptest.NewRequest(http.MethodPost, "/telegram/webhook", strings.NewReader(`{"update_id":43,"callback_query":{"id":"unknown"}}`))
	request.Header.Set("X-Telegram-Bot-Api-Secret-Token", "correct-secret")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if client.calls != 0 {
		t.Fatalf("outbound calls = %d, want 0", client.calls)
	}
}
