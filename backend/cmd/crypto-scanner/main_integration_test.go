package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"crypto-scanner/internal/migrate"
	"crypto-scanner/internal/platform/config"
	"crypto-scanner/internal/storage/postgres"
)

func TestValidateArgsRejectsRemovedTelegramWebhookCommand(t *testing.T) {
	err := validateArgs([]string{"telegram", "set-webhook"})
	if err == nil || err.Error() != "usage: crypto-scanner" {
		t.Fatalf("validateArgs() error = %v, want usage error", err)
	}
}

func TestRunServicesMakesHTTPAvailableBeforeSchedulerStartup(t *testing.T) {
	listener := &acceptProbeListener{accepted: make(chan struct{}), closed: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	probe := &startupProbeScheduler{listener: listener, started: make(chan struct{})}
	result := make(chan error, 1)
	go func() {
		result <- runServices(ctx, listener, http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			response.WriteHeader(http.StatusOK)
		}), probe, idleService{}, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Second)
	}()
	select {
	case <-probe.started:
	case <-time.After(time.Second):
		t.Fatal("scheduler did not observe the HTTP accept loop")
	}
	cancel()
	if err := <-result; err != nil {
		t.Fatalf("runServices() error = %v", err)
	}
}

type startupProbeScheduler struct {
	listener *acceptProbeListener
	started  chan struct{}
}

type idleService struct{}

func (idleService) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (scheduler *startupProbeScheduler) Run(ctx context.Context) error {
	select {
	case <-scheduler.listener.accepted:
	default:
		return errors.New("scheduler started before HTTP accept loop")
	}
	close(scheduler.started)
	<-ctx.Done()
	return nil
}

type acceptProbeListener struct {
	once     sync.Once
	accepted chan struct{}
	closed   chan struct{}
}

func (listener *acceptProbeListener) Accept() (net.Conn, error) {
	listener.once.Do(func() { close(listener.accepted) })
	<-listener.closed
	return nil, net.ErrClosed
}

func (listener *acceptProbeListener) Close() error {
	select {
	case <-listener.closed:
	default:
		close(listener.closed)
	}
	return nil
}

func (*acceptProbeListener) Addr() net.Addr { return dummyAddress("probe") }

type dummyAddress string

func (address dummyAddress) Network() string { return string(address) }
func (address dummyAddress) String() string  { return string(address) }

