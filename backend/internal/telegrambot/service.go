// Package telegrambot implements the private-chat administration interaction
// for Crypto Scanner's configured Telegram Administrator.
package telegrambot

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"crypto-scanner/internal/auth"

	telegram "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	menuListUsers   = "List users"
	menuAddUser     = "Add user"
	menuDeleteUser  = "Delete user"
	pageSize        = 8
	callbackPrefix  = "scanner-access:"
	callbackConfirm = "confirm"
	callbackCancel  = "cancel"
)

// Options makes the bot boundary testable without changing its production
// transport. ServerURL is only useful for a fake Telegram endpoint in tests.
type Options struct {
	ServerURL   string
	HTTPClient  telegram.HttpClient
	PollTimeout time.Duration
	Synchronous bool
	Logger      *slog.Logger
}

// Service owns one long-polling Telegram Bot API consumer and its ephemeral
// confirmation state. Confirmation state is deliberately in-memory: a restart
// invalidates every previously rendered button.
type Service struct {
	bot             *telegram.Bot
	store           auth.AccessStore
	administratorID int64
	logger          *slog.Logger

	mu         sync.Mutex
	nextPicker int32
	operations map[string]*operation
}

type operationKind string

const (
	operationAdd    operationKind = "add"
	operationDelete operationKind = "delete"
)

type operation struct {
	kind      operationKind
	chatID    int64
	requestID int32
	user      auth.User
	busy      bool
}

type userPage struct {
	users []auth.User
	more  bool
}

// New constructs a Telegram Bot API client without changing any BotFather
// settings. The configured administrator ID is the only authority for access
// management; enabled application users never gain that authority.
func New(token string, administratorID int64, store auth.AccessStore, options Options) (*Service, error) {
	if administratorID <= 0 {
		return nil, fmt.Errorf("administrator Telegram ID must be positive")
	}
	if store == nil {
		return nil, fmt.Errorf("access store is required")
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	service := &Service{store: store, administratorID: administratorID, logger: logger, operations: map[string]*operation{}}
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
	switch message.Text {
	case "/start":
		service.sendMenu(ctx, client, message.Chat.ID, "Administrator menu:")
	case "/help":
		service.sendMenu(ctx, client, message.Chat.ID, "Choose an access-management action:")
	case menuListUsers:
		service.listUsers(ctx, client, message.Chat.ID, 0)
	case menuAddUser:
		service.requestUser(ctx, client, message.Chat.ID)
	case menuDeleteUser:
		service.selectUserForDeletion(ctx, client, message.Chat.ID, 0)
	default:
		service.sendMenu(ctx, client, message.Chat.ID, "Use the menu to manage Scanner Access.")
	}
}

func (service *Service) handleNonAdministrator(ctx context.Context, client *telegram.Bot, message *models.Message) {
	if _, err := service.store.FindEnabledByTelegramID(ctx, message.From.ID); err == nil {
		service.send(ctx, client, message.Chat.ID, "Scanner Access is active. Open the existing Main Mini App from this bot's profile.", nil)
		return
	}
	service.send(ctx, client, message.Chat.ID, "Access has not been granted. Contact the Administrator.", nil)
}

func (service *Service) sendMenu(ctx context.Context, client *telegram.Bot, chatID int64, text string) {
	service.send(ctx, client, chatID, text, &models.ReplyKeyboardMarkup{ResizeKeyboard: true, Keyboard: [][]models.KeyboardButton{
		{{Text: menuListUsers}}, {{Text: menuAddUser}}, {{Text: menuDeleteUser}},
	}})
}

func (service *Service) requestUser(ctx context.Context, client *telegram.Bot, chatID int64) {
	requestID := service.newPickerOperation(chatID)
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
		if operation.kind == operationAdd && operation.chatID == message.Chat.ID && operation.requestID == int32(shared.RequestID) && operation.user.TelegramID == 0 && !operation.busy {
			token = candidate
			break
		}
	}
	if token == "" {
		hasPendingPicker := false
		for _, operation := range service.operations {
			if operation.kind == operationAdd && operation.chatID == message.Chat.ID && operation.user.TelegramID == 0 && !operation.busy {
				hasPendingPicker = true
				break
			}
		}
		service.mu.Unlock()
		if hasPendingPicker {
			service.send(ctx, client, message.Chat.ID, "This selection is no longer valid. Choose Add user again.", nil)
			return
		}
		service.sendWithoutReplyKeyboard(ctx, client, message.Chat.ID, "This selection is no longer valid. Choose Add user again.")
		return
	}
	selected := shared.Users[0]
	service.operations[token].user = auth.User{TelegramID: selected.UserID, Username: selected.Username, DisplayName: displayName(selected.FirstName, selected.LastName), Enabled: true}
	operation := service.operations[token]
	service.mu.Unlock()
	service.sendConfirmation(ctx, client, message.Chat.ID, operation, token)
}

