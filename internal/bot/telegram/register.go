package telegram

import (
	"context"
	"fmt"
)

const webhookPath = "/telegram/webhook"

// WebhookRegistrar is the Telegram operation used by the administrative
// registration command.
type WebhookRegistrar interface {
	SetWebhook(ctx context.Context, url, secret string) (bool, error)
}

// RegisterWebhook installs the service webhook and requires Telegram's
// affirmative response.
func RegisterWebhook(ctx context.Context, registrar WebhookRegistrar, publicBaseURL, secret string) error {
	confirmed, err := registrar.SetWebhook(ctx, publicBaseURL+webhookPath, secret)
	if err != nil {
		return fmt.Errorf("register Telegram webhook: %w", err)
	}
	if !confirmed {
		return fmt.Errorf("Telegram did not confirm webhook registration")
	}
	return nil
}
