package telegram

import "context"

// Update is the bot module's input after the webhook transport is decoded.
type Update struct {
	Message *Message
}

// Message contains only the fields used by the bot launch behavior.
type Message struct {
	Text   string
	ChatID int64
}

// Outbound sends bot responses produced by Telegram updates.
type Outbound interface {
	SendMiniAppLaunch(ctx context.Context, chatID int64, miniAppURL string) error
}

// UpdateHandler dispatches supported Telegram bot commands.
type UpdateHandler struct {
	miniAppURL string
	outbound   Outbound
}

// NewUpdateHandler constructs the bot application module.
func NewUpdateHandler(miniAppURL string, outbound Outbound) *UpdateHandler {
	return &UpdateHandler{miniAppURL: miniAppURL, outbound: outbound}
}

// HandleUpdate handles supported commands and deliberately ignores unknown
// update shapes.
func (handler *UpdateHandler) HandleUpdate(ctx context.Context, update Update) error {
	if update.Message == nil || update.Message.Text != "/start" {
		return nil
	}
	return handler.outbound.SendMiniAppLaunch(ctx, update.Message.ChatID, handler.miniAppURL)
}
