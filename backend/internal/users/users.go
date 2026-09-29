// Package users lets the administrator review and delete application users.
// Users are added through the Telegram bot.
package users

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"crypto-scanner/internal/auth"
)

// ErrAdministratorProtected means that the configured administrator cannot be deleted.
var ErrAdministratorProtected = errors.New("the administrator cannot be deleted")

// Store persists application users.
type Store interface {
	ListUsers(context.Context) ([]auth.User, error)
	// DeleteUser deletes a user with their favorites and price alerts and
	// reports whether the user existed.
	DeleteUser(ctx context.Context, telegramID int64) (bool, error)
}

// Service lists and deletes application users.
type Service struct {
	store           Store
	administratorID int64
	changed         func()
}

// New creates a service that protects administratorID and calls changed after
// every deletion, because a deleted user's favorites leave the monitored set.
func New(store Store, administratorID int64, changed func()) *Service {
	return &Service{store: store, administratorID: administratorID, changed: changed}
}

// List returns every user, the administrator first.
func (service *Service) List(ctx context.Context) ([]auth.User, error) {
	items, err := service.store.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].Administrator = items[i].TelegramID == service.administratorID
	}
	slices.SortStableFunc(items, func(left, right auth.User) int {
		switch {
		case left.Administrator == right.Administrator:
			return 0
		case left.Administrator:
			return -1
		default:
			return 1
		}
	})
	return items, nil
}

// Delete deletes a user with their favorites and price alerts. It fails with
// ErrAdministratorProtected or auth.ErrUserNotFound.
func (service *Service) Delete(ctx context.Context, telegramID int64) error {
	if telegramID == service.administratorID {
		return ErrAdministratorProtected
	}
	deleted, err := service.store.DeleteUser(ctx, telegramID)
	if err != nil {
		return fmt.Errorf("delete user %d: %w", telegramID, err)
	}
	if !deleted {
		return auth.ErrUserNotFound
	}
	service.changed()
	return nil
}
