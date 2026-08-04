package bootstrapadmin_test

import (
	"context"
	"os"
	"testing"
	"time"

	"crypto-scanner/internal/bootstrapadmin"
	"crypto-scanner/internal/migrate"
	"crypto-scanner/internal/platform/config"
	"crypto-scanner/internal/storage/postgres"
)

func TestStandaloneCommandBootstrapsOnlyTheConfiguredAdministrator(t *testing.T) {
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

	var appSchema, marketSchema bool
	if err := db.QueryRow(ctx, `SELECT to_regnamespace('app') IS NOT NULL, to_regnamespace('binance_spot') IS NOT NULL`).Scan(&appSchema, &marketSchema); err != nil {
		t.Fatalf("inspect disposable database: %v", err)
	}
	if appSchema || marketSchema {
		t.Fatalf("integration database is not empty: app=%t binance_spot=%t", appSchema, marketSchema)
	}

	const adminID int64 = 987654321
	loadConfig := func() (config.AdminBootstrapConfig, error) {
		return config.AdminBootstrapConfig{DatabaseURL: databaseURL, AdminTelegramID: adminID}, nil
	}
	if err := bootstrapadmin.Run(ctx, loadConfig); err == nil {
		t.Fatal("Run() accepted an unmigrated PostgreSQL database")
	}

	loadDatabaseURL := func() (string, error) { return databaseURL, nil }
	if err := migrate.Run(ctx, []string{"up"}, loadDatabaseURL); err != nil {
		t.Fatalf("migrate disposable PostgreSQL: %v", err)
	}
	t.Cleanup(func() {
		if err := migrate.Run(context.Background(), []string{"down"}, loadDatabaseURL); err != nil {
			t.Errorf("reset disposable PostgreSQL: %v", err)
		}
	})

	unchangedAt := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)
	if _, err := db.Exec(ctx, `
		INSERT INTO app.users (telegram_id, username, display_name, is_enabled, created_at, updated_at)
		VALUES (111, 'unrelated', 'Unrelated User', FALSE, $1, $1)
	`, unchangedAt); err != nil {
		t.Fatalf("seed unrelated user: %v", err)
	}

	if err := bootstrapadmin.Run(ctx, loadConfig); err != nil {
		t.Fatalf("create administrator: %v", err)
	}
	assertAdmin(t, ctx, db, adminID, true, 1)
	assertUnrelatedUnchanged(t, ctx, db, unchangedAt)

	var firstID int64
	var firstUpdatedAt time.Time
	if err := db.QueryRow(ctx, `SELECT id, updated_at FROM app.users WHERE telegram_id = $1`, adminID).Scan(&firstID, &firstUpdatedAt); err != nil {
		t.Fatalf("read created administrator: %v", err)
	}
	firstSequence := readUsersSequence(t, ctx, db)
	if err := bootstrapadmin.Run(ctx, loadConfig); err != nil {
		t.Fatalf("repeat administrator bootstrap: %v", err)
	}
	assertAdmin(t, ctx, db, adminID, true, 1)
	var repeatedID int64
	var repeatedUpdatedAt time.Time
	if err := db.QueryRow(ctx, `SELECT id, updated_at FROM app.users WHERE telegram_id = $1`, adminID).Scan(&repeatedID, &repeatedUpdatedAt); err != nil {
		t.Fatalf("read repeated administrator: %v", err)
	}
	if repeatedID != firstID || !repeatedUpdatedAt.Equal(firstUpdatedAt) {
		t.Fatalf("idempotent bootstrap rewrote administrator: id=%d updated_at=%s, want id=%d updated_at=%s", repeatedID, repeatedUpdatedAt, firstID, firstUpdatedAt)
	}
	if repeatedSequence := readUsersSequence(t, ctx, db); repeatedSequence != firstSequence {
		t.Fatalf("idempotent bootstrap advanced users sequence: got %#v, want %#v", repeatedSequence, firstSequence)
	}
	assertUnrelatedUnchanged(t, ctx, db, unchangedAt)

	disabledAt := time.Date(2024, time.February, 3, 4, 5, 6, 0, time.UTC)
	if _, err := db.Exec(ctx, `UPDATE app.users SET is_enabled = FALSE, updated_at = $2 WHERE telegram_id = $1`, adminID, disabledAt); err != nil {
		t.Fatalf("disable administrator: %v", err)
	}
	if err := bootstrapadmin.Run(ctx, loadConfig); err != nil {
		t.Fatalf("re-enable administrator: %v", err)
	}
	assertAdmin(t, ctx, db, adminID, true, 1)
	var enabledAt time.Time
	if err := db.QueryRow(ctx, `SELECT updated_at FROM app.users WHERE telegram_id = $1`, adminID).Scan(&enabledAt); err != nil {
		t.Fatalf("read re-enabled administrator: %v", err)
	}
	if !enabledAt.After(disabledAt) {
		t.Fatalf("re-enabled administrator updated_at = %s, want after %s", enabledAt, disabledAt)
	}
	assertUnrelatedUnchanged(t, ctx, db, unchangedAt)
}

type sequenceState struct {
	LastValue int64
	Called    bool
}

func readUsersSequence(t *testing.T, ctx context.Context, db *postgres.DB) sequenceState {
	t.Helper()
	var state sequenceState
	if err := db.QueryRow(ctx, `SELECT last_value, is_called FROM app.users_id_seq`).Scan(&state.LastValue, &state.Called); err != nil {
		t.Fatalf("read users sequence: %v", err)
	}
	return state
}

func assertAdmin(t *testing.T, ctx context.Context, db *postgres.DB, telegramID int64, enabled bool, wantCount int) {
	t.Helper()
	var count int
	var gotEnabled bool
	if err := db.QueryRow(ctx, `
		SELECT count(*), bool_and(is_enabled)
		FROM app.users
		WHERE telegram_id = $1
	`, telegramID).Scan(&count, &gotEnabled); err != nil {
		t.Fatalf("inspect administrator: %v", err)
	}
	if count != wantCount || gotEnabled != enabled {
		t.Fatalf("administrator count=%d enabled=%t, want count=%d enabled=%t", count, gotEnabled, wantCount, enabled)
	}
}

func assertUnrelatedUnchanged(t *testing.T, ctx context.Context, db *postgres.DB, wantUpdatedAt time.Time) {
	t.Helper()
	var username, displayName string
	var enabled bool
	var updatedAt time.Time
	if err := db.QueryRow(ctx, `
		SELECT username, display_name, is_enabled, updated_at
		FROM app.users
		WHERE telegram_id = 111
	`).Scan(&username, &displayName, &enabled, &updatedAt); err != nil {
		t.Fatalf("read unrelated user: %v", err)
	}
	if username != "unrelated" || displayName != "Unrelated User" || enabled || !updatedAt.Equal(wantUpdatedAt) {
		t.Fatalf("unrelated user changed: username=%q display_name=%q enabled=%t updated_at=%s", username, displayName, enabled, updatedAt)
	}
}
