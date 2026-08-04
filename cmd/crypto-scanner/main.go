// Command crypto-scanner is the entry point for the Crypto Scanner service.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"crypto-scanner/internal/httpapi"
	"crypto-scanner/internal/platform/config"
	"crypto-scanner/internal/platform/logging"
	"crypto-scanner/internal/storage/postgres"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		logFailure(logging.New(os.Stderr, "info"), "load_configuration", fmt.Errorf("load configuration: %w", err))
		os.Exit(1)
	}

	logger := logging.New(
		os.Stdout,
		cfg.LogLevel,
		cfg.DatabaseURL,
		cfg.TelegramBotToken,
		cfg.TelegramWebhookSecret,
	)
	if err := run(ctx, cfg, logger); err != nil {
		logFailure(logger, "run", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	database, err := postgres.OpenVerified(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("initialize PostgreSQL: %w", err)
	}
	defer database.Close()

	listener, err := net.Listen("tcp", cfg.HTTPAddress)
	if err != nil {
		return fmt.Errorf("listen for HTTP: %w", err)
	}

	logger.InfoContext(ctx, "HTTP server starting",
		"module", "lifecycle",
		"operation", "start",
		"address", listener.Addr().String(),
	)
	if err := httpapi.Serve(ctx, listener, httpapi.New(logger), logger, cfg.ShutdownTimeout); err != nil {
		return err
	}
	logger.Info("HTTP server stopped",
		"module", "lifecycle",
		"operation", "shutdown",
		"outcome", "success",
	)
	return nil
}

func logFailure(logger *slog.Logger, operation string, err error) {
	logger.Error("application failed",
		"module", "lifecycle",
		"operation", operation,
		"outcome", "failure",
		"error", err.Error(),
	)
}
