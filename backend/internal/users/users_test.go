package users_test

import (
	"context"
	"testing"

	"crypto-scanner/internal/auth"
	"crypto-scanner/internal/users"
)

// A grant that adds a user and a deletion change the monitored set; a repeated
// grant does not.
func TestAccessChangesAreReported(t *testing.T) {
	store := &store{users: map[int64]bool{}}
	service := users.New(store, 100)
	changes := 0
	service.OnChanged(func() { changes++ })

	for range 2 {
		if _, err := service.Grant(t.Context(), 200, "ada", "Ada"); err != nil {
			t.Fatal(err)
		}
	}
	if changes != 1 {
		t.Fatalf("changes after granting twice = %d, want 1", changes)
	}
	if err := service.Delete(t.Context(), 200); err != nil {
		t.Fatal(err)
	}
	if changes != 2 {
		t.Fatalf("changes after deleting = %d, want 2", changes)
	}
}

type store struct{ users map[int64]bool }

func (s *store) GrantAccess(_ context.Context, telegramID int64, _, _ string) (auth.User, bool, error) {
	created := !s.users[telegramID]
	s.users[telegramID] = true
	return auth.User{TelegramID: telegramID}, created, nil
}

func (s *store) ListUsers(context.Context) ([]users.User, error) { return nil, nil }

func (s *store) DeleteUser(_ context.Context, telegramID int64) (bool, error) {
	existed := s.users[telegramID]
	delete(s.users, telegramID)
	return existed, nil
}

func (s *store) SetUserStrategyAlerts(context.Context, int64, bool) (bool, error) { return true, nil }
