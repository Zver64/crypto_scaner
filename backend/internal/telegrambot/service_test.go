package telegrambot_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"crypto-scanner/internal/alerts"
	"crypto-scanner/internal/auth"
	"crypto-scanner/internal/telegrambot"

	"github.com/go-telegram/bot/models"
)

func TestAdministratorCanAddAndConfirmUserAccess(t *testing.T) {
	transport := newTelegramTransport(t)
	store := &accessStore{}
	accessChanges := 0
	service, err := telegrambot.New("123456:test-token", 100, store, slog.New(slog.DiscardHandler), func() { accessChanges++ }, telegrambot.Options{ServerURL: transport.URL, Synchronous: true})
	if err != nil {
		t.Fatalf("create bot service: %v", err)
	}

	service.ProcessUpdate(context.Background(), messageUpdate(100, "/start"))
	assertMenu(t, transport.lastSendMessage(t))

	service.ProcessUpdate(context.Background(), messageUpdate(100, "Add user"))
	picker := transport.lastSendMessage(t)
	if picker.ReplyMarkup.Keyboard[0][0].RequestUsers == nil || picker.ReplyMarkup.Keyboard[0][0].RequestUsers.MaxQuantity != 1 || !picker.ReplyMarkup.Keyboard[0][0].RequestUsers.RequestName || !picker.ReplyMarkup.Keyboard[0][0].RequestUsers.RequestUsername {
		t.Fatalf("unexpected user picker: %#v", picker.ReplyMarkup)
	}
	requestID := picker.ReplyMarkup.Keyboard[0][0].RequestUsers.RequestID
	service.ProcessUpdate(context.Background(), &models.Update{Message: &models.Message{
		From: &models.User{ID: 100}, Chat: models.Chat{ID: 100, Type: models.ChatTypePrivate},
		UsersShared: &models.UsersShared{RequestID: int(requestID), Users: []models.SharedUser{{UserID: 200, FirstName: "Ada", Username: "ada"}}},
	}})
	confirmation := transport.lastSendMessage(t)
	if confirmation.Text != "Grant Scanner Access to Ada (@ada, ID 200)?" || confirmation.Inline.InlineKeyboard[0][0].Text != "Confirm" || confirmation.Inline.InlineKeyboard[0][1].Text != "Cancel" {
		t.Fatalf("unexpected add confirmation: %#v", confirmation)
	}
	confirm := confirmation.Inline.InlineKeyboard[0][0].CallbackData
	service.ProcessUpdate(context.Background(), &models.Update{Message: &models.Message{
		From: &models.User{ID: 100}, Chat: models.Chat{ID: 100, Type: models.ChatTypePrivate},
		UsersShared: &models.UsersShared{RequestID: int(requestID), Users: []models.SharedUser{{UserID: 201, FirstName: "Babbage"}}},
	}})
	service.ProcessUpdate(context.Background(), callbackUpdate(100, confirm))
	if _, err := store.FindByTelegramID(context.Background(), 200); err != nil {
		t.Fatalf("confirmed user has no access: %v", err)
	}
	if _, err := store.FindByTelegramID(context.Background(), 201); err != auth.ErrUserNotFound {
		t.Fatalf("replayed picker selection changed confirmed identity: %v", err)
	}
	success := transport.lastSendMessage(t)
	if success.Text != "Scanner Access granted to Ada (@ada, ID 200)." {
		t.Fatalf("success message = %q", success.Text)
	}
	if accessChanges != 1 {
		t.Fatalf("access change notifications = %d, want 1", accessChanges)
	}
	assertMenu(t, success)
	if got := transport.buttonRemovals(); got != 1 {
		t.Fatalf("confirmation button removals = %d, want 1", got)
	}

	service.ProcessUpdate(context.Background(), callbackUpdate(100, confirm))
	if got := transport.lastCallback(t).Text; got != "This action is no longer valid." {
		t.Fatalf("stale callback message = %q", got)
	}
	if got := transport.buttonRemovals(); got != 2 {
		t.Fatalf("stale confirmation button removals = %d, want 2", got)
	}
}

