package telegram

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	telegramapi "github.com/go-telegram/bot"
)

func TestClientSendsMiniAppLaunchButton(t *testing.T) {
	var gotChatID, gotText, gotMarkup string
	doer := httpDoerFunc(func(request *http.Request) (*http.Response, error) {
		if err := request.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("parse Telegram request: %v", err)
		}
		gotChatID = request.FormValue("chat_id")
		gotText = request.FormValue("text")
		gotMarkup = request.FormValue("reply_markup")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"ok":true,"result":{"message_id":1,"date":1,"chat":{"id":77,"type":"private"}}}`)),
		}, nil
	})
	bot, err := telegramapi.New("123456:test-token", telegramapi.WithHTTPClient(time.Second, doer), telegramapi.WithSkipGetMe())
	if err != nil {
		t.Fatalf("create Telegram SDK client: %v", err)
	}
	client := newClient(bot)

	if err := client.SendMiniAppLaunch(context.Background(), 77, "https://scanner.example/app"); err != nil {
		t.Fatalf("SendMiniAppLaunch() error = %v", err)
	}

	if gotChatID != "77" || gotText == "" {
		t.Fatalf("Telegram message chat_id=%q text=%q", gotChatID, gotText)
	}
	var markup struct {
		InlineKeyboard [][]struct {
			WebApp *struct {
				URL string `json:"url"`
			} `json:"web_app"`
		} `json:"inline_keyboard"`
	}
	if err := json.Unmarshal([]byte(gotMarkup), &markup); err != nil {
		t.Fatalf("decode reply markup %q: %v", gotMarkup, err)
	}
	if len(markup.InlineKeyboard) != 1 || len(markup.InlineKeyboard[0]) != 1 || markup.InlineKeyboard[0][0].WebApp == nil || markup.InlineKeyboard[0][0].WebApp.URL != "https://scanner.example/app" {
		t.Fatalf("reply markup lacks configured Mini App button: %#v", markup)
	}
}

type httpDoerFunc func(*http.Request) (*http.Response, error)

func (do httpDoerFunc) Do(request *http.Request) (*http.Response, error) { return do(request) }

func TestClientRegistersWebhookURLAndSecret(t *testing.T) {
	var gotURL, gotSecret string
	doer := httpDoerFunc(func(request *http.Request) (*http.Response, error) {
		if err := request.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("parse Telegram request: %v", err)
		}
		gotURL = request.FormValue("url")
		gotSecret = request.FormValue("secret_token")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"ok":true,"result":true}`)),
		}, nil
	})
	bot, err := telegramapi.New("123456:test-token", telegramapi.WithHTTPClient(time.Second, doer), telegramapi.WithSkipGetMe())
	if err != nil {
		t.Fatalf("create Telegram SDK client: %v", err)
	}

	confirmed, err := newClient(bot).SetWebhook(context.Background(), "https://scanner.example/telegram/webhook", "webhook-secret")
	if err != nil {
		t.Fatalf("SetWebhook() error = %v", err)
	}
	if !confirmed || gotURL != "https://scanner.example/telegram/webhook" || gotSecret != "webhook-secret" {
		t.Fatalf("confirmed=%t URL=%q secret=%q", confirmed, gotURL, gotSecret)
	}
}
