// Package telegrambot grants Scanner Access through the configured Telegram
// Administrator's private chat and delivers price alerts.
package telegrambot

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"crypto-scanner/internal/alerts"
	"crypto-scanner/internal/auth"

	telegram "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"golang.org/x/time/rate"
)

const (
	menuAddUser     = "Add user"
	callbackPrefix  = "scanner-access:"
	callbackConfirm = "confirm"
	callbackCancel  = "cancel"
	menuText        = "Add users here. Delete them in the Mini App settings."
)

// Options makes the bot boundary testable without changing its production
// transport. ServerURL is only useful for a fake Telegram endpoint in tests.
type Options struct {
	ServerURL     string
	HTTPClient    telegram.HttpClient
	PollTimeout   time.Duration
	Synchronous   bool
	AccessChanged func()
}

// Service owns one long-polling Telegram Bot API consumer and its ephemeral
// confirmation state. Confirmation state is deliberately in-memory: a restart
// invalidates every previously rendered button.
type Service struct {
	bot             *telegram.Bot
	store           auth.AccessStore
	administratorID int64
	logger          *slog.Logger

	mu            sync.Mutex
	nextPicker    int32
	operations    map[string]*operation
	sendLimiter   *rate.Limiter
	userLimiters  [4096]*rate.Limiter
	accessChanged func()
}

// operation is one Add user flow: a picker request, then the chosen person
// awaiting confirmation.
type operation struct {
	chatID    int64
	requestID int32
	user      auth.User
	busy      bool
	// confirmationID is the message with the Confirm and Cancel buttons, or
	// zero before it is sent.
	confirmationID int
}

// New constructs a Telegram Bot API client without changing any BotFather
// settings. The configured administrator ID is the only authority for granting
// access; other application users never gain that authority.
func New(token string, administratorID int64, store auth.AccessStore, logger *slog.Logger, options Options) (*Service, error) {
	if administratorID <= 0 {
		return nil, fmt.Errorf("administrator Telegram ID must be positive")
	}
	if store == nil {
		return nil, fmt.Errorf("access store is required")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}
	service := &Service{store: store, administratorID: administratorID, logger: logger, operations: map[string]*operation{}, sendLimiter: rate.NewLimiter(rate.Limit(25), 25), accessChanged: options.AccessChanged}
	botOptions := []telegram.Option{
		telegram.WithAllowedUpdates(telegram.AllowedUpdates{"message", "callback_query"}),
		telegram.WithDefaultHandler(service.handleUpdate),
	}
	if options.ServerURL != "" {
		botOptions = append(botOptions, telegram.WithServerURL(options.ServerURL))
	}
	if options.HTTPClient != nil {
		pollTimeout := options.PollTimeout
		if pollTimeout <= 0 {
			pollTimeout = 30 * time.Second
		}
		botOptions = append(botOptions, telegram.WithHTTPClient(pollTimeout, options.HTTPClient))
	}
	if options.Synchronous {
		botOptions = append(botOptions, telegram.WithNotAsyncHandlers())
	}
	client, err := telegram.New(token, botOptions...)
	if err != nil {
		return nil, fmt.Errorf("create Telegram bot: %w", err)
	}
	service.bot = client
	return service, nil
}

// Run consumes Telegram updates until context cancellation. The Bot API client
// owns long polling; this service never configures a webhook or BotFather menu.
func (service *Service) Run(ctx context.Context) error {
	service.bot.Start(ctx)
	return nil
}

// ProcessUpdate is the update-processing seam used by the feature harness.
func (service *Service) ProcessUpdate(ctx context.Context, update *models.Update) {
	if update != nil {
		service.bot.ProcessUpdate(ctx, update)
	}
}

func (service *Service) handleUpdate(ctx context.Context, client *telegram.Bot, update *models.Update) {
	if update.Message != nil {
		service.handleMessage(ctx, client, update.Message)
		return
	}
	if update.CallbackQuery != nil {
		service.handleCallback(ctx, client, update.CallbackQuery)
	}
}