func TestAddUserCancelReturnsToMenu(t *testing.T) {
	transport := newTelegramTransport(t)
	service, err := telegrambot.New("123456:test-token", 100, &accessStore{}, slog.New(slog.DiscardHandler), func() {}, telegrambot.Options{ServerURL: transport.URL, Synchronous: true})
	if err != nil {
		t.Fatalf("create bot service: %v", err)
	}

	service.ProcessUpdate(context.Background(), messageUpdate(100, "Add user"))
	requestID := transport.lastSendMessage(t).ReplyMarkup.Keyboard[0][0].RequestUsers.RequestID
	service.ProcessUpdate(context.Background(), sharedUserUpdate(requestID, 200, "Ada", "ada"))
	cancel := transport.lastSendMessage(t).Inline.InlineKeyboard[0][1].CallbackData
	service.ProcessUpdate(context.Background(), callbackUpdate(100, cancel))

	outcome := transport.lastSendMessage(t)
	if outcome.Text != "No Scanner Access changes were made." {
		t.Fatalf("cancel message = %q", outcome.Text)
	}
	assertMenu(t, outcome)
	if got := transport.buttonRemovals(); got != 1 {
		t.Fatalf("confirmation button removals = %d, want 1", got)
	}
}

func TestAddUserGrantErrorReturnsToMenu(t *testing.T) {
	transport := newTelegramTransport(t)
	store := &accessStore{grantErr: errors.New("storage unavailable")}
	service, err := telegrambot.New("123456:test-token", 100, store, slog.New(slog.DiscardHandler), func() {}, telegrambot.Options{ServerURL: transport.URL, Synchronous: true})
	if err != nil {
		t.Fatalf("create bot service: %v", err)
	}

	service.ProcessUpdate(context.Background(), messageUpdate(100, "Add user"))
	requestID := transport.lastSendMessage(t).ReplyMarkup.Keyboard[0][0].RequestUsers.RequestID
	service.ProcessUpdate(context.Background(), sharedUserUpdate(requestID, 200, "Ada", "ada"))
	confirm := transport.lastSendMessage(t).Inline.InlineKeyboard[0][0].CallbackData
	service.ProcessUpdate(context.Background(), callbackUpdate(100, confirm))

	outcome := transport.lastSendMessage(t)
	if outcome.Text != "Scanner Access was not changed. Please try again." {
		t.Fatalf("error message = %q", outcome.Text)
	}
	assertMenu(t, outcome)
	if got := transport.buttonRemovals(); got != 1 {
		t.Fatalf("confirmation button removals = %d, want 1", got)
	}
}

func TestStaleAddUserSelectionReturnsToMenu(t *testing.T) {
	transport := newTelegramTransport(t)
	service, err := telegrambot.New("123456:test-token", 100, &accessStore{}, slog.New(slog.DiscardHandler), func() {}, telegrambot.Options{ServerURL: transport.URL, Synchronous: true})
	if err != nil {
		t.Fatalf("create bot service: %v", err)
	}

	service.ProcessUpdate(context.Background(), messageUpdate(100, "Add user"))
	requestID := transport.lastSendMessage(t).ReplyMarkup.Keyboard[0][0].RequestUsers.RequestID
	service.ProcessUpdate(context.Background(), sharedUserUpdate(requestID, 200, "Ada", "ada"))
	service.ProcessUpdate(context.Background(), sharedUserUpdate(requestID, 201, "Babbage", ""))

	outcome := transport.lastSendMessage(t)
	if outcome.Text != "This selection is no longer valid. Choose Add user again." {
		t.Fatalf("stale selection message = %q", outcome.Text)
	}
	assertMenu(t, outcome)
}

