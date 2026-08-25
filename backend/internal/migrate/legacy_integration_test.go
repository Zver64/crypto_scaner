package migrate_test

import (
	"context"
	"net/url"
	"os"
	"testing"

	"crypto-scanner/internal/migrate"
	"crypto-scanner/internal/storage/postgres"
	"crypto-scanner/migrations"
)

func TestPostgresLegacyMigrationAdoptionPreservesDataAndSupportsOneStepRollback(t *testing.T) {
	databaseURL := os.Getenv("CRYPTO_SCANNER_TEST_DATABASE_URL")
	if databaseURL == "" || os.Getenv("CRYPTO_SCANNER_TEST_DATABASE_RESET_OK") != "1" {
		t.Skip("set CRYPTO_SCANNER_TEST_DATABASE_URL to a disposable empty database and CRYPTO_SCANNER_TEST_DATABASE_RESET_OK=1")
	}
	ctx := context.Background()
	db, err := postgres.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open disposable PostgreSQL: %v", err)
	}
	t.Cleanup(db.Close)
	lockDisposablePostgres(t, db)
	reset := func() {
		t.Helper()
		if _, err := db.Exec(ctx, `DROP SCHEMA IF EXISTS app CASCADE; DROP SCHEMA IF EXISTS binance_spot CASCADE; DROP TABLE IF EXISTS public.crypto_scanner_schema_versions`); err != nil {
			t.Fatalf("reset disposable PostgreSQL: %v", err)
		}
	}
	reset()
	t.Cleanup(reset)
	var appSchema, binanceSchema, migrationMetadata bool
	if err := db.QueryRow(ctx, `SELECT to_regnamespace('app') IS NOT NULL, to_regnamespace('binance_spot') IS NOT NULL, to_regclass('public.crypto_scanner_schema_versions') IS NOT NULL`).Scan(&appSchema, &binanceSchema, &migrationMetadata); err != nil {
		t.Fatalf("inspect reset disposable PostgreSQL: %v", err)
	}
	if appSchema || binanceSchema || migrationMetadata {
		t.Fatalf("reset disposable database is not empty: app=%t binance_spot=%t migration_metadata=%t", appSchema, binanceSchema, migrationMetadata)
	}
	migrationDatabaseURL := databaseURL
	if parsed, err := url.Parse(databaseURL); err == nil && parsed.Scheme != "" {
		query := parsed.Query()
		query.Set("pool_max_conns", "1")
		parsed.RawQuery = query.Encode()
		migrationDatabaseURL = parsed.String()
	} else {
		migrationDatabaseURL += " pool_max_conns=1"
	}
	loadURL := func() (string, error) { return migrationDatabaseURL, nil }

	apply := func(name string) {
		t.Helper()
		sql, err := migrations.Files.ReadFile(name)
		if err != nil {
			t.Fatalf("read embedded migration %s: %v", name, err)
		}
		if _, err := db.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("execute legacy migration %s: %v", name, err)
		}
	}
	assertLibraryVersion := func(want int64) {
		t.Helper()
		var got int64
		var dirty bool
		if err := db.QueryRow(ctx, "SELECT version, dirty FROM public.crypto_scanner_schema_versions").Scan(&got, &dirty); err != nil {
			t.Fatalf("read golang-migrate metadata: %v", err)
		}
		if got != want || dirty {
			t.Fatalf("golang-migrate metadata = version %d dirty %t, want version %d clean", got, dirty, want)
		}
	}
	assertRows := func() {
		t.Helper()
		var users, instruments, daily int
		if err := db.QueryRow(ctx, `SELECT (SELECT COUNT(*) FROM app.users), (SELECT COUNT(*) FROM binance_spot.instruments), (SELECT COUNT(*) FROM binance_spot.candles WHERE interval = '1d')`).Scan(&users, &instruments, &daily); err != nil {
			t.Fatalf("count preserved legacy rows: %v", err)
		}
		if users != 1 || instruments != 1 || daily != 1 {
			t.Fatalf("preserved rows = users:%d instruments:%d daily:%d, want 1 each", users, instruments, daily)
		}
		var username, symbol, close string
		if err := db.QueryRow(ctx, `SELECT u.username, i.symbol, c.close FROM app.users u CROSS JOIN binance_spot.instruments i CROSS JOIN binance_spot.candles c WHERE u.telegram_id = 7001 AND i.symbol = 'LEGACYUSDT' AND c.interval = '1d'`).Scan(&username, &symbol, &close); err != nil {
			t.Fatalf("read preserved legacy rows: %v", err)
		}
		if username != "legacy-user" || symbol != "LEGACYUSDT" || close != "105.25" {
			t.Fatalf("preserved row values = username:%q symbol:%q close:%q", username, symbol, close)
		}
	}
	seedLegacyRows := func() {
		t.Helper()
		if _, err := db.Exec(ctx, `INSERT INTO app.users (telegram_id, username) VALUES (7001, 'legacy-user'); INSERT INTO binance_spot.instruments (symbol, base_asset, quote_asset, exchange_status, is_active) VALUES ('LEGACYUSDT', 'LEGACY', 'USDT', 'TRADING', true); INSERT INTO binance_spot.candles (instrument_id, interval, open_time, close_time, open, high, low, close, volume, quote_asset_volume, trade_count) SELECT id, '1d', '2026-01-01T00:00:00Z', '2026-01-01T23:59:00Z', 100, 110, 90, 105.25, 10, 1000, 7 FROM binance_spot.instruments WHERE symbol = 'LEGACYUSDT'`); err != nil {
			t.Fatalf("seed legacy rows: %v", err)
		}
	}

	t.Run("legacy v1 baseline applies v2 and repeated up is a no-op", func(t *testing.T) {
		apply("000001_initial.up.sql")
		seedLegacyRows()
		if err := migrate.Run(ctx, []string{"up"}, loadURL); err != nil {
			t.Fatalf("adopt legacy v1 and migrate up: %v", err)
		}
		assertLibraryVersion(2)
		assertRows()
		if _, err := db.Exec(ctx, `INSERT INTO binance_spot.candles (instrument_id, interval, open_time, close_time, open, high, low, close, volume, quote_asset_volume, trade_count) SELECT id, '1h', '2026-01-02T00:00:00Z', '2026-01-02T00:59:00Z', 100, 110, 90, 105, 1, 100, 1 FROM binance_spot.instruments WHERE symbol = 'LEGACYUSDT'`); err != nil {
			t.Fatalf("hourly constraint rejected migrated schema: %v", err)
		}
		if err := migrate.Run(ctx, []string{"up"}, loadURL); err != nil {
			t.Fatalf("repeated migrate up: %v", err)
		}
		assertLibraryVersion(2)
		assertRows()
		reset()
	})

	t.Run("legacy v2 adoption and one-step down/up", func(t *testing.T) {
		apply("000001_initial.up.sql")
		apply("000002_hourly_candles.up.sql")
		if _, err := db.Exec(ctx, "UPDATE app.schema_migrations SET version = 2"); err != nil {
			t.Fatalf("construct legacy v2 metadata: %v", err)
		}
		seedLegacyRows()
		if _, err := db.Exec(ctx, `INSERT INTO binance_spot.candles (instrument_id, interval, open_time, close_time, open, high, low, close, volume, quote_asset_volume, trade_count) SELECT id, '1h', '2026-01-02T00:00:00Z', '2026-01-02T00:59:00Z', 100, 110, 90, 105, 1, 100, 1 FROM binance_spot.instruments WHERE symbol = 'LEGACYUSDT'`); err != nil {
			t.Fatalf("seed legacy hourly row: %v", err)
		}
		if err := migrate.Run(ctx, []string{"up"}, loadURL); err != nil {
			t.Fatalf("adopt legacy v2: %v", err)
		}
		assertLibraryVersion(2)
		assertRows()
		var hourly int
		if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM binance_spot.candles WHERE interval = '1h'").Scan(&hourly); err != nil || hourly != 1 {
			t.Fatalf("adopted hourly rows = %d, error = %v", hourly, err)
		}
		if _, err := db.Exec(ctx, `INSERT INTO binance_spot.sync_state (profile_key, status, last_succeeded_at) VALUES ('binance:spot:USDT:1d:UTC', 'succeeded', now()), ('binance:spot:USDT:1h:UTC', 'succeeded', now())`); err != nil {
			t.Fatalf("seed successful profile states: %v", err)
		}
		assertReady := func(want bool) {
			t.Helper()
			var ready bool
			if err := db.QueryRow(ctx, `SELECT COUNT(DISTINCT profile_key) = 2 FROM binance_spot.sync_state WHERE profile_key IN ('binance:spot:USDT:1d:UTC', 'binance:spot:USDT:1h:UTC') AND last_succeeded_at IS NOT NULL`).Scan(&ready); err != nil {
				t.Fatalf("inspect profile readiness: %v", err)
			}
			if ready != want {
				t.Fatalf("profile readiness = %t, want %t", ready, want)
			}
		}
		assertReady(true)
		if err := migrate.Run(ctx, []string{"down"}, loadURL); err != nil {
			t.Fatalf("one-step down from adopted v2: %v", err)
		}
		assertLibraryVersion(1)
		assertRows()
		if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM binance_spot.candles WHERE interval = '1h'").Scan(&hourly); err != nil || hourly != 0 {
			t.Fatalf("hourly rows after one-step down = %d, error = %v", hourly, err)
		}
		assertReady(false)
		if err := migrate.Run(ctx, []string{"up"}, loadURL); err != nil {
			t.Fatalf("up after one-step down: %v", err)
		}
		assertLibraryVersion(2)
		assertRows()
		assertReady(false)
		if _, err := db.Exec(ctx, `INSERT INTO binance_spot.sync_state (profile_key, status, last_succeeded_at) VALUES ('binance:spot:USDT:1h:UTC', 'succeeded', now())`); err != nil {
			t.Fatalf("record hourly resynchronization: %v", err)
		}
		assertReady(true)
	})
}
