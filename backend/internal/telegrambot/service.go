// Package telegrambot grants Scanner Access through the configured Telegram
// Administrator's private chat and delivers price and strategy alerts.
package telegrambot

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"crypto-scanner/internal/alerts"
	"crypto-scanner/internal/auth"
	"crypto-scanner/internal/platform/backoff"
	"crypto-scanner/internal/platform/logging"
	"crypto-scanner/internal/telegramhtml"

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
	// sendTimeout bounds the recipient lookup and the Telegram API call of a
	// notification. Rate-limit waits, the bot-wide limiter's and Telegram's
	// retry_after, are not bounded, so a burst is delayed, never dropped.
	sendTimeout = 10 * time.Second
	// clientQuietPeriod re-arms the client failure warning: while polling
	// fails, errors follow each other within the client's backoff (at most
	// 5s), and a successful long poll lasts its timeout (1 minute), so an
	// error after a longer silence starts a new failure.
	clientQuietPeriod = 2 * time.Minute
	// getMeTimeout bounds the getMe call that names the bot, the Bot API
	// client's own default for it.
	getMeTimeout = 5 * time.Second
)

// Options makes the bot boundary testable without changing its production
// transport. ServerURL is only useful for a fake Telegram endpoint in tests.
type Options struct {
	ServerURL string
}