func TestNewRejectsTelegramInitializationFailure(t *testing.T) {
	transport := newTelegramTransport(t)
	transport.setGetMeResponse(`{"ok":false,"error_code":401,"description":"Unauthorized"}`)

	service, err := telegrambot.New("123456:test-token", 100, &accessStore{}, slog.New(slog.DiscardHandler), func() {}, telegrambot.Options{ServerURL: transport.URL, Synchronous: true})
	if err == nil {
		t.Fatal("New() error = nil, want Telegram initialization failure")
	}
	if service != nil {
		t.Fatal("New() service != nil after Telegram initialization failure")
	}
	if !strings.Contains(err.Error(), "create Telegram bot") || !strings.Contains(err.Error(), "unauthorized") {
		t.Fatalf("New() error = %v", err)
	}
	if strings.Contains(err.Error(), "test-token") {
		t.Fatalf("New() exposed the token in its error: %v", err)
	}
	if transport.getMeCallsCount() != 1 {
		t.Fatalf("getMe calls = %d, want 1", transport.getMeCallsCount())
	}
}

func TestAccessAdministrationIsPrivateAndAdministratorOnly(t *testing.T) {
	transport := newTelegramTransport(t)
	store := &accessStore{users: map[int64]auth.User{300: {ID: 1, TelegramID: 300}}}
	service, err := telegrambot.New("123456:test-token", 100, store, slog.New(slog.DiscardHandler), func() {}, telegrambot.Options{ServerURL: transport.URL, Synchronous: true})
	if err != nil {
		t.Fatalf("create bot service: %v", err)
	}

	service.ProcessUpdate(context.Background(), messageUpdate(300, "Add user"))
	if got := transport.lastSendMessage(t).Text; got != "Scanner Access is active. Open the existing Main Mini App from this bot's profile." {
		t.Fatalf("enabled user response = %q", got)
	}
	service.ProcessUpdate(context.Background(), messageUpdate(400, "Add user"))
	if got := transport.lastSendMessage(t).Text; got != "Access has not been granted. Contact the Administrator." {
		t.Fatalf("unknown user response = %q", got)
	}

	before := transport.messageCount()
	service.ProcessUpdate(context.Background(), &models.Update{Message: &models.Message{From: &models.User{ID: 100}, Chat: models.Chat{ID: -1000, Type: models.ChatTypeGroup}, Text: "Add user"}})
	if after := transport.messageCount(); after != before {
		t.Fatalf("group message disclosed a response: before=%d after=%d", before, after)
	}

	service.ProcessUpdate(context.Background(), callbackUpdate(100, "scanner-access:confirm:forged"))
	if got := transport.lastCallback(t).Text; got != "This action is no longer valid." {
		t.Fatalf("forged callback response = %q", got)
	}
}

func TestPriceAlertRechecksAccessAfterRateLimitWait(t *testing.T) {
	transport := newTelegramTransport(t)
	store := &synchronizedAccessStore{user: auth.User{ID: 1, TelegramID: 200}}
	service, err := telegrambot.New("123456:test-token", 100, store, slog.New(slog.DiscardHandler), func() {}, telegrambot.Options{ServerURL: transport.URL, Synchronous: true})
	if err != nil {
		t.Fatal(err)
	}
	fired := alerts.Fired{Alert: alerts.Alert{ID: 1, TelegramID: 200, Symbol: "BTCUSDT", Target: "100"}, Price: "100", EventTime: time.Now()}
	if err := service.SendPriceAlert(context.Background(), fired); err != nil {
		t.Fatalf("first SendPriceAlert: %v", err)
	}
	result := make(chan error, 1)
	go func() { result <- service.SendPriceAlert(context.Background(), fired) }()
	time.Sleep(100 * time.Millisecond)
	store.delete()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("notification sent after the user was deleted during limiter wait")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SendPriceAlert did not finish after limiter wait")
	}
	if got := transport.messageCount(); got != 1 {
		t.Fatalf("Telegram sendMessage requests = %d, want 1", got)
	}
}

type synchronizedAccessStore struct {
	mu      sync.Mutex
	user    auth.User
	deleted bool
}

