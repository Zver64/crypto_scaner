package postgres

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"crypto-scanner/migrations"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const migrationLockID int64 = 19193286063743041

// DB is the application's PostgreSQL pool.
type DB struct {
	*pgxpool.Pool
}

// Direction is a validated migration operation.
type Direction uint8

const (
	DirectionUp Direction = iota + 1
	DirectionDown
)

func (d Direction) String() string {
	if d == DirectionUp {
		return "up"
	}
	if d == DirectionDown {
		return "down"
	}
	return "unknown"
}

// Open connects to PostgreSQL and verifies that it responds. It does not apply
// or verify migrations, allowing the standalone migration command to use the
// same connection path.
func Open(ctx context.Context, databaseURL string) (*DB, error) {
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse PostgreSQL configuration: invalid DATABASE_URL")
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, safeError("open PostgreSQL", err, databaseURL)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, safeError("ping PostgreSQL", err, databaseURL)
	}
	return &DB{Pool: pool}, nil
}

// OpenVerified connects to PostgreSQL and rejects an absent or outdated schema.
// It never mutates the database.
func OpenVerified(ctx context.Context, databaseURL string) (*DB, error) {
	db, err := Open(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := VerifySchema(ctx, db, databaseURL); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// RowQuerier is the narrow PostgreSQL boundary needed for schema verification.
type RowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

// VerifySchema checks the migration version without applying migrations.
func VerifySchema(ctx context.Context, queries RowQuerier, databaseURL string) error {
	exists, version, err := schemaMetadata(ctx, queries)
	if err != nil {
		return safeError("verify PostgreSQL schema metadata", err, databaseURL)
	}
	if !exists {
		return fmt.Errorf("PostgreSQL schema is not initialized; apply database migrations")
	}
	versions, err := migrations.Versions()
	if err != nil {
		return fmt.Errorf("discover embedded migrations: %w", err)
	}
	latestVersion := int64(0)
	if len(versions) > 0 {
		latestVersion = versions[len(versions)-1]
	}
	if version != latestVersion {
		return fmt.Errorf("PostgreSQL schema is at version %d; version %d is required; apply database migrations", version, latestVersion)
	}
	return nil
}

// Migrate applies all pending migrations, or rolls back one migration.
func Migrate(ctx context.Context, databaseURL string, direction Direction) error {
	db, err := Open(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.Begin(ctx)
	if err != nil {
		return safeError("begin PostgreSQL migration", err, databaseURL)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", migrationLockID); err != nil {
		return safeError("lock PostgreSQL migrations", err, databaseURL)
	}
	version, err := schemaVersion(ctx, tx)
	if err != nil {
		return safeError("read PostgreSQL schema version", err, databaseURL)
	}

	versions, err := migrations.Versions()
	if err != nil {
		return fmt.Errorf("discover embedded migrations: %w", err)
	}
	if len(versions) == 0 {
		return fmt.Errorf("no embedded migrations found")
	}
	latestVersion := versions[len(versions)-1]
	switch direction {
	case DirectionUp:
		if version == latestVersion {
			if err := tx.Commit(ctx); err != nil {
				return safeError("commit no-op PostgreSQL up migration", err, databaseURL)
			}
			return nil
		}
		if version < 0 || version >= latestVersion {
			return fmt.Errorf("cannot migrate PostgreSQL up from version %d; latest version is %d", version, latestVersion)
		}
		for _, nextVersion := range versions {
			if nextVersion <= version {
				continue
			}
			filename, err := migrations.Filename(nextVersion, "up")
			if err != nil {
				return err
			}
			if err := applyMigration(ctx, tx, filename, databaseURL); err != nil {
				return err
			}
		}
	case DirectionDown:
		if version == 0 {
			if err := tx.Commit(ctx); err != nil {
				return safeError("commit no-op PostgreSQL down migration", err, databaseURL)
			}
			return nil
		}
		if version < 1 || version > latestVersion {
			return fmt.Errorf("cannot migrate PostgreSQL down from version %d; latest version is %d", version, latestVersion)
		}
		filename, err := migrations.Filename(version, "down")
		if err != nil {
			return err
		}
		if err := applyMigration(ctx, tx, filename, databaseURL); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported migration direction %q", direction)
	}
	if err := tx.Commit(ctx); err != nil {
		return safeError("commit PostgreSQL migration", err, databaseURL)
	}
	return nil
}

func applyMigration(ctx context.Context, tx pgx.Tx, filename, databaseURL string) error {
	statement, err := migrations.Files.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("read embedded migration %s: %w", filename, err)
	}
	if _, err := tx.Exec(ctx, string(statement)); err != nil {
		return safeError("apply PostgreSQL migration "+filename, err, databaseURL)
	}
	return nil
}

func schemaVersion(ctx context.Context, queries RowQuerier) (int64, error) {
	_, version, err := schemaMetadata(ctx, queries)
	return version, err
}

func schemaMetadata(ctx context.Context, queries RowQuerier) (bool, int64, error) {
	var exists bool
	if err := queries.QueryRow(ctx,
		"SELECT to_regclass('app.schema_migrations') IS NOT NULL",
	).Scan(&exists); err != nil {
		return false, 0, err
	}
	if !exists {
		return false, 0, nil
	}
	var version int64
	if err := queries.QueryRow(ctx,
		"SELECT COALESCE(MAX(version), 0) FROM app.schema_migrations",
	).Scan(&version); err != nil {
		return true, 0, err
	}
	return true, version, nil
}

func safeError(operation string, err error, databaseURL string) error {
	message := err.Error()
	message = strings.ReplaceAll(message, databaseURL, "[REDACTED]")
	if parsed, parseErr := pgxpool.ParseConfig(databaseURL); parseErr == nil && parsed.ConnConfig.Password != "" {
		message = strings.ReplaceAll(message, parsed.ConnConfig.Password, "[REDACTED]")
		message = strings.ReplaceAll(message, url.QueryEscape(parsed.ConnConfig.Password), "[REDACTED]")
	}
	return fmt.Errorf("%s: %s", operation, message)
}