func (service *Service) listUsers(ctx context.Context, client *telegram.Bot, chatID int64, offset int) {
	administrator, err := service.store.FindEnabledByTelegramID(ctx, service.administratorID)
	if err != nil {
		service.send(ctx, client, chatID, "Could not list Scanner Access users. Please try again.", nil)
		return
	}
	page, err := service.nonAdministratorPage(ctx, offset)
	if err != nil {
		service.send(ctx, client, chatID, "Could not list Scanner Access users. Please try again.", nil)
		return
	}
	if len(page.users) == 0 && offset == 0 {
		service.send(ctx, client, chatID, "Scanner Access users:\nAdministrator — "+formatUser(administrator)+"\nNo other users have Scanner Access.", nil)
		return
	}
	rows := make([]string, 0, len(page.users)+1)
	if offset == 0 {
		rows = append(rows, "Administrator — "+formatUser(administrator))
	}
	for _, user := range page.users {
		rows = append(rows, formatUser(user))
	}
	keyboard := &models.InlineKeyboardMarkup{}
	if offset > 0 || page.more {
		buttons := []models.InlineKeyboardButton{}
		if offset > 0 {
			buttons = append(buttons, models.InlineKeyboardButton{Text: "Previous", CallbackData: fmt.Sprintf("%slist:%d", callbackPrefix, max(offset-pageSize, 0))})
		}
		if page.more {
			buttons = append(buttons, models.InlineKeyboardButton{Text: "Next", CallbackData: fmt.Sprintf("%slist:%d", callbackPrefix, offset+pageSize)})
		}
		keyboard.InlineKeyboard = [][]models.InlineKeyboardButton{buttons}
	}
	service.send(ctx, client, chatID, "Scanner Access users:\n"+strings.Join(rows, "\n"), keyboard)
}

func (service *Service) selectUserForDeletion(ctx context.Context, client *telegram.Bot, chatID int64, offset int) {
	service.mu.Lock()
	service.invalidateChatOperations(chatID)
	service.mu.Unlock()
	page, err := service.nonAdministratorPage(ctx, offset)
	if err != nil {
		service.send(ctx, client, chatID, "Could not list Scanner Access users. Please try again.", nil)
		return
	}
	buttons := make([][]models.InlineKeyboardButton, 0, len(page.users)+1)
	for _, user := range page.users {
		if user.TelegramID == service.administratorID {
			continue
		}
		token := service.newDeleteOperation(chatID, user)
		buttons = append(buttons, []models.InlineKeyboardButton{{Text: formatUser(user), CallbackData: callbackPrefix + "delete:" + token}})
	}
	if len(buttons) == 0 {
		service.send(ctx, client, chatID, "No other users can be removed.", nil)
		return
	}
	navigation := []models.InlineKeyboardButton{}
	if offset > 0 {
		navigation = append(navigation, models.InlineKeyboardButton{Text: "Previous", CallbackData: fmt.Sprintf("%sdelete-page:%d", callbackPrefix, max(offset-pageSize, 0))})
	}
	if page.more {
		navigation = append(navigation, models.InlineKeyboardButton{Text: "Next", CallbackData: fmt.Sprintf("%sdelete-page:%d", callbackPrefix, offset+pageSize)})
	}
	if len(navigation) > 0 {
		buttons = append(buttons, navigation)
	}
	service.send(ctx, client, chatID, "Choose a user to remove from Scanner Access.", &models.InlineKeyboardMarkup{InlineKeyboard: buttons})
}