func (store *synchronizedAccessStore) FindByTelegramID(_ context.Context, telegramID int64) (auth.User, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.deleted || store.user.TelegramID != telegramID {
		return auth.User{}, auth.ErrUserNotFound
	}
	return store.user, nil
}
func (store *synchronizedAccessStore) GrantAccess(context.Context, int64, string, string) (auth.User, bool, error) {
	return auth.User{}, false, nil
}
func (store *synchronizedAccessStore) delete() {
	store.mu.Lock()
	store.deleted = true
	store.mu.Unlock()
}

type accessStore struct {
	users    map[int64]auth.User
	next     int64
	grantErr error
}

func (store *accessStore) FindByTelegramID(_ context.Context, telegramID int64) (auth.User, error) {
	user, ok := store.users[telegramID]
	if !ok {
		return auth.User{}, auth.ErrUserNotFound
	}
	return user, nil
}

func (store *accessStore) GrantAccess(_ context.Context, telegramID int64, username, displayName string) (auth.User, bool, error) {
	if store.grantErr != nil {
		return auth.User{}, false, store.grantErr
	}
	if store.users == nil {
		store.users = map[int64]auth.User{}
	}
	if user, ok := store.users[telegramID]; ok {
		return user, false, nil
	}
	store.next++
	user := auth.User{ID: store.next, TelegramID: telegramID, Username: username, DisplayName: displayName}
	store.users[telegramID] = user
	return user, true, nil
}

type telegramTransport struct {
	t *testing.T
	*httptest.Server
	mu            sync.Mutex
	messages      []sendMessage
	callbacks     []answerCallback
	removals      int
	getMeResponse string
	getMeCalls    int
}

type sendMessage struct {
	ChatID          int64                       `json:"chat_id"`
	Text            string                      `json:"text"`
	ReplyMarkup     models.ReplyKeyboardMarkup  `json:"-"`
	Inline          models.InlineKeyboardMarkup `json:"-"`
	replyMarkupWire json.RawMessage
}

