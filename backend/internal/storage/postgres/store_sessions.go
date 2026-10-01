package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"crypto-scanner/internal/auth"
	generated "crypto-scanner/internal/storage/postgres/sqlc"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// CreateSession stores a session and keeps at most auth.MaxSessionsPerUser
// sessions of its user, deleting the oldest.
func (store *Store) CreateSession(ctx context.Context, session auth.NewSession) error {
	err := store.queries.CreateSession(ctx, generated.CreateSessionParams{
		TokenHash:         session.TokenHash,
		UserID:            session.UserID,
		CreatedAt:         pgtype.Timestamptz{Time: session.CreatedAt, Valid: true},
		IdleExpiresAt:     pgtype.Timestamptz{Time: session.IdleExpiresAt, Valid: true},
		AbsoluteExpiresAt: pgtype.Timestamptz{Time: session.AbsoluteExpiresAt, Valid: true},
		KeepExisting:      auth.MaxSessionsPerUser - 1,
	})
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (store *Store) FindSession(ctx context.Context, tokenHash []byte) (auth.Session, error) {
	row, err := store.queries.FindSession(ctx, tokenHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.Session{}, auth.ErrSessionNotFound
	}
	if err != nil {
		return auth.Session{}, fmt.Errorf("find session: %w", err)
	}
	return auth.Session{
		User:              userFromColumns(row.AppUser.ID, row.AppUser.TelegramID, row.AppUser.Username, row.AppUser.DisplayName),
		IdleExpiresAt:     row.IdleExpiresAt.Time,
		AbsoluteExpiresAt: row.AbsoluteExpiresAt.Time,
	}, nil
}

func (store *Store) ExtendSession(ctx context.Context, tokenHash []byte, idleExpiresAt time.Time, minimumExtension time.Duration) error {
	if err := store.queries.ExtendSession(ctx, generated.ExtendSessionParams{
		TokenHash:     tokenHash,
		IdleExpiresAt: pgtype.Timestamptz{Time: idleExpiresAt, Valid: true},
		ExtendBefore:  pgtype.Timestamptz{Time: idleExpiresAt.Add(-minimumExtension), Valid: true},
	}); err != nil {
		return fmt.Errorf("extend session: %w", err)
	}
	return nil
}

func (store *Store) DeleteSession(ctx context.Context, tokenHash []byte) error {
	if err := store.queries.DeleteSession(ctx, tokenHash); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// DeleteExpiredSessions deletes every session expired at now.
func (store *Store) DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error) {
	deleted, err := store.queries.DeleteExpiredSessions(ctx, pgtype.Timestamptz{Time: now, Valid: true})
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}
	return deleted, nil
}
