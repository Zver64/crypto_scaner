// Package telegram verifies Telegram Mini App init data.
package telegram

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"crypto-scanner/internal/auth"
)

// futureSkew tolerates init data dated slightly ahead of this server's clock.
const futureSkew = 30 * time.Second

// Options exposes the time boundary used to validate init-data age.
type Options struct {
	Now func() time.Time
}

// Verifier checks the signature and age of Telegram Mini App init data.
type Verifier struct {
	// secretKey is HMAC-SHA-256("WebAppData", bot token), derived once.
	secretKey []byte
	maxAge    time.Duration
	now       func() time.Time
}

// New creates a verifier for init data issued to the bot with botToken at
// most maxAge ago; zero options use the system clock.
func New(botToken string, maxAge time.Duration, options Options) *Verifier {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	secretMAC := hmac.New(sha256.New, []byte("WebAppData"))
	_, _ = secretMAC.Write([]byte(botToken))
	return &Verifier{secretKey: secretMAC.Sum(nil), maxAge: maxAge, now: now}
}

// Verify returns the Telegram user ID of valid init data. It fails with
// auth.ErrUnauthenticated.
func (verifier *Verifier) Verify(raw string) (int64, error) {
	values, err := url.ParseQuery(raw)
	if err != nil || !singleValues(values) {
		return 0, auth.ErrUnauthenticated
	}
	receivedHash, err := hex.DecodeString(values.Get("hash"))
	if err != nil || len(receivedHash) != sha256.Size {
		return 0, auth.ErrUnauthenticated
	}
	dataMAC := hmac.New(sha256.New, verifier.secretKey)
	_, _ = dataMAC.Write([]byte(makeDataCheckString(values)))
	if !hmac.Equal(receivedHash, dataMAC.Sum(nil)) {
		return 0, auth.ErrUnauthenticated
	}
	authUnix, err := strconv.ParseInt(values.Get("auth_date"), 10, 64)
	if err != nil {
		return 0, auth.ErrUnauthenticated
	}
	age := verifier.now().Sub(time.Unix(authUnix, 0))
	if age < -futureSkew || age > verifier.maxAge {
		return 0, auth.ErrUnauthenticated
	}
	var telegramUser struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal([]byte(values.Get("user")), &telegramUser); err != nil || telegramUser.ID <= 0 {
		return 0, auth.ErrUnauthenticated
	}
	return telegramUser.ID, nil
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
