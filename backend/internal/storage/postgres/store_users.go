package postgres

import (
	"context"
	"errors"
	"fmt"

	"crypto-scanner/internal/auth"
	generated "crypto-scanner/internal/storage/postgres/sqlc"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (store *Store) FindByTelegramID(ctx context.Context, telegramID int64) (auth.User, error) {
	row, err := store.queries.FindUserByTelegramID(ctx, telegramID)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, auth.ErrUserNotFound
	}
	if err != nil {
		return auth.User{}, fmt.Errorf("find user by Telegram ID: %w", err)
	}
	return userFromColumns(row.ID, row.TelegramID, row.Username, row.DisplayName), nil
}

// GrantAccess creates an application user. The boolean reports whether Scanner
// Access changed, so repeated additions remain harmless.
func (store *Store) GrantAccess(ctx context.Context, telegramID int64, username, displayName string) (auth.User, bool, error) {
	existing, err := store.FindByTelegramID(ctx, telegramID)
	if err == nil {
		return existing, false, nil
	}
	if !errors.Is(err, auth.ErrUserNotFound) {
		return auth.User{}, false, fmt.Errorf("look up user before grant: %w", err)
	}
	row, err := store.queries.GrantUserAccess(ctx, generated.GrantUserAccessParams{
		TelegramID:  telegramID,
		Username:    pgtype.Text{String: username, Valid: username != ""},
		DisplayName: pgtype.Text{String: displayName, Valid: displayName != ""},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// A concurrent grant inserted the user first.
		existing, err = store.FindByTelegramID(ctx, telegramID)
		if err != nil {
			return auth.User{}, false, fmt.Errorf("look up concurrently granted user: %w", err)
		}
		return existing, false, nil
	}
	if err != nil {
		return auth.User{}, false, fmt.Errorf("grant user access: %w", err)
	}
	return userFromColumns(row.ID, row.TelegramID, row.Username, row.DisplayName), true, nil
}

// ListUsers returns every application user ordered by Telegram ID.
func (store *Store) ListUsers(ctx context.Context) ([]auth.User, error) {
	rows, err := store.queries.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	users := make([]auth.User, 0, len(rows))
	for _, row := range rows {
		users = append(users, userFromColumns(row.ID, row.TelegramID, row.Username, row.DisplayName))
	}
	return users, nil
}

// DeleteUser deletes a user together with their favorites and price alerts.
// The boolean reports whether the user existed.
func (store *Store) DeleteUser(ctx context.Context, telegramID int64) (bool, error) {
	_, err := store.queries.DeleteUserByTelegramID(ctx, telegramID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("delete user: %w", err)
	}
	return true, nil
}

// BootstrapAdministrator inserts the configured administrator only if absent.
// Existing rows and their timestamps stay untouched. Administrative authority
// is always checked against configuration.
func (store *Store) BootstrapAdministrator(ctx context.Context, telegramID int64) error {
	if err := store.queries.BootstrapAdministrator(ctx, telegramID); err != nil {
		return fmt.Errorf("bootstrap administrator: %w", err)
	}
	return nil
}

func userFromColumns(id, telegramID int64, username, displayName pgtype.Text) auth.User {
	return auth.User{ID: id, TelegramID: telegramID, Username: username.String, DisplayName: displayName.String}
}
