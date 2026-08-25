package postgres

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"crypto-scanner/migrations"
	"github.com/golang-migrate/migrate/v4"
	migratepgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

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
	exists, version, dirty, err := schemaMetadata(ctx, queries)
	if err != nil {
		return safeError("verify PostgreSQL schema metadata", err, databaseURL)
	}
	if !exists {
		return fmt.Errorf("PostgreSQL schema is not initialized; apply database migrations")
	}
	latestVersion, err := latestMigrationVersion()
	if err != nil {
		return err
	}
	if dirty || version != latestVersion {
		return fmt.Errorf("PostgreSQL schema is at version %d; version %d is required and metadata dirty=%t; apply database migrations", version, latestVersion, dirty)
	}
	return nil
}

// Migrate applies all pending migrations, or rolls back one migration.
func Migrate(ctx context.Context, databaseURL string, direction Direction) error {
	if direction != DirectionUp && direction != DirectionDown {
		return fmt.Errorf("unsupported migration direction %q", direction)
	}
	lockDB, err := Open(ctx, databaseURL)
	if err != nil {
		return err
	}
	lockConn, err := lockDB.Acquire(ctx)
	if err != nil {
		lockDB.Close()
		return safeError("acquire PostgreSQL migration lock connection", err, databaseURL)
	}
	if _, err := lockConn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLockID); err != nil {
		lockConn.Release()
		lockDB.Close()
		return safeError("acquire PostgreSQL migration lock", err, databaseURL)
	}
	defer func() {
		_, _ = lockConn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", migrationLockID)
		lockConn.Release()
		lockDB.Close()
	}()

	source, err := iofs.New(migrations.Files, ".")
	if err != nil {
		return fmt.Errorf("load embedded migrations: %w", err)
	}
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return fmt.Errorf("parse PostgreSQL configuration: invalid DATABASE_URL")
	}
	sqlDB := stdlib.OpenDB(*poolConfig.ConnConfig)
	databaseDriver, err := migratepgx.WithInstance(sqlDB, &migratepgx.Config{
		DatabaseName:    poolConfig.ConnConfig.Database,
		MigrationsTable: migrationTable,
		SchemaName:      "public",
	})
	if err != nil {
		sqlDB.Close()
		return safeError("open migration database", err, databaseURL)
	}
	instance, err := migrate.NewWithInstance("iofs", source, poolConfig.ConnConfig.Database, databaseDriver)
	if err != nil {
		_ = databaseDriver.Close()
		sqlDB.Close()
		return safeError("open migration instance", err, databaseURL)
	}
	defer sqlDB.Close()
	defer instance.Close()
	stopGraceful := make(chan struct{})
	gracefulDone := make(chan struct{})
	go func() {
		defer close(gracefulDone)
		select {
		case <-ctx.Done():
			select {
			case instance.GracefulStop <- true:
			case <-stopGraceful:
			}
		case <-stopGraceful:
		}
	}()
	defer func() {
		close(stopGraceful)
		<-gracefulDone
	}()
	if err := adoptLegacyBaseline(ctx, lockConn, instance); err != nil {
		return err
	}
	switch direction {
	case DirectionUp:
		err = instance.Up()
	case DirectionDown:
		if _, _, versionErr := instance.Version(); errors.Is(versionErr, migrate.ErrNilVersion) {
			return nil
		} else if versionErr != nil {
			return safeError("read PostgreSQL migration version", versionErr, databaseURL)
		}
		err = instance.Steps(-1)
	}
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return safeError("apply PostgreSQL migration", err, databaseURL)
	}
	return nil
}

func schemaMetadata(ctx context.Context, queries RowQuerier) (bool, int64, bool, error) {
	var exists bool
	if err := queries.QueryRow(ctx,
		"SELECT to_regclass('public.crypto_scanner_schema_versions') IS NOT NULL",
	).Scan(&exists); err != nil {
		return false, 0, false, err
	}
	if !exists {
		return false, 0, false, nil
	}
	var version int64
	var dirty bool
	if err := queries.QueryRow(ctx, "SELECT version, dirty FROM public.crypto_scanner_schema_versions").Scan(&version, &dirty); err != nil {
		return true, 0, false, err
	}
	return true, version, dirty, nil
}

const migrationTable = "crypto_scanner_schema_versions"

const migrationLockID int64 = 19193286063743041

func adoptLegacyBaseline(ctx context.Context, queries RowQuerier, instance *migrate.Migrate) error {
	version, dirty, err := instance.Version()
	if err == nil {
		if dirty {
			return fmt.Errorf("migration metadata is dirty at version %d", version)
		}
		return nil
	}
	if !errors.Is(err, migrate.ErrNilVersion) {
		return err
	}
	var legacyExists bool
	if err := queries.QueryRow(ctx, "SELECT to_regclass('app.schema_migrations') IS NOT NULL").Scan(&legacyExists); err != nil {
		return err
	}
	if !legacyExists {
		return nil
	}
	var legacyVersion int64
	if err := queries.QueryRow(ctx, "SELECT COALESCE(MAX(version), 0) FROM app.schema_migrations").Scan(&legacyVersion); err != nil {
		return err
	}
	if err := validateLegacyMigrationVersion(legacyVersion); err != nil {
		return err
	}
	if err := instance.Force(int(legacyVersion)); err != nil {
		return fmt.Errorf("adopt legacy migration version %d: %w", legacyVersion, err)
	}
	return nil
}

func latestMigrationVersion() (int64, error) {
	versions, err := embeddedMigrationVersions()
	if err != nil {
		return 0, err
	}
	return versions[len(versions)-1], nil
}

func embeddedMigrationVersions() ([]int64, error) {
	driver, err := iofs.New(migrations.Files, ".")
	if err != nil {
		return nil, fmt.Errorf("discover embedded migrations: %w", err)
	}
	version, err := driver.First()
	if err != nil {
		return nil, fmt.Errorf("discover embedded migrations: %w", err)
	}
	versions := []int64{int64(version)}
	for {
		next, err := driver.Next(version)
		if errors.Is(err, os.ErrNotExist) {
			return versions, nil
		}
		if err != nil {
			return nil, fmt.Errorf("discover embedded migrations: %w", err)
		}
		version = next
		versions = append(versions, int64(version))
	}
}

func validateLegacyMigrationVersion(version int64) error {
	if version <= 0 {
		return fmt.Errorf("legacy migration version %d is invalid; expected an available embedded migration", version)
	}
	versions, err := embeddedMigrationVersions()
	if err != nil {
		return err
	}
	for _, available := range versions {
		if version == available {
			return nil
		}
	}
	return fmt.Errorf("legacy migration version %d is not an available embedded migration", version)
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
