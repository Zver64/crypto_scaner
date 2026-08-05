package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"crypto-scanner/internal/migrate"
	"crypto-scanner/internal/platform/config"
	"crypto-scanner/internal/storage/postgres"
)

func TestRunServicesMakesHTTPAvailableBeforeSchedulerStartup(t *testing.T) {
	listener := &acceptProbeListener{accepted: make(chan struct{}), closed: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	probe := &startupProbeScheduler{listener: listener, started: make(chan struct{})}
	result := make(chan error, 1)
	go func() {
		result <- runServices(ctx, listener, http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			response.WriteHeader(http.StatusOK)
		}), probe, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Second)
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

func TestNormalServerStartupDoesNotMutateUsers(t *testing.T) {
	databaseURL := os.Getenv("CRYPTO_SCANNER_TEST_DATABASE_URL")
	if databaseURL == "" || os.Getenv("CRYPTO_SCANNER_TEST_DATABASE_RESET_OK") != "1" {
		t.Skip("set CRYPTO_SCANNER_TEST_DATABASE_URL to a disposable empty database and CRYPTO_SCANNER_TEST_DATABASE_RESET_OK=1")
	}
	ctx := context.Background()
	loadDatabaseURL := func() (string, error) { return databaseURL, nil }
	if err := migrate.Run(ctx, []string{"up"}, loadDatabaseURL); err != nil {
		t.Fatalf("migrate disposable PostgreSQL: %v", err)
	}
	t.Cleanup(func() {
		if err := migrate.Run(context.Background(), []string{"down"}, loadDatabaseURL); err != nil {
			t.Errorf("reset disposable PostgreSQL: %v", err)
		}
	})
	db, err := postgres.OpenVerified(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open verified PostgreSQL: %v", err)
	}
	t.Cleanup(db.Close)

	updatedAt := time.Date(2024, time.March, 4, 5, 6, 7, 0, time.UTC)
	if _, err := db.Exec(ctx, `
		INSERT INTO app.users (telegram_id, username, display_name, is_enabled, created_at, updated_at)
		VALUES (222, 'disabled', 'Disabled User', FALSE, $1, $1)
	`, updatedAt); err != nil {
		t.Fatalf("seed disabled user: %v", err)
	}

	address := availableAddress(t)
	serverCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	cfg := config.Config{
		DatabaseURL:           databaseURL,
		TelegramBotToken:      "123456:test-token",
		TelegramWebhookSecret: "test-webhook-secret",
		AdminTelegramID:       222,
		MiniAppURL:            "https://scanner.example/app",
		HTTPAddress:           address,
		ShutdownTimeout:       time.Second,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	result := make(chan error, 1)
	go func() { result <- run(serverCtx, cfg, logger) }()
	waitUntilListening(t, address, result)
	cancel()
	if err := <-result; err != nil {
		t.Fatalf("stop normal server: %v", err)
	}

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
		t.Fatalf("normal startup mutated users: count=%d username=%q display_name=%q enabled=%t updated_at=%s", count, username, displayName, enabled, gotUpdatedAt)
	}
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
