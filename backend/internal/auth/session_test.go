package auth

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

type sessionStoreFake struct {
	SessionStore
	session  Session
	extended time.Time
}

func (store *sessionStoreFake) FindSession(context.Context, []byte) (Session, error) {
	return store.session, nil
}

func (store *sessionStoreFake) ExtendSession(_ context.Context, _ []byte, idleExpiresAt time.Time, _ time.Duration) error {
	store.extended = idleExpiresAt
	return nil
}

// Sessions expire when idle or at their absolute expiry, and use moves the
// idle expiry forward but never past the absolute one.
func TestSessionsAuthenticateExpiry(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name           string
		idle, absolute time.Time
		wantErr        error
		wantExtended   time.Time
	}{
		{name: "idle expired", idle: now, absolute: now.Add(10 * time.Hour), wantErr: ErrUnauthenticated},
		{name: "absolute expired", idle: now.Add(time.Hour), absolute: now, wantErr: ErrUnauthenticated},
		{name: "extends idle expiry", idle: now.Add(10 * time.Minute), absolute: now.Add(10 * time.Hour), wantExtended: now.Add(time.Hour)},
		{name: "extension capped at absolute expiry", idle: now.Add(10 * time.Minute), absolute: now.Add(30 * time.Minute), wantExtended: now.Add(30 * time.Minute)},
		{name: "recent extension skipped", idle: now.Add(time.Hour - extendAfter/2), absolute: now.Add(10 * time.Hour)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &sessionStoreFake{session: Session{User: User{ID: 1}, IdleExpiresAt: test.idle, AbsoluteExpiresAt: test.absolute}}
			sessions := NewSessions(store, nil, 0, time.Hour, 24*time.Hour, slog.New(slog.DiscardHandler), SessionOptions{Now: func() time.Time { return now }})
			_, err := sessions.Authenticate(t.Context(), strings.Repeat("A", 43))
			if !errors.Is(err, test.wantErr) || !store.extended.Equal(test.wantExtended) {
				t.Fatalf("Authenticate() error = %v, extended to %v; want %v, %v", err, store.extended, test.wantErr, test.wantExtended)
			}
		})
	}
}
