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

func (store *Store) CreateAPIToken(ctx context.Context, token auth.NewAPIToken) (int64, error) {
	id, err := store.queries.CreateAPIToken(ctx, generated.CreateAPITokenParams{
		UserID:    token.UserID,
		Name:      token.Name,
		TokenHash: token.TokenHash,
		CreatedAt: pgtype.Timestamptz{Time: token.CreatedAt, Valid: true},
	})
	if err != nil {
		return 0, fmt.Errorf("create API token: %w", err)
	}
	return id, nil
}

// ListAPITokens returns the API tokens of a user, newest first.
func (store *Store) ListAPITokens(ctx context.Context, userID int64) ([]auth.APIToken, error) {
	rows, err := store.queries.ListAPITokens(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list API tokens: %w", err)
	}
	tokens := make([]auth.APIToken, len(rows))
	for i, row := range rows {
		tokens[i] = auth.APIToken{ID: row.ID, Name: row.Name, CreatedAt: row.CreatedAt.Time, LastUsedAt: row.LastUsedAt.Time}
	}
	return tokens, nil
}

// DeleteAPIToken fails with auth.ErrAPITokenNotFound unless the user owns
// the token.
func (store *Store) DeleteAPIToken(ctx context.Context, userID, id int64) error {
	deleted, err := store.queries.DeleteAPIToken(ctx, generated.DeleteAPITokenParams{ID: id, UserID: userID})
	if err != nil {
		return fmt.Errorf("delete API token: %w", err)
	}
	if deleted == 0 {
		return auth.ErrAPITokenNotFound
	}
	return nil
}

func (store *Store) FindAPIToken(ctx context.Context, tokenHash []byte) (auth.StoredAPIToken, error) {
	row, err := store.queries.FindAPIToken(ctx, tokenHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.StoredAPIToken{}, auth.ErrAPITokenNotFound
	}
	if err != nil {
		return auth.StoredAPIToken{}, fmt.Errorf("find API token: %w", err)
	}
	return auth.StoredAPIToken{
		ID:         row.ID,
		User:       userFromColumns(row.AppUser.ID, row.AppUser.TelegramID, row.AppUser.Username, row.AppUser.DisplayName),
		LastUsedAt: row.LastUsedAt.Time,
	}, nil
}

func (store *Store) TouchAPIToken(ctx context.Context, id int64, usedAt time.Time, minimumInterval time.Duration) error {
	if err := store.queries.TouchAPIToken(ctx, generated.TouchAPITokenParams{
		ID:          id,
		UsedAt:      pgtype.Timestamptz{Time: usedAt, Valid: true},
		TouchBefore: pgtype.Timestamptz{Time: usedAt.Add(-minimumInterval), Valid: true},
	}); err != nil {
		return fmt.Errorf("touch API token: %w", err)
	}
	return nil
}
