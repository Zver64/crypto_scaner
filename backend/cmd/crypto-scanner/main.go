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

	"crypto-scanner/internal/platform/config"
	"crypto-scanner/internal/platform/envfile"
	"crypto-scanner/internal/platform/logging"
	"crypto-scanner/internal/storage/postgres"
)

func main() {
	if err := envfile.LoadRoot(); err != nil {
		logFailure(logging.New(os.Stderr, "info"), "load_environment_file", err)
		os.Exit(1)
	}
	if err := validateArgs(os.Args[1:]); err != nil {
		logFailure(logging.New(os.Stderr, "info"), "validate_arguments", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg, err := config.LoadServer()
	if err != nil {
		logFailure(logging.New(os.Stderr, "info"), "load_configuration", fmt.Errorf("load configuration: %w", err))
		os.Exit(1)
	}

	logger := logging.New(
		os.Stdout,
		cfg.LogLevel,
		cfg.DatabaseURL,
		cfg.TelegramBotToken,
	)
	if err := run(ctx, cfg, logger); err != nil {
		logFailure(logger, "run", err)
		os.Exit(1)
	}
}

func validateArgs(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: crypto-scanner")
	}
	return nil
}

func run(ctx context.Context, cfg config.ServerConfig, logger *slog.Logger) error {
	database, err := postgres.OpenVerified(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("initialize PostgreSQL: %w", err)
	}
	defer database.Close()
	store := postgres.NewStore(database)
	if err := store.BootstrapAdministrator(ctx, cfg.AdminTelegramID); err != nil {
		return err
	}
	application, err := buildApp(cfg, logger, store)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", cfg.HTTPAddress)
	if err != nil {
		return fmt.Errorf("listen for HTTP: %w", err)
	}

	logger.InfoContext(ctx, "HTTP server starting",
		"module", "lifecycle",
		"operation", "start",
		"address", listener.Addr().String(),
	)
	if err := runServices(ctx, listener, application.handler, logger, cfg.ShutdownTimeout, application.services...); err != nil {
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
