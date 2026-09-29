package auth

import "context"

// AccessStore grants Scanner Access from the Telegram bot. It is separate from
// UserStore so authentication consumers retain their narrow read-only dependency.
type AccessStore interface {
	UserStore
	GrantAccess(ctx context.Context, telegramID int64, username, displayName string) (User, bool, error)
}