func (service *Service) handleMessage(ctx context.Context, client *telegram.Bot, message *models.Message) {
	if message == nil || message.From == nil || message.Chat.Type != models.ChatTypePrivate || message.Chat.ID != message.From.ID {
		return
	}
	if message.From.ID != service.administratorID {
		service.handleNonAdministrator(ctx, client, message)
		return
	}
	if message.UsersShared != nil {
		service.handleSharedUser(ctx, client, message)
		return
	}
	if message.Text == menuAddUser {
		service.requestUser(ctx, client, message.Chat.ID)
		return
	}
	// Any other message abandons an unfinished Add user flow.
	service.mu.Lock()
	abandoned := service.invalidateChatOperations(message.Chat.ID)
	service.mu.Unlock()
	service.removeAllButtons(ctx, client, message.Chat.ID, abandoned)
	service.sendMenu(ctx, client, message.Chat.ID, menuText)
}

func (service *Service) handleNonAdministrator(ctx context.Context, client *telegram.Bot, message *models.Message) {
	if _, err := service.store.FindByTelegramID(ctx, message.From.ID); err == nil {
		service.send(ctx, client, message.Chat.ID, "Scanner Access is active. Open the existing Main Mini App from this bot's profile.", nil)
		return
	}
	service.send(ctx, client, message.Chat.ID, "Access has not been granted. Contact the Administrator.", nil)
}

// sendMenu also replaces the picker keyboard, so every finished flow returns
// the administrator to the menu.
func (service *Service) sendMenu(ctx context.Context, client *telegram.Bot, chatID int64, text string) {
	service.send(ctx, client, chatID, text, &models.ReplyKeyboardMarkup{ResizeKeyboard: true, Keyboard: [][]models.KeyboardButton{
		{{Text: menuAddUser}},
	}})
}

func (service *Service) requestUser(ctx context.Context, client *telegram.Bot, chatID int64) {
	requestID, abandoned := service.newPickerOperation(chatID)
	service.removeAllButtons(ctx, client, chatID, abandoned)
	service.send(ctx, client, chatID, "Choose one person to grant Scanner Access.", &models.ReplyKeyboardMarkup{ResizeKeyboard: true, OneTimeKeyboard: true, Keyboard: [][]models.KeyboardButton{{{
		Text: "Choose a person",
		RequestUsers: &models.KeyboardButtonRequestUsers{
			// TODO: Restrict the picker to people once github.com/go-telegram/bot
			// represents user_is_bot as *bool. In v1.25.0 false is omitted during
			// JSON encoding, which Telegram interprets as no bot restriction.
			RequestID: requestID, UserIsBot: false, MaxQuantity: 1, RequestName: true, RequestUsername: true,
		},
	}}}})
}

func (service *Service) handleSharedUser(ctx context.Context, client *telegram.Bot, message *models.Message) {
	shared := message.UsersShared
	if len(shared.Users) != 1 || shared.Users[0].UserID <= 0 {
		service.send(ctx, client, message.Chat.ID, "Choose exactly one person from the picker.", nil)
		return
	}
	service.mu.Lock()
	var token string
	for candidate, operation := range service.operations {
		if operation.chatID == message.Chat.ID && operation.requestID == int32(shared.RequestID) && operation.user.TelegramID == 0 && !operation.busy {
			token = candidate
			break
		}
	}
	if token == "" {
		service.mu.Unlock()
		service.sendMenu(ctx, client, message.Chat.ID, "This selection is no longer valid. Choose Add user again.")
		return
	}
	selected := shared.Users[0]
	service.operations[token].user = auth.User{TelegramID: selected.UserID, Username: selected.Username, DisplayName: displayName(selected.FirstName, selected.LastName)}
	user := service.operations[token].user
	service.mu.Unlock()
	confirmation, err := client.SendMessage(ctx, &telegram.SendMessageParams{ChatID: message.Chat.ID, Text: "Grant Scanner Access to " + formatUser(user) + "?", ReplyMarkup: &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{
		{Text: "Confirm", CallbackData: callbackPrefix + callbackConfirm + ":" + token},
		{Text: "Cancel", CallbackData: callbackPrefix + callbackCancel + ":" + token},
	}}}})
	if err != nil {
		service.logger.WarnContext(ctx, "Telegram message failed", "module", "telegram_bot", "operation", "send_message", "error", err)
		return
	}
	service.mu.Lock()
	operation := service.operations[token]
	if operation != nil {
		operation.confirmationID = confirmation.ID
	}
	service.mu.Unlock()
	if operation == nil {
		// The flow was abandoned while the confirmation was being sent.
		service.removeButtons(ctx, client, message.Chat.ID, confirmation.ID)
	}
}

