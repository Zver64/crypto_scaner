package postgres_test

import (
	"context"
	"testing"

	"crypto-scanner/internal/storage/postgres"
)

const postgresIntegrationLock int64 = 739184027451

func lockDisposablePostgres(t *testing.T, db *postgres.DB) {
	t.Helper()
	conn, err := db.Acquire(context.Background())
	if err != nil {
		t.Fatalf("acquire PostgreSQL integration lock connection: %v", err)
	}
	if _, err := conn.Exec(context.Background(), "SELECT pg_advisory_lock($1)", postgresIntegrationLock); err != nil {
		conn.Release()
		t.Fatalf("acquire PostgreSQL integration lock: %v", err)
	}
	t.Cleanup(func() {
		if _, err := conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", postgresIntegrationLock); err != nil {
			t.Errorf("release PostgreSQL integration lock: %v", err)
		}
		conn.Release()
	})
}
