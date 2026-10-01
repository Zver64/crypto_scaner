package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"crypto-scanner/internal/platform/backoff"
)

const (
	tokenBytes = 32
	// MaxSessionsPerUser bounds the sessions kept per user; a new session
	// replaces the oldest beyond it.
	MaxSessionsPerUser = 10
	// extendAfter is how much idle lifetime a session must have used before a
	// request moves its idle expiry, so most requests write nothing.
	extendAfter  = time.Minute
	prunePeriod  = time.Hour
	pruneTimeout = 30 * time.Second
)

// ErrSessionNotFound means that no stored session matches a token.
var ErrSessionNotFound = errors.New("session not found")

// InitDataVerifier verifies Telegram Mini App init data and returns the
// Telegram user ID it was issued for. It fails with ErrUnauthenticated.
type InitDataVerifier interface {
	Verify(rawInitData string) (int64, error)
}

// Session is a stored session with its user.
type Session struct {
	User              User
	IdleExpiresAt     time.Time
	AbsoluteExpiresAt time.Time
}

// NewSession is a session to store under the SHA-256 of its token.
type NewSession struct {
	TokenHash         []byte
	UserID            int64
	CreatedAt         time.Time
	IdleExpiresAt     time.Time
	AbsoluteExpiresAt time.Time
}

// SessionStore persists sessions by token hash.
type SessionStore interface {
	UserStore
	// CreateSession stores a session and keeps at most MaxSessionsPerUser
	// sessions of its user.
	CreateSession(context.Context, NewSession) error
	// FindSession fails with ErrSessionNotFound.
	FindSession(ctx context.Context, tokenHash []byte) (Session, error)
	// ExtendSession moves the idle expiry to idleExpiresAt only if that
	// extends it by at least minimumExtension.
	ExtendSession(ctx context.Context, tokenHash []byte, idleExpiresAt time.Time, minimumExtension time.Duration) error
	DeleteSession(ctx context.Context, tokenHash []byte) error
	DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error)
}

// IssuedSession is a new session token. The token is returned only once.
type IssuedSession struct {
	Token     string
	ExpiresAt time.Time
}

// SessionOptions exposes the clock used for session lifetimes.
type SessionOptions struct {
	Now func() time.Time
}

// Sessions exchanges verified Telegram init data for opaque session tokens and
// authenticates requests by them. Tokens expire after idleTTL without use and
// after absoluteTTL in any case.
type Sessions struct {
	store           SessionStore
	verifier        InitDataVerifier
	administratorID int64
	idleTTL         time.Duration
	absoluteTTL     time.Duration
	logger          *slog.Logger
	now             func() time.Time
}

// NewSessions creates the session service. administratorID is the Telegram ID
// of the scanner administrator; zero options use the system clock.
func NewSessions(store SessionStore, verifier InitDataVerifier, administratorID int64, idleTTL, absoluteTTL time.Duration, logger *slog.Logger, options SessionOptions) *Sessions {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Sessions{store: store, verifier: verifier, administratorID: administratorID, idleTTL: idleTTL, absoluteTTL: absoluteTTL,
		logger: logger.With("module", "auth"), now: now}
}

// Exchange verifies init data and issues a session for its application user.
// It fails with ErrUnauthenticated or ErrAccessDenied.
func (sessions *Sessions) Exchange(ctx context.Context, rawInitData string) (IssuedSession, error) {
	telegramID, err := sessions.verifier.Verify(rawInitData)
	if err != nil {
		return IssuedSession{}, err
	}
	user, err := sessions.store.FindByTelegramID(ctx, telegramID)
	if errors.Is(err, ErrUserNotFound) {
		return IssuedSession{}, ErrAccessDenied
	}
	if err != nil {
		return IssuedSession{}, err
	}
	raw := make([]byte, tokenBytes)
	_, _ = rand.Read(raw)
	token := base64.RawURLEncoding.EncodeToString(raw)
	now := sessions.now()
	absolute := now.Add(sessions.absoluteTTL)
	if err := sessions.store.CreateSession(ctx, NewSession{
		TokenHash:         hashToken(token),
		UserID:            user.ID,
		CreatedAt:         now,
		IdleExpiresAt:     earliest(now.Add(sessions.idleTTL), absolute),
		AbsoluteExpiresAt: absolute,
	}); err != nil {
		return IssuedSession{}, fmt.Errorf("create session: %w", err)
	}
	return IssuedSession{Token: token, ExpiresAt: absolute}, nil
}

// Authenticate returns the user of a live session and extends its idle
// expiry. It fails with ErrUnauthenticated, also once the user is deleted.
func (sessions *Sessions) Authenticate(ctx context.Context, token string) (User, error) {
	if !wellFormedToken(token) {
		return User{}, ErrUnauthenticated
	}
	tokenHash := hashToken(token)
	session, err := sessions.store.FindSession(ctx, tokenHash)
	if errors.Is(err, ErrSessionNotFound) {
		return User{}, ErrUnauthenticated
	}
	if err != nil {
		return User{}, fmt.Errorf("find session: %w", err)
	}
	now := sessions.now()
	if !now.Before(session.IdleExpiresAt) || !now.Before(session.AbsoluteExpiresAt) {
		return User{}, ErrUnauthenticated
	}
	if idle := earliest(now.Add(sessions.idleTTL), session.AbsoluteExpiresAt); idle.Sub(session.IdleExpiresAt) >= extendAfter {
		if err := sessions.store.ExtendSession(ctx, tokenHash, idle, extendAfter); err != nil {
			return User{}, fmt.Errorf("extend session: %w", err)
		}
	}
	user := session.User
	user.Administrator = user.TelegramID == sessions.administratorID
	return user, nil
}

// Revoke deletes the session of a token; unknown tokens are ignored.
func (sessions *Sessions) Revoke(ctx context.Context, token string) error {
	if !wellFormedToken(token) {
		return nil
	}
	if err := sessions.store.DeleteSession(ctx, hashToken(token)); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// Run deletes expired sessions hourly until ctx is cancelled.
func (sessions *Sessions) Run(ctx context.Context) error {
	for {
		if backoff.Sleep(ctx, prunePeriod) != nil {
			return nil
		}
		pruneCtx, cancel := context.WithTimeout(ctx, pruneTimeout)
		deleted, err := sessions.store.DeleteExpiredSessions(pruneCtx, sessions.now())
		cancel()
		switch {
		case ctx.Err() != nil:
			return nil
		case err != nil:
			sessions.logger.WarnContext(ctx, "expired session pruning failed", "operation", "prune_sessions", "outcome", "failure", "error", err)
		case deleted > 0:
			sessions.logger.InfoContext(ctx, "expired sessions pruned", "operation", "prune_sessions", "outcome", "success", "deleted", deleted)
		}
	}
}

func earliest(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func wellFormedToken(token string) bool {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(raw) == tokenBytes
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