func (service *Service) nonAdministratorPage(ctx context.Context, offset int) (userPage, error) {
	users, err := service.store.ListNonAdministratorUsers(ctx, service.administratorID, offset, pageSize+1)
	if err != nil {
		return userPage{}, err
	}
	page := userPage{users: users, more: len(users) > pageSize}
	if page.more {
		page.users = page.users[:pageSize]
	}
	return page, nil
}

func (service *Service) handleCallback(ctx context.Context, client *telegram.Bot, callback *models.CallbackQuery) {
	chat, ok := callbackChat(callback)
	if !ok || callback.From.ID != service.administratorID || chat.ID != service.administratorID {
		service.answer(ctx, client, callback.ID, "This action is not available.")
		return
	}
	if strings.HasPrefix(callback.Data, callbackPrefix+"list:") {
		var offset int
		if _, err := fmt.Sscanf(strings.TrimPrefix(callback.Data, callbackPrefix+"list:"), "%d", &offset); err == nil && offset >= 0 {
			service.answer(ctx, client, callback.ID, "")
			service.listUsers(ctx, client, chat.ID, offset)
			return
		}
	}
	if strings.HasPrefix(callback.Data, callbackPrefix+"delete-page:") {
		var offset int
		if _, err := fmt.Sscanf(strings.TrimPrefix(callback.Data, callbackPrefix+"delete-page:"), "%d", &offset); err == nil && offset >= 0 {
			service.answer(ctx, client, callback.ID, "")
			service.selectUserForDeletion(ctx, client, chat.ID, offset)
			return
		}
	}
	parts := strings.Split(callback.Data, ":")
	if len(parts) == 3 && parts[0] == "scanner-access" && parts[1] == "delete" {
		service.beginDeletionConfirmation(ctx, client, callback, chat.ID, parts[2])
		return
	}
	if len(parts) == 4 && parts[0] == "scanner-access" && (parts[1] == string(operationAdd) || parts[1] == string(operationDelete)) {
		service.confirmOrCancel(ctx, client, callback, chat.ID, operationKind(parts[1]), parts[2], parts[3])
		return
	}
	service.answer(ctx, client, callback.ID, "This action is no longer valid.")
}

func (service *Service) beginDeletionConfirmation(ctx context.Context, client *telegram.Bot, callback *models.CallbackQuery, chatID int64, token string) {
	service.mu.Lock()
	operation := service.operations[token]
	if operation == nil || operation.kind != operationDelete || operation.chatID != chatID || operation.busy || operation.user.TelegramID == service.administratorID {
		service.mu.Unlock()
		service.answer(ctx, client, callback.ID, "This action is no longer valid.")
		return
	}
	service.mu.Unlock()
	service.answer(ctx, client, callback.ID, "")
	service.sendConfirmation(ctx, client, chatID, operation, token)
}