func (service *Service) handleCallback(ctx context.Context, client *telegram.Bot, callback *models.CallbackQuery) {
	chat, ok := callbackChat(callback)
	if !ok || callback.From.ID != service.administratorID || chat.ID != service.administratorID {
		service.answer(ctx, client, callback.ID, "This action is not available.")
		return
	}
	action, token, _ := strings.Cut(strings.TrimPrefix(callback.Data, callbackPrefix), ":")
	service.mu.Lock()
	operation := service.operations[token]
	valid := strings.HasPrefix(callback.Data, callbackPrefix) && (action == callbackConfirm || action == callbackCancel) &&
		operation != nil && operation.chatID == chat.ID && operation.user.TelegramID != 0
	if !valid || operation.busy {
		pending := service.hasChatOperations(chat.ID)
		service.mu.Unlock()
		service.answer(ctx, client, callback.ID, "This action is no longer valid.")
		if !valid {
			// Buttons of a finished or forgotten flow, such as one from before
			// a restart, must not linger, and the picker keyboard must give way
			// to the menu unless a newer flow is using it.
			service.removeButtons(ctx, client, chat.ID, callback.Message.Message.ID)
			if !pending {
				service.sendMenu(ctx, client, chat.ID, "This confirmation has expired. Choose Add user again.")
			}
		}
		return
	}
	if action == callbackCancel {
		delete(service.operations, token)
		service.mu.Unlock()
		service.answer(ctx, client, callback.ID, "Cancelled.")
		service.removeButtons(ctx, client, chat.ID, callback.Message.Message.ID)
		service.sendMenu(ctx, client, chat.ID, "No Scanner Access changes were made.")
		return
	}
	operation.busy = true
	service.mu.Unlock()

	_, created, err := service.store.GrantAccess(ctx, operation.user.TelegramID, operation.user.Username, operation.user.DisplayName)
	service.mu.Lock()
	delete(service.operations, token)
	service.mu.Unlock()
	service.removeButtons(ctx, client, chat.ID, callback.Message.Message.ID)
	if err != nil {
		service.logger.ErrorContext(ctx, "Scanner Access grant failed", "module", "telegram_bot", "error", err)
		service.answer(ctx, client, callback.ID, "This action could not be completed.")
		service.sendMenu(ctx, client, chat.ID, "Scanner Access was not changed. Please try again.")
		return
	}
	service.answer(ctx, client, callback.ID, "")
	if !created {
		service.sendMenu(ctx, client, chat.ID, formatUser(operation.user)+" already has Scanner Access.")
		return
	}
	if service.accessChanged != nil {
		service.accessChanged()
	}
	service.sendMenu(ctx, client, chat.ID, "Scanner Access granted to "+formatUser(operation.user)+".")
}

// newPickerOperation starts an Add user flow and returns its picker request ID
// and the confirmations of the flows it abandoned.
func (service *Service) newPickerOperation(chatID int64) (int32, []int) {
	service.mu.Lock()
	defer service.mu.Unlock()
	abandoned := service.invalidateChatOperations(chatID)
	service.nextPicker++
	if service.nextPicker <= 0 {
		service.nextPicker = 1
	}
	service.operations[newToken()] = &operation{chatID: chatID, requestID: service.nextPicker}
	return service.nextPicker, abandoned
}

// invalidateChatOperations abandons the chat's idle flows and returns the
// confirmation messages whose buttons must be removed. The caller holds mu.
func (service *Service) invalidateChatOperations(chatID int64) []int {
	var confirmations []int
	for token, operation := range service.operations {
		if operation.chatID == chatID && !operation.busy {
			delete(service.operations, token)
			if operation.confirmationID != 0 {
				confirmations = append(confirmations, operation.confirmationID)
			}
		}
	}
	return confirmations
}

// hasChatOperations reports an unfinished flow in the chat. The caller holds mu.
func (service *Service) hasChatOperations(chatID int64) bool {
	for _, operation := range service.operations {
		if operation.chatID == chatID {
			return true
		}
	}
	return false
}