func (message *sendMessage) UnmarshalJSON(data []byte) error {
	var wire struct {
		ChatID      int64           `json:"chat_id"`
		Text        string          `json:"text"`
		ReplyMarkup json.RawMessage `json:"reply_markup"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	message.ChatID, message.Text = wire.ChatID, wire.Text
	message.replyMarkupWire = append(message.replyMarkupWire[:0], wire.ReplyMarkup...)
	if len(wire.ReplyMarkup) == 0 {
		return nil
	}
	if err := json.Unmarshal(wire.ReplyMarkup, &message.ReplyMarkup); err != nil {
		return err
	}
	return json.Unmarshal(wire.ReplyMarkup, &message.Inline)
}

type answerCallback struct {
	Text string `json:"text"`
}

func newTelegramTransport(t *testing.T) *telegramTransport {
	t.Helper()
	transport := &telegramTransport{t: t, getMeResponse: `{"ok":true,"result":{"id":123456,"is_bot":true,"first_name":"Scanner"}}`}
	transport.Server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/bot123456:test-token/getMe":
			transport.mu.Lock()
			transport.getMeCalls++
			getMeResponse := transport.getMeResponse
			transport.mu.Unlock()
			_, _ = response.Write([]byte(getMeResponse))
		case "/bot123456:test-token/sendMessage":
			if err := request.ParseMultipartForm(1 << 20); err != nil {
				t.Errorf("parse sendMessage form: %v", err)
			}
			message := sendMessage{Text: request.FormValue("text")}
			if _, err := fmt.Sscan(request.FormValue("chat_id"), &message.ChatID); err != nil {
				t.Errorf("parse chat ID: %v", err)
			}
			if markup := request.FormValue("reply_markup"); markup != "" {
				message.replyMarkupWire = append(message.replyMarkupWire, markup...)
				if err := json.Unmarshal([]byte(markup), &message.ReplyMarkup); err != nil {
					t.Errorf("decode reply keyboard: %v", err)
				}
				if err := json.Unmarshal([]byte(markup), &message.Inline); err != nil {
					t.Errorf("decode inline keyboard: %v", err)
				}
			}
			transport.mu.Lock()
			transport.messages = append(transport.messages, message)
			transport.mu.Unlock()
			_, _ = response.Write([]byte(`{"ok":true,"result":{"message_id":1,"date":0,"chat":{"id":100,"type":"private"}}}`))
		case "/bot123456:test-token/editMessageReplyMarkup":
			if err := request.ParseMultipartForm(1 << 20); err != nil {
				t.Errorf("parse editMessageReplyMarkup form: %v", err)
			}
			if request.FormValue("reply_markup") != "" {
				t.Errorf("confirmation edit kept a reply markup: %s", request.FormValue("reply_markup"))
			}
			transport.mu.Lock()
			transport.removals++
			transport.mu.Unlock()
			_, _ = response.Write([]byte(`{"ok":true,"result":true}`))
		case "/bot123456:test-token/answerCallbackQuery":
			if err := request.ParseMultipartForm(1 << 20); err != nil {
				t.Errorf("parse answerCallbackQuery form: %v", err)
			}
			callback := answerCallback{Text: request.FormValue("text")}
			transport.mu.Lock()
			transport.callbacks = append(transport.callbacks, callback)
			transport.mu.Unlock()
			_, _ = response.Write([]byte(`{"ok":true,"result":true}`))
		default:
			t.Errorf("unexpected Telegram request: %s", request.URL.Path)
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(transport.Close)
	return transport
}

func (transport *telegramTransport) lastSendMessage(t *testing.T) sendMessage {
	t.Helper()
	transport.mu.Lock()
	defer transport.mu.Unlock()
	if len(transport.messages) == 0 {
		t.Fatal("expected a sendMessage request")
	}
	return transport.messages[len(transport.messages)-1]
}

func (transport *telegramTransport) lastCallback(t *testing.T) answerCallback {
	t.Helper()
	transport.mu.Lock()
	defer transport.mu.Unlock()
	if len(transport.callbacks) == 0 {
		t.Fatal("expected an answerCallbackQuery request")
	}
	return transport.callbacks[len(transport.callbacks)-1]
}

func (transport *telegramTransport) setGetMeResponse(response string) {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	transport.getMeResponse = response
}

func (transport *telegramTransport) getMeCallsCount() int {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	return transport.getMeCalls
}

func (transport *telegramTransport) messageCount() int {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	return len(transport.messages)
}

func (transport *telegramTransport) buttonRemovals() int {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	return transport.removals
}

// assertMenu checks that a message returns the administrator to the menu,
// which also replaces the user picker keyboard.
func assertMenu(t *testing.T, message sendMessage) {
	t.Helper()
	if len(message.ReplyMarkup.Keyboard) != 1 || len(message.ReplyMarkup.Keyboard[0]) != 1 || message.ReplyMarkup.Keyboard[0][0].Text != "Add user" || message.ReplyMarkup.Keyboard[0][0].RequestUsers != nil {
		t.Fatalf("reply markup = %s, want the Add user menu", message.replyMarkupWire)
	}
}

func sharedUserUpdate(requestID int32, userID int64, firstName, username string) *models.Update {
	return &models.Update{Message: &models.Message{
		From: &models.User{ID: 100}, Chat: models.Chat{ID: 100, Type: models.ChatTypePrivate},
		UsersShared: &models.UsersShared{RequestID: int(requestID), Users: []models.SharedUser{{UserID: userID, FirstName: firstName, Username: username}}},
	}}
}

func messageUpdate(userID int64, text string) *models.Update {
	return &models.Update{Message: &models.Message{From: &models.User{ID: userID}, Chat: models.Chat{ID: userID, Type: models.ChatTypePrivate}, Text: text}}
}

func callbackUpdate(userID int64, data string) *models.Update {
	return &models.Update{CallbackQuery: &models.CallbackQuery{ID: "callback", From: models.User{ID: userID}, Data: data, Message: models.MaybeInaccessibleMessage{Message: &models.Message{ID: 1, Chat: models.Chat{ID: userID, Type: models.ChatTypePrivate}}}}}
}
