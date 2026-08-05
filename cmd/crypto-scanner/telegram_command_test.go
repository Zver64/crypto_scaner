package main

import (
	"context"
	"testing"

	telegrambot "crypto-scanner/internal/bot/telegram"
	"crypto-scanner/internal/platform/config"
)

func TestTelegramSetWebhookCommandRegistersConfiguredEndpoint(t *testing.T) {
	registrar := &commandRegistrar{confirmed: true}
	load := func() (config.TelegramWebhookConfig, error) {
		return config.TelegramWebhookConfig{
			BotToken:      "123456:token",
			Secret:        "webhook-secret",
			PublicBaseURL: "https://scanner.example",
		}, nil
	}
	newRegistrar := func(token string) (telegrambot.WebhookRegistrar, error) {
		if token != "123456:token" {
			t.Fatalf("bot token = %q", token)
		}
		return registrar, nil
	}

	if err := runCommand(context.Background(), []string{"telegram", "set-webhook"}, load, newRegistrar); err != nil {
		t.Fatalf("runCommand() error = %v", err)
	}
	if registrar.url != "https://scanner.example/telegram/webhook" || registrar.secret != "webhook-secret" {
		t.Fatalf("registration URL=%q secret=%q", registrar.url, registrar.secret)
	}
}

type commandRegistrar struct {
	url       string
	secret    string
	confirmed bool
}

func (registrar *commandRegistrar) SetWebhook(_ context.Context, url, secret string) (bool, error) {
	registrar.url = url
	registrar.secret = secret
	return registrar.confirmed, nil
}
