package telegram

import (
	"context"
	"fmt"

	telegramapi "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// Client adapts the Telegram Bot API to the bot module's outbound operations.
type Client struct {
	bot *telegramapi.Bot
}

// NewClient constructs a Telegram outbound client without making a network
// request. Connectivity is exercised only by explicit bot operations.
func NewClient(token string) (*Client, error) {
	bot, err := telegramapi.New(token, telegramapi.WithSkipGetMe())
	if err != nil {
		return nil, fmt.Errorf("create Telegram client: %w", err)
	}
	return newClient(bot), nil
}

func newClient(bot *telegramapi.Bot) *Client {
	return &Client{bot: bot}
}

// SendMiniAppLaunch sends the launch UX for the configured Mini App.
func (client *Client) SendMiniAppLaunch(ctx context.Context, chatID int64, miniAppURL string) error {
	_, err := client.bot.SendMessage(ctx, &telegramapi.SendMessageParams{
		ChatID: chatID,
		Text:   "Open Crypto Scanner",
		ReplyMarkup: models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{{
			Text:   "Open Crypto Scanner",
			WebApp: &models.WebAppInfo{URL: miniAppURL},
		}}}},
	})
	if err != nil {
		return fmt.Errorf("send Mini App launch: %w", err)
	}
	return nil
}

// SetWebhook requests Telegram webhook registration.
func (client *Client) SetWebhook(ctx context.Context, url, secret string) (bool, error) {
	confirmed, err := client.bot.SetWebhook(ctx, &telegramapi.SetWebhookParams{
		URL:         url,
		SecretToken: secret,
	})
	if err != nil {
		return false, fmt.Errorf("set Telegram webhook: %w", err)
	}
	return confirmed, nil
}
