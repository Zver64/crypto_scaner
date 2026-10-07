package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
)

// APITokenPrefix starts every API token, so Authenticate tells them from
// session tokens and people recognize them.
const APITokenPrefix = "cst_"

// touchAfter is how old the recorded last use of an API token must be before
// a request records it again, so most requests write nothing.
const touchAfter = time.Minute

// ErrInvalidAPITokenName means that an API token name is blank.
var ErrInvalidAPITokenName = errors.New("API token name must not be blank")

// ErrAPITokenNotFound means that no stored API token matches a token or an
// identifier of its user.
var ErrAPITokenNotFound = errors.New("API token not found")

// APIToken describes a stored API token. LastUsedAt is zero until its first
// use and is recorded at most once a minute.
type APIToken struct {
	ID         int64
	Name       string
	CreatedAt  time.Time
	LastUsedAt time.Time
}

// IssuedAPIToken is a new API token. The token is returned only once.
type IssuedAPIToken struct {
	APIToken
	Token string
}

// NewAPIToken is an API token to store under the SHA-256 of its token.
type NewAPIToken struct {
	UserID    int64
	Name      string
	TokenHash []byte
	CreatedAt time.Time
}

// StoredAPIToken is a stored API token with its user.
type StoredAPIToken struct {
	ID         int64
	User       User
	LastUsedAt time.Time
}

// APITokenStore persists API tokens by token hash.
type APITokenStore interface {
	CreateAPIToken(context.Context, NewAPIToken) (int64, error)
	// ListAPITokens returns the tokens of a user, newest first.
	ListAPITokens(ctx context.Context, userID int64) ([]APIToken, error)
	// DeleteAPIToken fails with ErrAPITokenNotFound unless the user owns the
	// token.
	DeleteAPIToken(ctx context.Context, userID, id int64) error
	// FindAPIToken fails with ErrAPITokenNotFound.
	FindAPIToken(ctx context.Context, tokenHash []byte) (StoredAPIToken, error)
	// TouchAPIToken records usedAt as the last use only if none is recorded
	// or the recorded one is at least minimumInterval older.
	TouchAPIToken(ctx context.Context, id int64, usedAt time.Time, minimumInterval time.Duration) error
}

// CreateAPIToken issues an API token of a user under the trimmed name. The
// token authenticates like a session of that user until it is deleted,
// without expiring. It fails with ErrInvalidAPITokenName for a blank name.
func (sessions *Sessions) CreateAPIToken(ctx context.Context, userID int64, name string) (IssuedAPIToken, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return IssuedAPIToken{}, ErrInvalidAPITokenName
	}
	raw := make([]byte, tokenBytes)
	_, _ = rand.Read(raw)
	token := APITokenPrefix + base64.RawURLEncoding.EncodeToString(raw)
	now := sessions.now()
	id, err := sessions.store.CreateAPIToken(ctx, NewAPIToken{UserID: userID, Name: name, TokenHash: hashToken(token), CreatedAt: now})
	if err != nil {
		return IssuedAPIToken{}, fmt.Errorf("create API token: %w", err)
	}
	return IssuedAPIToken{APIToken: APIToken{ID: id, Name: name, CreatedAt: now}, Token: token}, nil
}

// ListAPITokens returns the API tokens of a user, newest first.
func (sessions *Sessions) ListAPITokens(ctx context.Context, userID int64) ([]APIToken, error) {
	tokens, err := sessions.store.ListAPITokens(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list API tokens: %w", err)
	}
	return tokens, nil
}

// DeleteAPIToken revokes an API token of a user. It fails with
// ErrAPITokenNotFound for tokens of other users.
func (sessions *Sessions) DeleteAPIToken(ctx context.Context, userID, id int64) error {
	if err := sessions.store.DeleteAPIToken(ctx, userID, id); err != nil {
		return fmt.Errorf("delete API token: %w", err)
	}
	return nil
}

// authenticateAPIToken authenticates a well-formed API token.
func (sessions *Sessions) authenticateAPIToken(ctx context.Context, token string) (User, error) {
	stored, err := sessions.store.FindAPIToken(ctx, hashToken(token))
	if errors.Is(err, ErrAPITokenNotFound) {
		return User{}, ErrUnauthenticated
	}
	if err != nil {
		return User{}, fmt.Errorf("find API token: %w", err)
	}
	if now := sessions.now(); now.Sub(stored.LastUsedAt) >= touchAfter {
		// The last use is informational, so failing to record it never
		// refuses a valid token.
		if err := sessions.store.TouchAPIToken(ctx, stored.ID, now, touchAfter); err != nil {
			sessions.logger.WarnContext(ctx, "API token last use not recorded", "operation", "touch_api_token", "outcome", "failure", "api_token_id", stored.ID, "error", err)
		}
	}
	user := sessions.withRole(stored.User)
	user.APIToken = true
	return user, nil
}