// Service owns one long-polling Telegram Bot API consumer and its ephemeral
// confirmation state. Confirmation state is deliberately in-memory: a restart
// invalidates every previously rendered button.
type Service struct {
	bot             *telegram.Bot
	coins           telegramhtml.Coins
	store           auth.AccessStore
	administratorID int64
	logger          *slog.Logger

	mu            sync.Mutex
	nextPicker    int32
	operations    map[string]*operation
	sendLimiter   *rate.Limiter
	accessChanged func()
	// runDone holds the Done channel of Run's context, so the errors handler
	// tells the errors of the shutdown apart the moment it begins.
	runDone atomic.Pointer[<-chan struct{}]
	// clientFailure warns once while the client keeps failing; lastClientError
	// is when it last reported an error. clientMu guards both.
	clientMu        sync.Mutex
	clientFailure   *logging.Failure
	lastClientError time.Time
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
// access; other application users never gain that authority. accessChanged
// runs after the bot grants access.
func New(ctx context.Context, token string, administratorID int64, store auth.AccessStore, logger *slog.Logger, accessChanged func(), options Options) (*Service, error) {
	if administratorID <= 0 {
		return nil, fmt.Errorf("administrator Telegram ID must be positive")
	}
	if store == nil {
		return nil, fmt.Errorf("access store is required")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}
	if accessChanged == nil {
		return nil, fmt.Errorf("access change listener is required")
	}
	service := &Service{store: store, administratorID: administratorID, logger: logger, clientFailure: logging.NewFailure(logger), operations: map[string]*operation{}, sendLimiter: rate.NewLimiter(rate.Limit(25), 25), accessChanged: accessChanged}
	botOptions := []telegram.Option{
		telegram.WithAllowedUpdates(telegram.AllowedUpdates{"message", "callback_query"}),
		telegram.WithDefaultHandler(service.handleUpdate),
		// Handlers run on the polling workers, so none outlives Run.
		telegram.WithNotAsyncHandlers(),
		// The client reports polling errors, such as a conflicting consumer or
		// a revoked token, here instead of the standard logger.
		// Errors of the shutdown are not reported to the administrator.
		telegram.WithErrorsHandler(func(err error) {
			if service.stopping() || errors.Is(err, context.Canceled) {
				service.logger.Info("Telegram Bot API client failed", "module", "telegram_bot", "error", err)
				return
			}
			service.clientFailed(err)
		}),
	}
	if options.ServerURL != "" {
		botOptions = append(botOptions, telegram.WithServerURL(options.ServerURL))
	}
	// The bot's own getMe is skipped: the one below also learns its username.
	client, err := telegram.New(token, append(botOptions, telegram.WithSkipGetMe())...)
	if err != nil {
		return nil, fmt.Errorf("create Telegram bot: %w", err)
	}
	getMeCtx, cancel := context.WithTimeout(ctx, getMeTimeout)
	defer cancel()
	me, err := client.GetMe(getMeCtx)
	if err != nil {
		return nil, fmt.Errorf("get Telegram bot: %w", err)
	}
	if me.Username == "" {
		return nil, fmt.Errorf("get Telegram bot: no username")
	}
	service.bot = client
	service.coins = telegramhtml.NewCoins(me.Username)
	return service, nil
}

// Coins links coins in messages to the bot's Mini App.
func (service *Service) Coins() telegramhtml.Coins { return service.coins }

// Run consumes Telegram updates until context cancellation. The Bot API client
// owns long polling; this service never configures a webhook or BotFather menu.
func (service *Service) Run(ctx context.Context) error {
	done := ctx.Done()
	service.runDone.Store(&done)
	service.bot.Start(ctx)
	return nil
}

// clientFailed logs an error of the Bot API client, warning once while
// errors keep following each other within clientQuietPeriod.
func (service *Service) clientFailed(err error) {
	service.clientMu.Lock()
	defer service.clientMu.Unlock()
	now := time.Now()
	if !service.lastClientError.IsZero() && now.Sub(service.lastClientError) > clientQuietPeriod {
		service.clientFailure.Forget()
	}
	service.lastClientError = now
	service.clientFailure.Warn(context.Background(), "Telegram Bot API client failed", "module", "telegram_bot", "error", err)
}

// stopping reports that Run's context is done.
func (service *Service) stopping() bool {
	done := service.runDone.Load()
	if done == nil {
		return false
	}
	select {
	case <-*done:
		return true
	default:
		return false
	}
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
	service.accessChanged()
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

// SendPriceAlert delivers a price alert. It rechecks access immediately
// before sending and retries only once, after a Telegram 429.
func (service *Service) SendPriceAlert(ctx context.Context, fired alerts.Fired) error {
	return service.deliver(ctx, fired.Alert.TelegramID, priceAlertText(service.coins, fired))
}

// SendStrategyMessage delivers a strategy alert or summary like a price alert.
func (service *Service) SendStrategyMessage(ctx context.Context, telegramID int64, text string) error {
	return service.deliver(ctx, telegramID, text)
}

// SendAdministratorMessage sends an operational message to the configured
// administrator within the bot-wide rate limit. The administrator is
// configured, so access is not rechecked.
func (service *Service) SendAdministratorMessage(ctx context.Context, text string) error {
	return service.notify(ctx, service.administratorID, text, false)
}

// notification builds the parameters of text, written in Telegram HTML,
// without previews of its links. Every notification goes so; the bot's own
// dialog stays plain.
func notification(chatID int64, text string) *telegram.SendMessageParams {
	disabled := true
	return &telegram.SendMessageParams{ChatID: chatID, Text: text, ParseMode: models.ParseModeHTML, LinkPreviewOptions: &models.LinkPreviewOptions{IsDisabled: &disabled}}
}

// deliver sends a notification to a user, rechecking access before sending.
func (service *Service) deliver(ctx context.Context, telegramID int64, text string) error {
	return service.notify(ctx, telegramID, text, true)
}

// notify sends text to chatID and, when Telegram answers 429, waits its
// retry_after with the caller's context and tries once more.
func (service *Service) notify(ctx context.Context, chatID int64, text string, recheckAccess bool) error {
	for retried := false; ; retried = true {
		err := service.sendNotification(ctx, chatID, text, recheckAccess)
		var tooMany *telegram.TooManyRequestsError
		if retried || !errors.As(err, &tooMany) {
			return err
		}
		if err := backoff.Sleep(ctx, time.Duration(tooMany.RetryAfter)*time.Second); err != nil {
			return err
		}
	}
}

// sendNotification waits for the bot-wide rate limit with the caller's
// context, so only its cancellation ends the wait, and then sends text,
// rechecking the recipient's access first when asked.
func (service *Service) sendNotification(ctx context.Context, chatID int64, text string, recheckAccess bool) error {
	if err := service.sendLimiter.Wait(ctx); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	if recheckAccess {
		if _, err := service.store.FindByTelegramID(ctx, chatID); errors.Is(err, auth.ErrUserNotFound) {
			return fmt.Errorf("recipient no longer has access")
		} else if err != nil {
			return fmt.Errorf("look up recipient: %w", err)
		}
	}
	_, err := service.bot.SendMessage(ctx, notification(chatID, text))
	return err
}

// trimDecimal drops the trailing zeros Binance pads prices with, such as
// 65012.30000000 to 65012.3.
func trimDecimal(price string) string {
	if !strings.Contains(price, ".") {
		return price
	}
	trimmed := strings.TrimSuffix(strings.TrimRight(price, "0"), ".")
	if trimmed == "" {
		return "0"
	}
	return trimmed
}

// priceAlertText names the symbol, the price that hit the alert, and its
// target.
func priceAlertText(coins telegramhtml.Coins, fired alerts.Fired) string {
	return telegramhtml.Title("🔔 "+fired.Alert.Symbol+" · price alert") + "\n" +
		coins.Symbol(fired.Alert.Symbol) + " " + html.EscapeString(trimDecimal(fired.Price)) + " · target " + html.EscapeString(fired.Alert.Target)
}