func (service *Service) send(ctx context.Context, client *telegram.Bot, chatID int64, text string, markup models.ReplyMarkup) {
	if _, err := client.SendMessage(ctx, &telegram.SendMessageParams{ChatID: chatID, Text: text, ReplyMarkup: markup}); err != nil {
		service.logger.WarnContext(ctx, "Telegram message failed", "module", "telegram_bot", "operation", "send_message", "error", err)
	}
}

func (service *Service) removeAllButtons(ctx context.Context, client *telegram.Bot, chatID int64, messageIDs []int) {
	for _, messageID := range messageIDs {
		service.removeButtons(ctx, client, chatID, messageID)
	}
}

// removeButtons strips the inline keyboard from a confirmation message.
func (service *Service) removeButtons(ctx context.Context, client *telegram.Bot, chatID int64, messageID int) {
	if _, err := client.EditMessageReplyMarkup(ctx, &telegram.EditMessageReplyMarkupParams{ChatID: chatID, MessageID: messageID}); err != nil {
		service.logger.WarnContext(ctx, "Telegram message edit failed", "module", "telegram_bot", "operation", "edit_message_reply_markup", "error", err)
	}
}

func (service *Service) answer(ctx context.Context, client *telegram.Bot, callbackID, text string) {
	if _, err := client.AnswerCallbackQuery(ctx, &telegram.AnswerCallbackQueryParams{CallbackQueryID: callbackID, Text: text}); err != nil {
		service.logger.WarnContext(ctx, "Telegram callback answer failed", "module", "telegram_bot", "operation", "answer_callback", "error", err)
	}
}

func callbackChat(callback *models.CallbackQuery) (models.Chat, bool) {
	if callback == nil || callback.Message.Message == nil || callback.Message.Message.Chat.Type != models.ChatTypePrivate {
		return models.Chat{}, false
	}
	return callback.Message.Message.Chat, true
}

func displayName(first, last string) string {
	return strings.TrimSpace(strings.TrimSpace(first) + " " + strings.TrimSpace(last))
}

func formatUser(user auth.User) string {
	name := strings.TrimSpace(user.DisplayName)
	if name == "" {
		name = "User"
	}
	if username := strings.TrimSpace(user.Username); username != "" {
		return fmt.Sprintf("%s (@%s, ID %d)", name, username, user.TelegramID)
	}
	return fmt.Sprintf("%s (ID %d)", name, user.TelegramID)
}

func newToken() string {
	bytes := make([]byte, 12)
	_, _ = rand.Read(bytes) // never fails since Go 1.24
	return hex.EncodeToString(bytes)
}

// SendPriceAlert performs one best-effort Telegram API request. It rechecks
// access immediately before sending and intentionally has no retry.
func (service *Service) SendPriceAlert(ctx context.Context, fired alerts.Fired) error {
	service.mu.Lock()
	// A fixed shard set keeps limiter memory bounded. Hash collisions only make
	// delivery more conservative; they can never let one user exceed the limit.
	shard := uint64(fired.Alert.TelegramID) % uint64(len(service.userLimiters))
	limiter := service.userLimiters[shard]
	if limiter == nil {
		limiter = rate.NewLimiter(rate.Every(time.Second), 1)
		service.userLimiters[shard] = limiter
	}
	service.mu.Unlock()
	if err := service.sendLimiter.Wait(ctx); err != nil {
		return err
	}
	if err := limiter.Wait(ctx); err != nil {
		return err
	}
	if _, err := service.store.FindByTelegramID(ctx, fired.Alert.TelegramID); errors.Is(err, auth.ErrUserNotFound) {
		return fmt.Errorf("alert owner no longer has access")
	} else if err != nil {
		return fmt.Errorf("look up alert owner: %w", err)
	}
	_, err := service.bot.SendMessage(ctx, &telegram.SendMessageParams{ChatID: fired.Alert.TelegramID, Text: priceAlertText(fired)})
	return err
}

// priceAlertText names only the symbol and the target that was hit.
func priceAlertText(fired alerts.Fired) string {
	return "🔔 " + fired.Alert.Symbol + " hit " + fired.Alert.Target
}
