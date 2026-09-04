package telegrambot_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	"crypto-scanner/internal/auth"
	"crypto-scanner/internal/telegrambot"

	"github.com/go-telegram/bot/models"
)

func TestAdministratorCanAddAndConfirmUserAccess(t *testing.T) {
	transport := newTelegramTransport(t)
	store := &accessStore{}
	service, err := telegrambot.New("123456:test-token", 100, store, telegrambot.Options{ServerURL: transport.URL, Synchronous: true})
	if err != nil {
		t.Fatalf("create bot service: %v", err)
	}

	service.ProcessUpdate(context.Background(), messageUpdate(100, "/start"))
	start := transport.lastSendMessage(t)
	if start.Text != "Administrator menu:" || start.ReplyMarkup.Keyboard[0][0].Text != "List users" || start.ReplyMarkup.Keyboard[1][0].Text != "Add user" || start.ReplyMarkup.Keyboard[2][0].Text != "Delete user" {
		t.Fatalf("unexpected administrator menu: %#v", start)
	}

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
	if _, err := store.FindEnabledByTelegramID(context.Background(), 200); err != nil {
		t.Fatalf("confirmed user has no access: %v", err)
	}
	if _, err := store.FindEnabledByTelegramID(context.Background(), 201); err != auth.ErrUserNotFound {
		t.Fatalf("replayed picker selection changed confirmed identity: %v", err)
	}
	if got := transport.lastSendMessage(t).Text; got != "Scanner Access granted to Ada (@ada, ID 200)." {
		t.Fatalf("success message = %q", got)
	}

	service.ProcessUpdate(context.Background(), callbackUpdate(100, confirm))
	if got := transport.lastCallback(t).Text; got != "This action is no longer valid." {
		t.Fatalf("stale callback message = %q", got)
	}
}

func TestAccessAdministrationIsPrivateAndAdministratorOnly(t *testing.T) {
	transport := newTelegramTransport(t)
	store := &accessStore{users: map[int64]auth.User{300: {ID: 1, TelegramID: 300, Enabled: true}}}
	service, err := telegrambot.New("123456:test-token", 100, store, telegrambot.Options{ServerURL: transport.URL, Synchronous: true})
	if err != nil {
		t.Fatalf("create bot service: %v", err)
	}

	service.ProcessUpdate(context.Background(), messageUpdate(300, "List users"))
	if got := transport.lastSendMessage(t).Text; got != "Scanner Access is active. Open the existing Main Mini App from this bot's profile." {
		t.Fatalf("enabled user response = %q", got)
	}
	service.ProcessUpdate(context.Background(), messageUpdate(400, "List users"))
	if got := transport.lastSendMessage(t).Text; got != "Access has not been granted. Contact the Administrator." {
		t.Fatalf("unknown user response = %q", got)
	}

	before := transport.messageCount()
	service.ProcessUpdate(context.Background(), &models.Update{Message: &models.Message{From: &models.User{ID: 100}, Chat: models.Chat{ID: -1000, Type: models.ChatTypeGroup}, Text: "List users"}})
	if after := transport.messageCount(); after != before {
		t.Fatalf("group message disclosed a response: before=%d after=%d", before, after)
	}

	service.ProcessUpdate(context.Background(), callbackUpdate(100, "scanner-access:delete:forged"))
	if got := transport.lastCallback(t).Text; got != "This action is no longer valid." {
		t.Fatalf("forged callback response = %q", got)
	}
}

func TestAdministratorCanConfirmUserDeletion(t *testing.T) {
	transport := newTelegramTransport(t)
	store := &accessStore{users: map[int64]auth.User{
		100: {ID: 1, TelegramID: 100, DisplayName: "Administrator", Enabled: true},
		200: {ID: 2, TelegramID: 200, DisplayName: "Taylor", Enabled: true},
	}}
	service, err := telegrambot.New("123456:test-token", 100, store, telegrambot.Options{ServerURL: transport.URL, Synchronous: true})
	if err != nil {
		t.Fatalf("create bot service: %v", err)
	}

	service.ProcessUpdate(context.Background(), messageUpdate(100, "Delete user"))
	selection := transport.lastSendMessage(t)
	if selection.Text != "Choose a user to remove from Scanner Access." || len(selection.Inline.InlineKeyboard) != 1 || selection.Inline.InlineKeyboard[0][0].Text != "Taylor (ID 200)" {
		t.Fatalf("unexpected deletion selection: %#v", selection)
	}
	service.ProcessUpdate(context.Background(), callbackUpdate(100, selection.Inline.InlineKeyboard[0][0].CallbackData))
	confirmation := transport.lastSendMessage(t)
	if confirmation.Text != "Remove Scanner Access from Taylor (ID 200)?" {
		t.Fatalf("unexpected deletion confirmation: %#v", confirmation)
	}
	confirm := confirmation.Inline.InlineKeyboard[0][0].CallbackData
	service.ProcessUpdate(context.Background(), callbackUpdate(100, confirm))
	if _, err := store.FindEnabledByTelegramID(context.Background(), 200); err != auth.ErrUserNotFound {
		t.Fatalf("deleted user still has access: %v", err)
	}
	if got := transport.lastSendMessage(t).Text; got != "Scanner Access removed from Taylor (ID 200)." {
		t.Fatalf("deletion success message = %q", got)
	}
	service.ProcessUpdate(context.Background(), callbackUpdate(100, confirm))
	if got := transport.lastCallback(t).Text; got != "This action is no longer valid." {
		t.Fatalf("stale deletion callback response = %q", got)
	}
}

