package migrate_test

import (
	"context"
	"os"
	"sort"
	"testing"

	"crypto-scanner/internal/migrate"
	"crypto-scanner/internal/storage/postgres"
)

func TestPostgresMigrationLifecycleAndSchemaOwnership(t *testing.T) {
	databaseURL := os.Getenv("CRYPTO_SCANNER_TEST_DATABASE_URL")
	if databaseURL == "" || os.Getenv("CRYPTO_SCANNER_TEST_DATABASE_RESET_OK") != "1" {
		t.Skip("set CRYPTO_SCANNER_TEST_DATABASE_URL to a disposable empty database and CRYPTO_SCANNER_TEST_DATABASE_RESET_OK=1")
	}
	ctx := context.Background()
	loadDatabaseURL := func() (string, error) { return databaseURL, nil }

	db, err := postgres.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open disposable PostgreSQL: %v", err)
	}
	t.Cleanup(db.Close)
	lockDisposablePostgres(t, db)
	reset := func() {
		t.Helper()
		if _, err := db.Exec(context.Background(), `DROP SCHEMA IF EXISTS app CASCADE; DROP SCHEMA IF EXISTS binance_spot CASCADE; DROP TABLE IF EXISTS public.crypto_scanner_schema_versions`); err != nil {
			t.Errorf("reset disposable PostgreSQL: %v", err)
		}
	}
	reset()
	t.Cleanup(reset)

	var appSchema, binanceSchema, migrationMetadata bool
	if err := db.QueryRow(ctx, `
		SELECT to_regnamespace('app') IS NOT NULL,
		       to_regnamespace('binance_spot') IS NOT NULL,
		       to_regclass('public.crypto_scanner_schema_versions') IS NOT NULL
	`).Scan(&appSchema, &binanceSchema, &migrationMetadata); err != nil {
		t.Fatalf("inspect disposable database: %v", err)
	}
	if appSchema || binanceSchema || migrationMetadata {
		t.Fatalf("integration database is not empty: app=%t binance_spot=%t migration_metadata=%t", appSchema, binanceSchema, migrationMetadata)
	}
	if err := postgres.VerifySchema(ctx, db, databaseURL); err == nil {
		t.Fatal("VerifySchema() accepted a zero-version database")
	}
	if err := migrate.Run(ctx, []string{"down"}, loadDatabaseURL); err != nil {
		t.Fatalf("down on fresh database: %v", err)
	}

	if err := migrate.Run(ctx, []string{"up"}, loadDatabaseURL); err != nil {
		t.Fatalf("first migrate up: %v", err)
	}
	if err := migrate.Run(ctx, []string{"up"}, loadDatabaseURL); err != nil {
		t.Fatalf("second migrate up: %v", err)
	}
	if err := postgres.VerifySchema(ctx, db, databaseURL); err != nil {
		t.Fatalf("verify current schema: %v", err)
	}

	wantRelations := []string{
		"app.schema_migrations",
		"app.users",
		"binance_spot.candles",
		"binance_spot.instruments",
		"binance_spot.sync_state",
	}
	rows, err := db.Query(ctx, `
		SELECT n.nspname || '.' || c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'r'
		  AND n.nspname IN ('app', 'binance_spot')
		ORDER BY 1
	`)
	if err != nil {
		t.Fatalf("list migrated tables: %v", err)
	}
	var relations []string
	for rows.Next() {
		var relation string
		if err := rows.Scan(&relation); err != nil {
			t.Fatalf("scan migrated table: %v", err)
		}
		relations = append(relations, relation)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate migrated tables: %v", err)
	}
	if !equalStrings(relations, wantRelations) {
		t.Fatalf("relations = %v, want %v", relations, wantRelations)
	}

	wantConstraints := []string{
		"candles_high_valid", "candles_instrument_id_fkey", "candles_low_valid",
		"candles_open_positive", "candles_pkey", "candles_quote_volume_nonnegative",
		"candles_supported_interval", "candles_time_order", "candles_trade_count_nonnegative",
		"candles_volume_nonnegative", "instruments_base_nonempty", "instruments_pkey",
		"instruments_quote_nonempty", "instruments_symbol_key", "instruments_symbol_nonempty",
		"schema_migrations_pkey", "sync_state_pkey", "sync_state_status", "users_pkey",
		"users_telegram_id_key",
	}
	constraintRows, err := db.Query(ctx, `
		SELECT con.conname
		FROM pg_constraint con
		JOIN pg_namespace n ON n.oid = con.connamespace
		WHERE n.nspname IN ('app', 'binance_spot')
		  AND con.contype <> 'n'
		ORDER BY con.conname
	`)
	if err != nil {
		t.Fatalf("list migrated constraints: %v", err)
	}
	var constraints []string
	for constraintRows.Next() {
		var constraint string
		if err := constraintRows.Scan(&constraint); err != nil {
			t.Fatalf("scan migrated constraint: %v", err)
		}
		constraints = append(constraints, constraint)
	}
	constraintRows.Close()
	if err := constraintRows.Err(); err != nil {
		t.Fatalf("iterate migrated constraints: %v", err)
	}
	if !equalStrings(constraints, wantConstraints) {
		t.Fatalf("constraints = %v, want %v", constraints, wantConstraints)
	}

	var indexExists bool
	if err := db.QueryRow(ctx,
		"SELECT to_regclass('binance_spot.candles_instrument_interval_time_desc_idx') IS NOT NULL",
	).Scan(&indexExists); err != nil || !indexExists {
		t.Fatalf("descending candle index exists = %t, error = %v", indexExists, err)
	}

	if _, err := db.Exec(ctx, "CREATE TABLE app.operator_owned (id BIGINT PRIMARY KEY)"); err != nil {
		t.Fatalf("create operator-owned rollback sentinel: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(context.Background(), "DROP TABLE IF EXISTS app.operator_owned"); err != nil {
			t.Errorf("clean up operator-owned sentinel: %v", err)
			return
		}
		if _, err := db.Exec(context.Background(), "DROP SCHEMA IF EXISTS app"); err != nil {
			t.Errorf("clean up app schema: %v", err)
		}
	})
	if err := migrate.Run(ctx, []string{"down"}, loadDatabaseURL); err != nil {
		t.Fatalf("migrate down: %v", err)
	}

	var operatorTable, usersTable, remainingBinanceSchema bool
	if err := db.QueryRow(ctx, `
		SELECT to_regclass('app.operator_owned') IS NOT NULL,
		       to_regclass('app.users') IS NOT NULL,
		       to_regnamespace('binance_spot') IS NOT NULL
	`).Scan(&operatorTable, &usersTable, &remainingBinanceSchema); err != nil {
		t.Fatalf("inspect rollback: %v", err)
	}
	if !operatorTable || !usersTable || !remainingBinanceSchema {
		t.Fatalf("rollback ownership: operator=%t users=%t binance_schema=%t", operatorTable, usersTable, remainingBinanceSchema)
	}
	if err := migrate.Run(ctx, []string{"down"}, loadDatabaseURL); err != nil {
		t.Fatalf("migrate initial schema down: %v", err)
	}
	if err := db.QueryRow(ctx, `SELECT to_regclass('app.operator_owned') IS NOT NULL, to_regclass('app.users') IS NOT NULL, to_regnamespace('binance_spot') IS NOT NULL`).Scan(&operatorTable, &usersTable, &remainingBinanceSchema); err != nil {
		t.Fatalf("inspect initial rollback: %v", err)
	}
	if !operatorTable || usersTable || remainingBinanceSchema {
		t.Fatalf("initial rollback ownership: operator=%t users=%t binance_schema=%t", operatorTable, usersTable, remainingBinanceSchema)
	}

	if _, err := db.Exec(ctx, "DROP TABLE app.operator_owned; DROP SCHEMA app"); err != nil {
		t.Fatalf("reset operator-owned sentinel: %v", err)
	}
	if err := migrate.Run(ctx, []string{"up"}, loadDatabaseURL); err != nil {
		t.Fatalf("migrate absent schemas up: %v", err)
	}
	if err := migrate.Run(ctx, []string{"down"}, loadDatabaseURL); err != nil {
		t.Fatalf("migrate absent schemas down: %v", err)
	}
	if err := migrate.Run(ctx, []string{"down"}, loadDatabaseURL); err != nil {
		t.Fatalf("migrate absent schemas initial down: %v", err)
	}
	if err := db.QueryRow(ctx, `SELECT to_regnamespace('app') IS NOT NULL, to_regnamespace('binance_spot') IS NOT NULL`).Scan(&appSchema, &binanceSchema); err != nil {
		t.Fatalf("inspect migration-owned schemas: %v", err)
	}
	if appSchema || binanceSchema {
		t.Fatalf("migration-owned schemas survived down: app=%t binance_spot=%t", appSchema, binanceSchema)
	}
	if err := migrate.Run(ctx, []string{"down"}, loadDatabaseURL); err != nil {
		t.Fatalf("migrate already-rolled-back schema down: %v", err)
	}
	if err := db.QueryRow(ctx, `SELECT to_regnamespace('app') IS NOT NULL, to_regnamespace('binance_spot') IS NOT NULL`).Scan(&appSchema, &binanceSchema); err != nil {
		t.Fatalf("inspect idempotent rollback: %v", err)
	}
	if appSchema || binanceSchema {
		t.Fatalf("idempotent rollback recreated schemas: app=%t binance_spot=%t", appSchema, binanceSchema)
	}

	if _, err := db.Exec(ctx, "CREATE SCHEMA app; CREATE SCHEMA binance_spot"); err != nil {
		t.Fatalf("create pre-existing schemas: %v", err)
	}
	if err := migrate.Run(ctx, []string{"up"}, loadDatabaseURL); err != nil {
		t.Fatalf("migrate pre-existing schemas up: %v", err)
	}
	if _, err := db.Exec(ctx, "UPDATE public.crypto_scanner_schema_versions SET version = 3 WHERE version = 2"); err != nil {
		t.Fatalf("create future migration metadata: %v", err)
	}
	if err := postgres.VerifySchema(ctx, db, databaseURL); err == nil {
		t.Fatal("VerifySchema() accepted future migration metadata")
	}
	if _, err := db.Exec(ctx, "UPDATE public.crypto_scanner_schema_versions SET version = 2 WHERE version = 3"); err != nil {
		t.Fatalf("restore current migration metadata: %v", err)
	}
	if err := migrate.Run(ctx, []string{"down"}, loadDatabaseURL); err != nil {
		t.Fatalf("migrate pre-existing schemas down: %v", err)
	}
	if err := db.QueryRow(ctx, `SELECT to_regnamespace('app') IS NOT NULL, to_regnamespace('binance_spot') IS NOT NULL`).Scan(&appSchema, &binanceSchema); err != nil {
		t.Fatalf("inspect pre-existing schemas: %v", err)
	}
	if !appSchema || !binanceSchema {
		t.Fatalf("pre-existing schemas removed on down: app=%t binance_spot=%t", appSchema, binanceSchema)
	}
	if err := migrate.Run(ctx, []string{"down"}, loadDatabaseURL); err != nil {
		t.Fatalf("migrate pre-existing schemas initial down: %v", err)
	}
	var appUsers, instruments bool
	if err := db.QueryRow(ctx, `SELECT to_regclass('app.users') IS NOT NULL, to_regclass('binance_spot.instruments') IS NOT NULL, to_regnamespace('app') IS NOT NULL, to_regnamespace('binance_spot') IS NOT NULL`).Scan(&appUsers, &instruments, &appSchema, &binanceSchema); err != nil {
		t.Fatalf("inspect pre-existing schemas after initial down: %v", err)
	}
	if appUsers || instruments || !appSchema || !binanceSchema {
		t.Fatalf("pre-existing schema ownership after v1 down: users=%t instruments=%t app=%t binance_spot=%t", appUsers, instruments, appSchema, binanceSchema)
	}
	if _, err := db.Exec(ctx, "DROP SCHEMA app; DROP SCHEMA binance_spot"); err != nil {
		t.Fatalf("clean up pre-existing schemas: %v", err)
	}
	if _, err := db.Exec(ctx, "DROP TABLE IF EXISTS public.crypto_scanner_schema_versions"); err != nil {
		t.Fatalf("clean up migration metadata: %v", err)
	}
}

func equalStrings(got, want []string) bool {
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}
