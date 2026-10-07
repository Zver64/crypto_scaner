// Package auth contains application-facing authentication domain values.
package auth

import (
	"context"
	"errors"
)

// ErrUserNotFound means that no application user matches an identity.
var ErrUserNotFound = errors.New("user not found")

var (
	// ErrUnauthenticated means that presented credentials are invalid or expired.
	ErrUnauthenticated = errors.New("Telegram authentication is invalid or expired")
	// ErrAccessDenied means that a valid identity is not an application user.
	ErrAccessDenied = errors.New("Telegram user is not allowed")
)

// User is an application user independent of its persistence representation.
type User struct {
	ID          int64
	TelegramID  int64
	Username    string
	DisplayName string
	// Administrator reports the scanner administrator, who manages global
	// settings such as the scanner indicators.
	Administrator bool
	// APIToken reports a user authenticated by an API token, a program,
	// rather than by a Telegram session.
	APIToken bool
}

// UserStore is the persistence seam used by authentication.
type UserStore interface {
	FindByTelegramID(context.Context, int64) (User, error)
}
