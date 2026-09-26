// Package telegram verifies Telegram Mini App init data.
package telegram

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"crypto-scanner/internal/auth"
)

// Options exposes the time boundary used to validate init-data age.
type Options struct {
	Now func() time.Time
}

// Authenticator verifies Telegram Mini App init data and authorizes the
// Telegram identity against the application user store.
type Authenticator struct {
	store    auth.UserStore
	botToken string
	maxAge   time.Duration
	now      func() time.Time
}

// New creates an authenticator; zero options use the system clock.
func New(store auth.UserStore, botToken string, maxAge time.Duration, options Options) *Authenticator {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Authenticator{store: store, botToken: botToken, maxAge: maxAge, now: now}
}

// AuthenticateInitData verifies raw init data and returns the enabled user.
// It fails with auth.ErrUnauthenticated or auth.ErrAccessDenied.
func (authenticator *Authenticator) AuthenticateInitData(ctx context.Context, rawInitData string) (auth.User, error) {
	telegramID, ok := authenticator.validate(rawInitData)
	if !ok {
		return auth.User{}, auth.ErrUnauthenticated
	}
	user, err := authenticator.store.FindEnabledByTelegramID(ctx, telegramID)
	if errors.Is(err, auth.ErrUserNotFound) || err == nil && !user.Enabled {
		return auth.User{}, auth.ErrAccessDenied
	}
	if err != nil {
		return auth.User{}, err
	}
	return user, nil
}

func (authenticator *Authenticator) validate(raw string) (int64, bool) {
	values, err := url.ParseQuery(raw)
	if err != nil || !singleValues(values) {
		return 0, false
	}
	receivedHash, err := hex.DecodeString(values.Get("hash"))
	if err != nil || len(receivedHash) != sha256.Size {
		return 0, false
	}
	dataCheckString := makeDataCheckString(values)
	secretMAC := hmac.New(sha256.New, []byte("WebAppData"))
	_, _ = secretMAC.Write([]byte(authenticator.botToken))
	dataMAC := hmac.New(sha256.New, secretMAC.Sum(nil))
	_, _ = dataMAC.Write([]byte(dataCheckString))
	if !hmac.Equal(receivedHash, dataMAC.Sum(nil)) {
		return 0, false
	}
	authUnix, err := strconv.ParseInt(values.Get("auth_date"), 10, 64)
	if err != nil {
		return 0, false
	}
	authTime := time.Unix(authUnix, 0)
	age := authenticator.now().Sub(authTime)
	if age < 0 || age > authenticator.maxAge {
		return 0, false
	}
	var telegramUser struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal([]byte(values.Get("user")), &telegramUser); err != nil || telegramUser.ID <= 0 {
		return 0, false
	}
	return telegramUser.ID, true
}

func singleValues(values url.Values) bool {
	if len(values) == 0 {
		return false
	}
	for key, items := range values {
		if key == "" || len(items) != 1 {
			return false
		}
	}
	return true
}

func makeDataCheckString(values url.Values) string {
	keys := make([]string, 0, len(values)-1)
	for key := range values {
		if key != "hash" {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+values.Get(key))
	}
	return strings.Join(parts, "\n")
}
