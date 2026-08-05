package telegram_test

import (
	"context"
	"strings"
	"testing"

	telegrambot "crypto-scanner/internal/bot/telegram"
)

func TestRegisterWebhookSuppliesPublicRouteAndSecret(t *testing.T) {
	registrar := &registrarStub{confirmed: true}

	if err := telegrambot.RegisterWebhook(context.Background(), registrar, "https://scanner.example", "webhook-secret"); err != nil {
		t.Fatalf("RegisterWebhook() error = %v", err)
	}
	if registrar.url != "https://scanner.example/telegram/webhook" || registrar.secret != "webhook-secret" {
		t.Fatalf("registration URL=%q secret=%q", registrar.url, registrar.secret)
	}
}

func TestRegisterWebhookFailsWhenTelegramDoesNotConfirm(t *testing.T) {
	registrar := &registrarStub{}

	err := telegrambot.RegisterWebhook(context.Background(), registrar, "https://scanner.example", "webhook-secret")
	if err == nil || !strings.Contains(err.Error(), "did not confirm") {
		t.Fatalf("RegisterWebhook() error = %v, want clear unconfirmed error", err)
	}
}

type registrarStub struct {
	url       string
	secret    string
	confirmed bool
}

func (registrar *registrarStub) SetWebhook(_ context.Context, url, secret string) (bool, error) {
	registrar.url = url
	registrar.secret = secret
	return registrar.confirmed, nil
}
