// Command crypto-scanner is the entry point for the Crypto Scanner service.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"crypto-scanner/internal/opsnotify"
	"crypto-scanner/internal/platform/config"
	"crypto-scanner/internal/platform/envfile"
	"crypto-scanner/internal/platform/logging"
	"crypto-scanner/internal/storage/postgres"
)

// failureNotificationTimeout bounds the delivery of the failure that stops the
// process to the administrator.
const failureNotificationTimeout = 10 * time.Second

func main() {
	if err := envfile.LoadRoot(); err != nil {
		logFailure(logging.New(os.Stderr, "info", logging.Options{}), "load_environment_file", err)
		os.Exit(1)
	}
	if err := validateArgs(os.Args[1:]); err != nil {
		logFailure(logging.New(os.Stderr, "info", logging.Options{}), "validate_arguments", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if len(os.Args) > 1 {
		if err := healthcheck(ctx); err != nil {
			logFailure(logging.New(os.Stderr, "info", logging.Options{}), "healthcheck", err)
			os.Exit(1)
		}
		return
	}
	cfg, err := config.LoadServer()
	if err != nil {
		logFailure(logging.New(os.Stderr, "info", logging.Options{}), "load_configuration", fmt.Errorf("load configuration: %w", err))
		os.Exit(1)
	}

	secrets := []string{cfg.DatabaseURL, cfg.TelegramBotToken, cfg.CoinGeckoDemoAPIKey}
	if parsed, err := url.Parse(cfg.DatabaseURL); err == nil {
		// Errors may quote the database password raw or URL-escaped.
		if password, ok := parsed.User.Password(); ok {
			secrets = append(secrets, password, url.QueryEscape(password))
		}
	}
	queue := opsnotify.NewQueue()
	logger := logging.New(os.Stdout, cfg.LogLevel, logging.Options{Forward: queue.Forward}, secrets...)
	// The notifier logs its own failures without forwarding them back.
	notifierLogger := logging.New(os.Stdout, cfg.LogLevel, logging.Options{}, secrets...)
	if err := run(ctx, cfg, logger, queue, notifierLogger); err != nil {
		os.Exit(1)
	}
}

func validateArgs(args []string) error {
	if len(args) != 0 && (len(args) != 1 || args[0] != "healthcheck") {
		return fmt.Errorf("usage: crypto-scanner [healthcheck]")
	}
	return nil
}

// run logs its own failure. Once the application is built, the failure and
// any records still queued are also sent to the administrator, because the
// notifier has already stopped with the other services.
func run(ctx context.Context, cfg config.ServerConfig, logger *slog.Logger, queue *opsnotify.Queue, notifierLogger *slog.Logger) (err error) {
	var notifier *opsnotify.Notifier
	defer func() {
		if err == nil {
			return
		}
		logFailure(logger, "run", err)
		if notifier != nil {
			flushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), failureNotificationTimeout)
			defer cancel()
			notifier.Flush(flushCtx)
		}
	}()
	database, err := postgres.OpenVerified(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("initialize PostgreSQL: %w", err)
	}
	defer database.Close()
	store := postgres.NewStore(database)
	if err := store.BootstrapAdministrator(ctx, cfg.AdminTelegramID); err != nil {
		return err
	}
	application, err := buildApp(ctx, cfg, logger, store, queue, notifierLogger)
	if err != nil {
		return err
	}
	notifier = application.notifier
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
