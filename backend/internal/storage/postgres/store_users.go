package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"

	"crypto-scanner/internal/auth"
	generated "crypto-scanner/internal/storage/postgres/sqlc"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (store *Store) FindEnabledByTelegramID(ctx context.Context, telegramID int64) (auth.User, error) {
	row, err := store.queries.FindEnabledUserByTelegramID(ctx, telegramID)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, auth.ErrUserNotFound
	}
	if err != nil {
		return auth.User{}, fmt.Errorf("find enabled user by Telegram ID: %w", err)
	}
	return auth.User{ID: row.ID, TelegramID: row.TelegramID, Username: row.Username.String, DisplayName: row.DisplayName.String, Enabled: row.IsEnabled}, nil
}

// GrantAccess creates or re-enables an application user. The boolean reports
// whether Scanner Access changed, so repeated additions remain harmless.
func (store *Store) GrantAccess(ctx context.Context, telegramID int64, username, displayName string) (auth.User, bool, error) {
	existing, err := store.FindEnabledByTelegramID(ctx, telegramID)
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
	if err != nil {
		return auth.User{}, false, fmt.Errorf("grant user access: %w", err)
	}
	return auth.User{ID: row.ID, TelegramID: row.TelegramID, Username: row.Username.String, DisplayName: row.DisplayName.String, Enabled: row.IsEnabled}, true, nil
}

// ListNonAdministratorUsers returns a deterministic page of accounts that the
// configured Administrator may remove.
func (store *Store) ListNonAdministratorUsers(ctx context.Context, administratorID int64, offset, limit int) ([]auth.User, error) {
	if offset < 0 || limit <= 0 || limit > math.MaxInt32 {
		return nil, fmt.Errorf("invalid user page")
	}
	rows, err := store.queries.ListNonAdministratorUsers(ctx, generated.ListNonAdministratorUsersParams{TelegramID: administratorID, Limit: int32(limit), Offset: int32(offset)})
	if err != nil {
		return nil, fmt.Errorf("list enabled users: %w", err)
	}
	users := make([]auth.User, 0, len(rows))
	for _, row := range rows {
		users = append(users, auth.User{ID: row.ID, TelegramID: row.TelegramID, Username: row.Username.String, DisplayName: row.DisplayName.String, Enabled: row.IsEnabled})
	}
	return users, nil
}

// DeleteUser preserves the account and its user-owned data while revoking access.
// The historical name remains on AccessStore for compatibility with the bot use case.
func (store *Store) DeleteUser(ctx context.Context, id, telegramID int64) (bool, error) {
	_, err := store.queries.DisableUserByID(ctx, generated.DisableUserByIDParams{ID: id, TelegramID: telegramID})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("delete user: %w", err)
	}
	return true, nil
}

// BootstrapAdministrator inserts the configured administrator only if absent.
// Existing rows, including disabled users and their timestamps, stay untouched.
// Administrative authority is always checked against configuration by telegrambot.
func (store *Store) BootstrapAdministrator(ctx context.Context, telegramID int64) error {
	if err := store.queries.BootstrapAdministrator(ctx, telegramID); err != nil {
		return fmt.Errorf("bootstrap administrator: %w", err)
	}
	return nil
}