func (service *Service) confirmOrCancel(ctx context.Context, client *telegram.Bot, callback *models.CallbackQuery, chatID int64, kind operationKind, action, token string) {
	service.mu.Lock()
	operation := service.operations[token]
	if operation == nil || operation.kind != kind || operation.chatID != chatID || operation.busy {
		service.mu.Unlock()
		service.answer(ctx, client, callback.ID, "This action is no longer valid.")
		return
	}
	if action == callbackCancel {
		delete(service.operations, token)
		service.mu.Unlock()
		service.answer(ctx, client, callback.ID, "Cancelled.")
		if kind == operationAdd {
			service.sendWithoutReplyKeyboard(ctx, client, chatID, "No Scanner Access changes were made.")
			return
		}
		service.send(ctx, client, chatID, "No Scanner Access changes were made.", nil)
		return
	}
	if action != callbackConfirm {
		service.mu.Unlock()
		service.answer(ctx, client, callback.ID, "This action is no longer valid.")
		return
	}
	operation.busy = true
	service.mu.Unlock()

	var err error
	var deleted bool
	var created bool
	if kind == operationAdd {
		_, created, err = service.store.GrantAccess(ctx, operation.user.TelegramID, operation.user.Username, operation.user.DisplayName)
	} else if operation.user.TelegramID == service.administratorID {
		err = fmt.Errorf("administrator removal is prohibited")
	} else {
		deleted, err = service.store.DeleteUser(ctx, operation.user.ID, operation.user.TelegramID)
	}
	service.mu.Lock()
	if kind == operationAdd || (err == nil && deleted) {
		delete(service.operations, token)
	} else if current := service.operations[token]; current != nil {
		current.busy = false
	}
	service.mu.Unlock()
	if err != nil || kind == operationDelete && !deleted {
		service.answer(ctx, client, callback.ID, "This action could not be completed.")
		if kind == operationAdd {
			service.sendWithoutReplyKeyboard(ctx, client, chatID, "Scanner Access was not changed. Please try again.")
			return
		}
		service.send(ctx, client, chatID, "Scanner Access was not changed. Please try again.", nil)
		return
	}
	service.answer(ctx, client, callback.ID, "")
	if kind == operationAdd && !created {
		service.sendWithoutReplyKeyboard(ctx, client, chatID, formatUser(operation.user)+" already has Scanner Access.")
		return
	}
	if kind == operationAdd {
		service.sendWithoutReplyKeyboard(ctx, client, chatID, "Scanner Access granted to "+formatUser(operation.user)+".")
		return
	}
	service.send(ctx, client, chatID, "Scanner Access removed from "+formatUser(operation.user)+".", nil)
}

func (service *Service) sendConfirmation(ctx context.Context, client *telegram.Bot, chatID int64, operation *operation, token string) {
	verb := "Grant Scanner Access to "
	if operation.kind == operationDelete {
		verb = "Remove Scanner Access from "
	}
	service.send(ctx, client, chatID, verb+formatUser(operation.user)+"?", &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{
		{Text: "Confirm", CallbackData: fmt.Sprintf("%s%s:%s:%s", callbackPrefix, operation.kind, callbackConfirm, token)},
		{Text: "Cancel", CallbackData: fmt.Sprintf("%s%s:%s:%s", callbackPrefix, operation.kind, callbackCancel, token)},
	}}})
}

func (service *Service) newPickerOperation(chatID int64) int32 {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.invalidateChatOperations(chatID)
	service.nextPicker++
	if service.nextPicker <= 0 {
		service.nextPicker = 1
	}
	service.operations[newToken()] = &operation{kind: operationAdd, chatID: chatID, requestID: service.nextPicker}
	return service.nextPicker
}

func (service *Service) newDeleteOperation(chatID int64, user auth.User) string {
	service.mu.Lock()
	defer service.mu.Unlock()
	token := newToken()
	service.operations[token] = &operation{kind: operationDelete, chatID: chatID, user: user}
	return token
}

func (service *Service) invalidateChatOperations(chatID int64) {
	for token, operation := range service.operations {
		if operation.chatID == chatID {
			delete(service.operations, token)
		}
	}
}

func (service *Service) sendWithoutReplyKeyboard(ctx context.Context, client *telegram.Bot, chatID int64, text string) {
	service.send(ctx, client, chatID, text, &models.ReplyKeyboardRemove{RemoveKeyboard: true})
}

func (service *Service) send(ctx context.Context, client *telegram.Bot, chatID int64, text string, markup models.ReplyMarkup) {
	if _, err := client.SendMessage(ctx, &telegram.SendMessageParams{ChatID: chatID, Text: text, ReplyMarkup: markup}); err != nil {
		service.logger.WarnContext(ctx, "Telegram message failed", "module", "telegram_bot", "operation", "send_message", "error", err.Error())
	}
}

func (service *Service) answer(ctx context.Context, client *telegram.Bot, callbackID, text string) {
	if _, err := client.AnswerCallbackQuery(ctx, &telegram.AnswerCallbackQueryParams{CallbackQueryID: callbackID, Text: text}); err != nil {
		service.logger.WarnContext(ctx, "Telegram callback answer failed", "module", "telegram_bot", "operation", "answer_callback", "error", err.Error())
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
	if _, err := rand.Read(bytes); err == nil {
		return hex.EncodeToString(bytes)
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func max(left, right int) int {
	if left > right {
		return left
	}
	return right
}