func TestDeletionInvalidatesOlderAddConfirmation(t *testing.T) {
	transport := newTelegramTransport(t)
	store := &accessStore{users: map[int64]auth.User{
		100: {ID: 1, TelegramID: 100, DisplayName: "Administrator", Enabled: true},
		200: {ID: 2, TelegramID: 200, DisplayName: "Taylor", Enabled: true},
	}}
	service, err := telegrambot.New("123456:test-token", 100, store, telegrambot.Options{ServerURL: transport.URL, Synchronous: true})
	if err != nil {
		t.Fatalf("create bot service: %v", err)
	}

	service.ProcessUpdate(context.Background(), messageUpdate(100, "Add user"))
	requestID := transport.lastSendMessage(t).ReplyMarkup.Keyboard[0][0].RequestUsers.RequestID
	service.ProcessUpdate(context.Background(), &models.Update{Message: &models.Message{
		From: &models.User{ID: 100}, Chat: models.Chat{ID: 100, Type: models.ChatTypePrivate},
		UsersShared: &models.UsersShared{RequestID: int(requestID), Users: []models.SharedUser{{UserID: 200, FirstName: "Taylor"}}},
	}})
	staleAddConfirmation := transport.lastSendMessage(t).Inline.InlineKeyboard[0][0].CallbackData

	service.ProcessUpdate(context.Background(), callbackUpdate(100, "scanner-access:delete-page:8"))
	service.ProcessUpdate(context.Background(), messageUpdate(100, "Delete user"))
	deleteSelection := transport.lastSendMessage(t).Inline.InlineKeyboard[0][0].CallbackData
	service.ProcessUpdate(context.Background(), callbackUpdate(100, deleteSelection))
	deleteConfirmation := transport.lastSendMessage(t).Inline.InlineKeyboard[0][0].CallbackData
	service.ProcessUpdate(context.Background(), callbackUpdate(100, deleteConfirmation))
	service.ProcessUpdate(context.Background(), callbackUpdate(100, staleAddConfirmation))

	if _, err := store.FindEnabledByTelegramID(context.Background(), 200); err != auth.ErrUserNotFound {
		t.Fatalf("stale add confirmation restored deleted access: %v", err)
	}
	if got := transport.lastCallback(t).Text; got != "This action is no longer valid." {
		t.Fatalf("stale add callback response = %q", got)
	}
}

func TestEmptyUserListIdentifiesAdministrator(t *testing.T) {
	transport := newTelegramTransport(t)
	store := &accessStore{users: map[int64]auth.User{100: {ID: 1, TelegramID: 100, DisplayName: "Administrator", Enabled: true}}}
	service, err := telegrambot.New("123456:test-token", 100, store, telegrambot.Options{ServerURL: transport.URL, Synchronous: true})
	if err != nil {
		t.Fatalf("create bot service: %v", err)
	}
	service.ProcessUpdate(context.Background(), messageUpdate(100, "List users"))
	if got := transport.lastSendMessage(t).Text; got != "Scanner Access users:\nAdministrator — Administrator (ID 100)\nNo other users have Scanner Access." {
		t.Fatalf("empty list = %q", got)
	}
}

