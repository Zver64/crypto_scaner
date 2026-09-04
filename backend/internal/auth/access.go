package auth

import "context"

// AccessStore manages the enabled application-user set used by Telegram access
// administration. It is intentionally separate from UserStore so existing
// authentication consumers retain their narrow read-only dependency.
type AccessStore interface {
	UserStore
	GrantAccess(ctx context.Context, telegramID int64, username, displayName string) (User, bool, error)
	ListNonAdministratorUsers(ctx context.Context, administratorID int64, offset, limit int) ([]User, error)
	DeleteUser(ctx context.Context, id, telegramID int64) (bool, error)
}