func TestNormalServerStartupBootstrapsConfiguredAdministrator(t *testing.T) {
	databaseURL := os.Getenv("CRYPTO_SCANNER_TEST_DATABASE_URL")
	if databaseURL == "" || os.Getenv("CRYPTO_SCANNER_TEST_DATABASE_RESET_OK") != "1" {
		t.Skip("set CRYPTO_SCANNER_TEST_DATABASE_URL to a disposable empty database and CRYPTO_SCANNER_TEST_DATABASE_RESET_OK=1")
	}
	// Keep real database/startup behavior, but never contact external APIs.
	// This test is deliberately not parallel because it replaces the default transport.
	transport := http.DefaultTransport
	http.DefaultTransport = startupTransport{}
	t.Cleanup(func() { http.DefaultTransport = transport })
	ctx := context.Background()
	db, err := postgres.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open disposable PostgreSQL: %v", err)
	}
	t.Cleanup(db.Close)
	lock, err := db.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Share the integration lock with the storage and migration test packages.
	const integrationLock int64 = 739184027451
	if _, err := lock.Exec(ctx, "SELECT pg_advisory_lock($1)", integrationLock); err != nil {
		lock.Release()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := lock.Exec(ctx, "SELECT pg_advisory_unlock($1)", integrationLock); err != nil {
			t.Errorf("release integration lock: %v", err)
		}
		lock.Release()
	})
	t.Cleanup(func() {
		if _, err := db.Exec(ctx, `DROP SCHEMA IF EXISTS app CASCADE; DROP SCHEMA IF EXISTS binance_spot CASCADE; DROP TABLE IF EXISTS public.crypto_scanner_schema_versions`); err != nil {
			t.Errorf("reset disposable PostgreSQL: %v", err)
		}
	})
	loadDatabaseURL := func() (string, error) { return databaseURL, nil }
	if err := migrate.Run(ctx, []string{"up"}, loadDatabaseURL); err != nil {
		t.Fatalf("migrate disposable PostgreSQL: %v", err)
	}

	updatedAt := time.Date(2024, time.March, 4, 5, 6, 7, 0, time.UTC)
	if _, err := db.Exec(ctx, `
		INSERT INTO app.users (telegram_id, username, display_name, is_enabled, created_at, updated_at)
		VALUES (222, 'disabled', 'Disabled User', FALSE, $1, $1)
	`, updatedAt); err != nil {
		t.Fatalf("seed disabled user: %v", err)
	}

	cfg := config.ServerConfig{
		DatabaseURL:      databaseURL,
		TelegramBotToken: "123456:test-token",
		AdminTelegramID:  111,
		ShutdownTimeout:  time.Second,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	startAndStop := func() {
		t.Helper()
		serverCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		cfg.HTTPAddress = availableAddress(t)
		result := make(chan error, 1)
		go func() { result <- run(serverCtx, cfg, logger) }()
		waitUntilListening(t, cfg.HTTPAddress, result)
		cancel()
		select {
		case err := <-result:
			if err != nil {
				t.Fatalf("stop normal server: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("normal server did not stop")
		}
	}
	startAndStop()

	var count int
	var username, displayName string
	var enabled bool
	var gotUpdatedAt time.Time
	if err := db.QueryRow(ctx, `
		SELECT count(*) OVER (), username, display_name, is_enabled, updated_at
		FROM app.users
		WHERE telegram_id = 222
	`).Scan(&count, &username, &displayName, &enabled, &gotUpdatedAt); err != nil {
		t.Fatalf("inspect user after startup: %v", err)
	}
	if count != 1 || username != "disabled" || displayName != "Disabled User" || enabled || !gotUpdatedAt.Equal(updatedAt) {
		t.Fatalf("normal startup changed unrelated users: count=%d username=%q display_name=%q enabled=%t updated_at=%s", count, username, displayName, enabled, gotUpdatedAt)
	}
	var administratorEnabled bool
	if err := db.QueryRow(ctx, `SELECT is_enabled FROM app.users WHERE telegram_id = 111`).Scan(&administratorEnabled); err != nil || !administratorEnabled {
		t.Fatalf("configured administrator was not bootstrapped: enabled=%t error=%v", administratorEnabled, err)
	}
	for _, enabled := range []bool{true, false} {
		if _, err := db.Exec(ctx, `UPDATE app.users SET username = 'admin', display_name = 'Existing Admin', is_enabled = $1, created_at = $2, updated_at = $2 WHERE telegram_id = 111`, enabled, updatedAt); err != nil {
			t.Fatal(err)
		}
		var before, after string
		if err := db.QueryRow(ctx, `SELECT row_to_json(u)::text FROM app.users u WHERE telegram_id = 111`).Scan(&before); err != nil {
			t.Fatal(err)
		}
		startAndStop()
		if err := db.QueryRow(ctx, `SELECT row_to_json(u)::text FROM app.users u WHERE telegram_id = 111`).Scan(&after); err != nil {
			t.Fatal(err)
		}
		if after != before {
			t.Fatalf("restart changed existing administrator (enabled=%t): before=%s after=%s", enabled, before, after)
		}
	}
}

type startupTransport struct{}

func (startupTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	// Consume and close uploads like a real transport, including Telegram's
	// pipe-backed multipart body, so its writer is not left blocked at shutdown.
	if request.Body != nil {
		_, err := io.Copy(io.Discard, request.Body)
		_ = request.Body.Close()
		if err != nil {
			return nil, err
		}
	}
	if request.URL.Host == "api.telegram.org" && strings.HasSuffix(request.URL.Path, "/getMe") {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"ok":true,"result":{"id":123456,"is_bot":true,"first_name":"Test","username":"test_bot"}}`)),
			Request:    request,
		}, nil
	}
	// Polling, market discovery and CoinGecko bootstrap wait for shutdown.
	<-request.Context().Done()
	return nil, request.Context().Err()
}

func availableAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve HTTP address: %v", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("release HTTP address: %v", err)
	}
	return address
}

func waitUntilListening(t *testing.T, address string, result <-chan error) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		connection, err := net.DialTimeout("tcp", address, 10*time.Millisecond)
		if err == nil {
			connection.Close()
			return
		}
		select {
		case runErr := <-result:
			t.Fatalf("server exited before listening: %v", runErr)
		default:
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("server did not listen on %s", address)
}
