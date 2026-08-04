package postgres_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"crypto-scanner/internal/storage/postgres"

	"github.com/jackc/pgx/v5"
)

func TestVerifySchemaRejectsAnUninitializedDatabaseWithOperatorGuidance(t *testing.T) {
	queries := &querySequence{rows: []*stubRow{{values: []any{false}}}}

	err := postgres.VerifySchema(context.Background(), queries, "postgres://localhost/test")

	if err == nil || !strings.Contains(err.Error(), "schema is not initialized") || !strings.Contains(err.Error(), "make migrate-up") {
		t.Fatalf("VerifySchema() error = %v, want migration guidance", err)
	}
}

func TestVerifySchemaRejectsAnOutdatedDatabase(t *testing.T) {
	queries := &querySequence{rows: []*stubRow{
		{values: []any{true}},
		{values: []any{int64(0)}},
	}}

	err := postgres.VerifySchema(context.Background(), queries, "postgres://localhost/test")

	if err == nil || !strings.Contains(err.Error(), "version 0") || !strings.Contains(err.Error(), "version 1") {
		t.Fatalf("VerifySchema() error = %v, want current and required versions", err)
	}
}

func TestVerifySchemaAcceptsTheCurrentVersion(t *testing.T) {
	queries := &querySequence{rows: []*stubRow{
		{values: []any{true}},
		{values: []any{int64(1)}},
	}}

	if err := postgres.VerifySchema(context.Background(), queries, "postgres://localhost/test"); err != nil {
		t.Fatalf("VerifySchema() error = %v", err)
	}
}

func TestVerifySchemaRedactsKeywordDSNPasswordFromRuntimeErrors(t *testing.T) {
	const credential = "keyword-secret"
	queries := &querySequence{rows: []*stubRow{{err: errors.New("driver failed using password=" + credential)}}}
	err := postgres.VerifySchema(context.Background(), queries, "host=127.0.0.1 port=1 user=scanner password="+credential+" dbname=test")
	if err == nil || !strings.Contains(err.Error(), "verify PostgreSQL schema metadata") {
		t.Fatalf("VerifySchema() error = %v, want operation context", err)
	}
	if strings.Contains(err.Error(), credential) {
		t.Fatalf("VerifySchema() exposed keyword DSN credential: %v", err)
	}
}

func TestOpenRejectsAnInvalidURLWithoutEchoingCredentials(t *testing.T) {
	const credential = "must-not-leak"
	_, err := postgres.Open(context.Background(), "postgres://scanner:"+credential+"@%zz/database")
	if err == nil || !strings.Contains(err.Error(), "invalid DATABASE_URL") {
		t.Fatalf("Open() error = %v, want useful configuration context", err)
	}
	if strings.Contains(err.Error(), credential) {
		t.Fatalf("Open() exposed database credential: %v", err)
	}
}

type querySequence struct {
	rows []*stubRow
}

func (q *querySequence) QueryRow(context.Context, string, ...any) pgx.Row {
	if len(q.rows) == 0 {
		return &stubRow{err: errors.New("unexpected query")}
	}
	row := q.rows[0]
	q.rows = q.rows[1:]
	return row
}

type stubRow struct {
	values []any
	err    error
}

func (r *stubRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return errors.New("unexpected scan destination count")
	}
	for index, value := range r.values {
		switch target := dest[index].(type) {
		case *bool:
			*target = value.(bool)
		case *int64:
			*target = value.(int64)
		default:
			return errors.New("unexpected scan destination type")
		}
	}
	return nil
}
