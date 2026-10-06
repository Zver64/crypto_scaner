package postgres

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"crypto-scanner/internal/alerts"
	"crypto-scanner/internal/auth"
	"crypto-scanner/internal/favorites"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/marketcap"
	generated "crypto-scanner/internal/storage/postgres/sqlc"
	"crypto-scanner/internal/strategy"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// Store adapts generated PostgreSQL access to application domain values.
type Store struct {
	db      *DB
	queries *generated.Queries
}

// NewStore creates the PostgreSQL application store.
func NewStore(db *DB) *Store { return &Store{db: db, queries: generated.New(db)} }

var (
	_ auth.UserStore        = (*Store)(nil)
	_ auth.AccessStore      = (*Store)(nil)
	_ auth.SessionStore     = (*Store)(nil)
	_ favorites.Store       = (*Store)(nil)
	_ alerts.CRUDStore      = (*Store)(nil)
	_ alerts.MonitorStore   = (*Store)(nil)
	_ marketcap.Store       = (*Store)(nil)
	_ strategy.Store        = (*Store)(nil)
	_ strategy.MonitorStore = (*Store)(nil)
)

func (store *Store) DatabaseReady(ctx context.Context) bool { return store.db.Ping(ctx) == nil }

func (store *Store) MigrationsReady(ctx context.Context) bool {
	return VerifySchema(ctx, store.db, "") == nil
}

func (store *Store) SuccessfulMarketSyncExists(ctx context.Context) bool {
	exists, err := store.queries.SuccessfulMarketSyncExists(ctx)
	return err == nil && exists
}

func decimal(value float64) string { return strconv.FormatFloat(value, 'g', -1, 64) }

func timestamptz(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *value, Valid: true}
}

func timePointer(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}

// lockUser serializes the user's favorite and price alert writes and returns
// the user's Telegram ID.
func lockUser(ctx context.Context, queries *generated.Queries, userID int64) (int64, error) {
	telegramID, err := queries.LockUser(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("lock user: %w", err)
	}
	return telegramID, nil
}

// activeInstrumentID fails with market.ErrInstrumentNotFound unless symbol
// names an active instrument.
func activeInstrumentID(ctx context.Context, queries *generated.Queries, symbol string) (int64, error) {
	id, err := queries.GetActiveInstrumentIDBySymbol(ctx, symbol)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, market.ErrInstrumentNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("get active instrument: %w", err)
	}
	return id, nil
}

func duplicateViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