func TestAdministratorCanNavigateLongUserList(t *testing.T) {
	transport := newTelegramTransport(t)
	store := &accessStore{users: map[int64]auth.User{100: {ID: 1, TelegramID: 100, DisplayName: "Administrator", Enabled: true}}}
	for index := int64(0); index < 9; index++ {
		telegramID := 200 + index
		store.users[telegramID] = auth.User{ID: telegramID, TelegramID: telegramID, DisplayName: fmt.Sprintf("User %d", index+1), Enabled: true}
	}
	service, err := telegrambot.New("123456:test-token", 100, store, telegrambot.Options{ServerURL: transport.URL, Synchronous: true})
	if err != nil {
		t.Fatalf("create bot service: %v", err)
	}

	service.ProcessUpdate(context.Background(), messageUpdate(100, "List users"))
	firstPage := transport.lastSendMessage(t)
	if !strings.Contains(firstPage.Text, "Administrator") || !strings.Contains(firstPage.Text, "User 8") || strings.Contains(firstPage.Text, "User 9") || firstPage.Inline.InlineKeyboard[0][0].Text != "Next" {
		t.Fatalf("unexpected first page: %#v", firstPage)
	}
	service.ProcessUpdate(context.Background(), callbackUpdate(100, firstPage.Inline.InlineKeyboard[0][0].CallbackData))
	secondPage := transport.lastSendMessage(t)
	if !strings.Contains(secondPage.Text, "User 9") || secondPage.Inline.InlineKeyboard[0][0].Text != "Previous" {
		t.Fatalf("unexpected second page: %#v", secondPage)
	}
}

type accessStore struct {
	users map[int64]auth.User
	next  int64
}

func (store *accessStore) FindEnabledByTelegramID(_ context.Context, telegramID int64) (auth.User, error) {
	user, ok := store.users[telegramID]
	if !ok || !user.Enabled {
		return auth.User{}, auth.ErrUserNotFound
	}
	return user, nil
}

func (store *accessStore) GrantAccess(_ context.Context, telegramID int64, username, displayName string) (auth.User, bool, error) {
	if store.users == nil {
		store.users = map[int64]auth.User{}
	}
	if user, ok := store.users[telegramID]; ok && user.Enabled {
		return user, false, nil
	}
	store.next++
	user := auth.User{ID: store.next, TelegramID: telegramID, Username: username, DisplayName: displayName, Enabled: true}
	store.users[telegramID] = user
	return user, true, nil
}

func (store *accessStore) ListNonAdministratorUsers(_ context.Context, administratorID int64, offset, limit int) ([]auth.User, error) {
	users := make([]auth.User, 0, len(store.users))
	for _, user := range store.users {
		if user.Enabled && user.TelegramID != administratorID {
			users = append(users, user)
		}
	}
	sort.Slice(users, func(left, right int) bool { return users[left].TelegramID < users[right].TelegramID })
	if offset >= len(users) {
		return nil, nil
	}
	end := min(offset+limit, len(users))
	return users[offset:end], nil
}

func (store *accessStore) DeleteUser(_ context.Context, id, telegramID int64) (bool, error) {
	user, ok := store.users[telegramID]
	if !ok || user.ID != id {
		return false, nil
	}
	delete(store.users, telegramID)
	return true, nil
}

type telegramTransport struct {
	t *testing.T
	*httptest.Server
	mu        sync.Mutex
	messages  []sendMessage
	callbacks []answerCallback
}

type sendMessage struct {
	ChatID      int64                       `json:"chat_id"`
	Text        string                      `json:"text"`
	ReplyMarkup models.ReplyKeyboardMarkup  `json:"-"`
	Inline      models.InlineKeyboardMarkup `json:"-"`
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
	transport := &telegramTransport{t: t}
	transport.Server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/bot123456:test-token/sendMessage":
			if err := request.ParseMultipartForm(1 << 20); err != nil {
				t.Errorf("parse sendMessage form: %v", err)
			}
			message := sendMessage{Text: request.FormValue("text")}
			if _, err := fmt.Sscan(request.FormValue("chat_id"), &message.ChatID); err != nil {
				t.Errorf("parse chat ID: %v", err)
			}
			if markup := request.FormValue("reply_markup"); markup != "" {
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

func (transport *telegramTransport) messageCount() int {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	return len(transport.messages)
}

func messageUpdate(userID int64, text string) *models.Update {
	return &models.Update{Message: &models.Message{From: &models.User{ID: userID}, Chat: models.Chat{ID: userID, Type: models.ChatTypePrivate}, Text: text}}
}

func callbackUpdate(userID int64, data string) *models.Update {
	return &models.Update{CallbackQuery: &models.CallbackQuery{ID: "callback", From: models.User{ID: userID}, Data: data, Message: models.MaybeInaccessibleMessage{Message: &models.Message{Chat: models.Chat{ID: userID, Type: models.ChatTypePrivate}}}}}
}
