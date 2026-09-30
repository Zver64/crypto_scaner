// Package users lets the administrator review and delete application users
// and choose who receives strategy alerts. Users are added through the
// Telegram bot.
package users

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"crypto-scanner/internal/auth"
)

// ErrAdministratorProtected means that the configured administrator cannot be
// deleted and always receives strategy alerts.
var ErrAdministratorProtected = errors.New("the administrator cannot be changed")

// User is an application user with their strategy alert setting.
type User struct {
	auth.User
	// StrategyAlerts reports whether the user receives strategy alerts; the
	// administrator always does.
	StrategyAlerts bool
}

// Store persists application users.
type Store interface {
	ListUsers(context.Context) ([]User, error)
	// DeleteUser deletes a user with their favorites and price alerts and
	// reports whether the user existed.
	DeleteUser(ctx context.Context, telegramID int64) (bool, error)
	// SetUserStrategyAlerts reports whether the user exists.
	SetUserStrategyAlerts(ctx context.Context, telegramID int64, enabled bool) (bool, error)
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
func (service *Service) List(ctx context.Context) ([]User, error) {
	items, err := service.store.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].Administrator = items[i].TelegramID == service.administratorID
		items[i].StrategyAlerts = items[i].StrategyAlerts || items[i].Administrator
	}
	slices.SortStableFunc(items, func(left, right User) int {
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

// SetStrategyAlerts chooses whether a user receives strategy alerts. It fails
// with ErrAdministratorProtected or auth.ErrUserNotFound.
func (service *Service) SetStrategyAlerts(ctx context.Context, telegramID int64, enabled bool) error {
	if telegramID == service.administratorID {
		return ErrAdministratorProtected
	}
	updated, err := service.store.SetUserStrategyAlerts(ctx, telegramID, enabled)
	if err != nil {
		return fmt.Errorf("set strategy alerts of user %d: %w", telegramID, err)
	}
	if !updated {
		return auth.ErrUserNotFound
	}
	return nil
}
