// Package auth contains application-facing authentication domain values.
package auth

import (
	"context"
	"errors"
)

// ErrUserNotFound means that no enabled application user matches an identity.
var ErrUserNotFound = errors.New("enabled user not found")

var (
	// ErrUnauthenticated means that presented credentials are invalid or expired.
	ErrUnauthenticated = errors.New("Telegram authentication is invalid or expired")
	// ErrAccessDenied means that a valid identity is not an enabled user.
	ErrAccessDenied = errors.New("Telegram user is not allowed")
)

// User is an application user independent of its persistence representation.
type User struct {
	ID          int64
	TelegramID  int64
	Username    string
	DisplayName string
	Enabled     bool
}

// UserStore is the persistence seam used by authentication.
type UserStore interface {
	FindEnabledByTelegramID(context.Context, int64) (User, error)
}
