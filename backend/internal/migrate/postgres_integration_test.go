package migrate_test

import (
	"context"
	"io/fs"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	"crypto-scanner/internal/migrate"
	"crypto-scanner/internal/storage/postgres"
	"crypto-scanner/migrations"
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
		"app.api_tokens",
		"app.coingecko_asset_mappings",
		"app.coingecko_mapping_bootstrap",
		"app.coingecko_market_caps",
		"app.favorites",
		"app.price_alerts",
		"app.scanner_indicator_intervals",
		"app.scanner_indicators",
		"app.schema_migrations",
		"app.sessions",
		"app.strategies",
		"app.strategy_coins",
		"app.strategy_states",
		"app.users",
		"binance_spot.candle_history_coverage",
		"binance_spot.candles",
		"binance_spot.coinm_perpetuals",
		"binance_spot.empty_candle_gaps",
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
		"api_tokens_name_check", "api_tokens_pkey", "api_tokens_token_hash_check",
		"api_tokens_token_hash_key", "api_tokens_user_id_fkey",
		"candle_history_coverage_instrument_id_fkey", "candle_history_coverage_pkey",
		"candle_history_coverage_policy_positive", "candle_history_coverage_supported_interval",
		"candle_history_coverage_target_positive", "candles_close_positive", "candles_high_valid",
		"candles_instrument_id_fkey", "candles_low_positive", "candles_low_valid", "candles_open_positive",
		"candles_pkey", "candles_quote_volume_nonnegative", "candles_supported_interval",
		"candles_time_order", "candles_trade_count_nonnegative", "candles_values_finite",
		"candles_volume_nonnegative", "coingecko_asset_mappings_coin_id_matches_status",
		"coingecko_asset_mappings_pkey", "coingecko_asset_mappings_status_check",
		"coingecko_mapping_bootstrap_id_check", "coingecko_mapping_bootstrap_pkey",
		"coingecko_market_caps_market_cap_usd_check", "coingecko_market_caps_pkey", "coinm_perpetuals_pkey",
		"empty_candle_gaps_instrument_id_fkey", "empty_candle_gaps_ordered", "empty_candle_gaps_pkey",
		"empty_candle_gaps_supported_interval", "favorites_instrument_id_fkey", "favorites_pkey",
		"favorites_user_id_fkey", "instruments_base_nonempty",
		"instruments_deactivated_at_matches_activity", "instruments_pkey", "instruments_quote_nonempty",
		"instruments_symbol_key", "instruments_symbol_nonempty", "price_alerts_favorite_fk",
		"price_alerts_pkey", "price_alerts_target_positive", "price_alerts_unique_target",
		"scanner_indicator_intervals_indicator_id_fkey", "scanner_indicator_intervals_pkey",
		"scanner_indicator_intervals_supported_interval", "scanner_indicators_pkey",
		"scanner_indicators_unique_selection", "schema_migrations_pkey", "sessions_check", "sessions_pkey",
		"sessions_token_hash_check", "sessions_user_id_fkey", "strategies_coin_source",
		"strategies_coin_source_signal", "strategies_direction_check", "strategies_market_cap_range",
		"strategies_market_cap_signal", "strategies_max_market_cap_usd_check",
		"strategies_min_market_cap_usd_check", "strategies_name_key", "strategies_pkey",
		"strategies_short_levels", "strategies_signal_evaluation", "strategies_signal_without_trading",
		"strategies_target_ratio_check", "strategies_trading_direction", "strategies_window_candles_check",
		"strategy_coins_instrument_id_fkey", "strategy_coins_pkey", "strategy_coins_strategy_id_fkey",
		"strategy_states_buys_check", "strategy_states_check", "strategy_states_instrument_id_fkey",
		"strategy_states_opened_when_filled", "strategy_states_pkey", "strategy_states_quantity_check",
		"strategy_states_stop_loss_check", "strategy_states_strategy_id_fkey",
		"strategy_states_take_profit_check", "sync_state_pkey", "sync_state_status", "users_pkey",
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

	latest := latestMigrationVersion(t)
	for version := latest; version > 9; version-- {
		if err := migrate.Run(ctx, []string{"down"}, loadDatabaseURL); err != nil {
			t.Fatalf("migrate v%d to v%d down: %v", version, version-1, err)
		}
	}

	var indexExists bool
	if err := db.QueryRow(ctx,
		"SELECT to_regclass('binance_spot.candles_instrument_interval_time_desc_idx') IS NOT NULL",
	).Scan(&indexExists); err != nil || indexExists {
		t.Fatalf("redundant descending candle index exists = %t, error = %v", indexExists, err)
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
		t.Fatalf("migrate v9 to v8 down: %v", err)
	}
	if err := db.QueryRow(ctx,
		"SELECT to_regclass('binance_spot.candles_instrument_interval_time_desc_idx') IS NOT NULL",
	).Scan(&indexExists); err != nil || !indexExists {
		t.Fatalf("v9 rollback restored descending candle index = %t, error = %v", indexExists, err)
	}
	if err := migrate.Run(ctx, []string{"down"}, loadDatabaseURL); err != nil {
		t.Fatalf("migrate v8 to v7 down: %v", err)
	}
	var alertTables bool
	if err := db.QueryRow(ctx, `SELECT to_regclass('app.favorites') IS NOT NULL OR to_regclass('app.price_alerts') IS NOT NULL`).Scan(&alertTables); err != nil || alertTables {
		t.Fatalf("v8 rollback left favorites/alerts tables = %t, error = %v", alertTables, err)
	}
	for version := 7; version > 2; version-- {
		if err := migrate.Run(ctx, []string{"down"}, loadDatabaseURL); err != nil {
			t.Fatalf("migrate v%d to v%d down: %v", version, version-1, err)
		}
	}

	var operatorTable, usersTable, remainingBinanceSchema, marketCapTables bool
	if err := db.QueryRow(ctx, `
		SELECT to_regclass('app.operator_owned') IS NOT NULL,
		       to_regclass('app.users') IS NOT NULL,
		       to_regnamespace('binance_spot') IS NOT NULL,
		       to_regclass('app.coingecko_mapping_bootstrap') IS NOT NULL
		       OR to_regclass('app.coingecko_asset_mappings') IS NOT NULL
		       OR to_regclass('app.coingecko_market_caps') IS NOT NULL
	`).Scan(&operatorTable, &usersTable, &remainingBinanceSchema, &marketCapTables); err != nil {
		t.Fatalf("inspect v3 to v2 rollback: %v", err)
	}
	if !operatorTable || !usersTable || !remainingBinanceSchema || marketCapTables {
		t.Fatalf("v3 to v2 rollback ownership: operator=%t users=%t binance_schema=%t market_cap_tables=%t", operatorTable, usersTable, remainingBinanceSchema, marketCapTables)
	}
	if err := migrate.Run(ctx, []string{"down"}, loadDatabaseURL); err != nil {
		t.Fatalf("migrate v2 to v1 down: %v", err)
	}
	if err := db.QueryRow(ctx, `SELECT to_regclass('app.operator_owned') IS NOT NULL, to_regclass('app.users') IS NOT NULL, to_regnamespace('binance_spot') IS NOT NULL`).Scan(&operatorTable, &usersTable, &remainingBinanceSchema); err != nil {
		t.Fatalf("inspect v2 to v1 rollback: %v", err)
	}
	if !operatorTable || !usersTable || !remainingBinanceSchema {
		t.Fatalf("v2 to v1 rollback ownership: operator=%t users=%t binance_schema=%t", operatorTable, usersTable, remainingBinanceSchema)
	}
	if err := migrate.Run(ctx, []string{"down"}, loadDatabaseURL); err != nil {
		t.Fatalf("migrate v1 down: %v", err)
	}
	if err := db.QueryRow(ctx, `SELECT to_regclass('app.operator_owned') IS NOT NULL, to_regclass('app.users') IS NOT NULL, to_regnamespace('binance_spot') IS NOT NULL`).Scan(&operatorTable, &usersTable, &remainingBinanceSchema); err != nil {
		t.Fatalf("inspect v1 rollback: %v", err)
	}
	if !operatorTable || usersTable || remainingBinanceSchema {
		t.Fatalf("v1 rollback ownership: operator=%t users=%t binance_schema=%t", operatorTable, usersTable, remainingBinanceSchema)
	}

	if _, err := db.Exec(ctx, "DROP TABLE app.operator_owned; DROP SCHEMA app"); err != nil {
		t.Fatalf("reset operator-owned sentinel: %v", err)
	}
	if err := migrate.Run(ctx, []string{"up"}, loadDatabaseURL); err != nil {
		t.Fatalf("migrate absent schemas up: %v", err)
	}
	for version := latest; version >= 1; version-- {
		if err := migrate.Run(ctx, []string{"down"}, loadDatabaseURL); err != nil {
			t.Fatalf("migrate absent schemas v%d down: %v", version, err)
		}
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
	if _, err := db.Exec(ctx, "UPDATE public.crypto_scanner_schema_versions SET version = version + 1"); err != nil {
		t.Fatalf("create future migration metadata: %v", err)
	}
	if err := postgres.VerifySchema(ctx, db, databaseURL); err == nil {
		t.Fatal("VerifySchema() accepted future migration metadata")
	}
	if _, err := db.Exec(ctx, "UPDATE public.crypto_scanner_schema_versions SET version = version - 1"); err != nil {
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
	for version := latest - 1; version >= 1; version-- {
		if err := migrate.Run(ctx, []string{"down"}, loadDatabaseURL); err != nil {
			t.Fatalf("migrate pre-existing schemas v%d down: %v", version, err)
		}
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

// latestMigrationVersion is the version of the newest embedded migration.
func latestMigrationVersion(t *testing.T) int64 {
	t.Helper()
	names, err := fs.Glob(migrations.Files, "*.up.sql")
	if err != nil || len(names) == 0 {
		t.Fatalf("list embedded migrations: %v", err)
	}
	sort.Strings(names)
	version, err := strconv.ParseInt(strings.SplitN(names[len(names)-1], "_", 2)[0], 10, 64)
	if err != nil {
		t.Fatalf("parse newest migration version: %v", err)
	}
	return version
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
