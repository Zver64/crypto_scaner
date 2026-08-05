// Command crypto-scanner is the entry point for the Crypto Scanner service.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	telegrambot "crypto-scanner/internal/bot/telegram"
	"crypto-scanner/internal/exchange/binance"
	"crypto-scanner/internal/httpapi"
	marketsync "crypto-scanner/internal/market/sync"
	"crypto-scanner/internal/platform/config"
	"crypto-scanner/internal/platform/logging"
	"crypto-scanner/internal/storage/postgres"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if len(os.Args) > 1 {
		newRegistrar := func(token string) (telegrambot.WebhookRegistrar, error) {
			return telegrambot.NewClient(token)
		}
		if err := runCommand(ctx, os.Args[1:], config.LoadTelegramWebhook, newRegistrar); err != nil {
			logFailure(logging.New(os.Stderr, "info"), "telegram_set_webhook", err)
			os.Exit(1)
		}
		return
	}

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

func runCommand(
	ctx context.Context,
	args []string,
	loadConfig func() (config.TelegramWebhookConfig, error),
	newRegistrar func(string) (telegrambot.WebhookRegistrar, error),
) error {
	if len(args) != 2 || args[0] != "telegram" || args[1] != "set-webhook" {
		return fmt.Errorf("usage: crypto-scanner telegram set-webhook")
	}
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load Telegram webhook configuration: %w", err)
	}
	registrar, err := newRegistrar(cfg.BotToken)
	if err != nil {
		return err
	}
	return telegrambot.RegisterWebhook(ctx, registrar, cfg.PublicBaseURL, cfg.Secret)
}

func run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	database, err := postgres.OpenVerified(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("initialize PostgreSQL: %w", err)
	}
	defer database.Close()
	store := postgres.NewStore(database)
	botClient, err := telegrambot.NewClient(cfg.TelegramBotToken)
	if err != nil {
		return err
	}
	botHandler := telegrambot.NewUpdateHandler(cfg.MiniAppURL, botClient)
	webhook := telegrambot.NewWebhookHandler(cfg.TelegramWebhookSecret, botHandler)
	exchange := binance.NewWithOptions(binance.Options{RetryAttempts: cfg.SyncRetryAttempts})
	synchronizer := marketsync.NewWithOptions(exchange, store, logger, cfg.SyncWorkers)
	scheduler := marketsync.NewScheduler(synchronizer, logger)

	listener, err := net.Listen("tcp", cfg.HTTPAddress)
	if err != nil {
		return fmt.Errorf("listen for HTTP: %w", err)
	}

	logger.InfoContext(ctx, "HTTP server starting",
		"module", "lifecycle",
		"operation", "start",
		"address", listener.Addr().String(),
	)
	if err := runServices(ctx, listener, httpapi.New(logger, store, webhook), scheduler, logger, cfg.ShutdownTimeout); err != nil {
		return err
	}
	logger.Info("HTTP server stopped",
		"module", "lifecycle",
		"operation", "shutdown",
		"outcome", "success",
	)
	return nil
}

type scheduledService interface {
	Run(context.Context) error
}

func runServices(
	ctx context.Context,
	listener net.Listener,
	handler http.Handler,
	scheduler scheduledService,
	logger *slog.Logger,
	shutdownTimeout time.Duration,
) error {
	httpCtx, stopHTTP := context.WithCancel(context.Background())
	defer stopHTTP()
	schedulerCtx, stopScheduler := context.WithCancel(context.Background())
	defer stopScheduler()

	httpReady := make(chan struct{})
	listener = &acceptSignalingListener{Listener: listener, ready: httpReady}
	httpResult := make(chan error, 1)
	go func() { httpResult <- httpapi.Serve(httpCtx, listener, handler, logger, shutdownTimeout) }()
	select {
	case <-httpReady:
	case err := <-httpResult:
		return err
	case <-ctx.Done():
		stopHTTP()
		return <-httpResult
	}

	schedulerResult := make(chan error, 1)
	go func() { schedulerResult <- scheduler.Run(schedulerCtx) }()

	select {
	case <-ctx.Done():
		stopScheduler()
		stopHTTP()
		schedulerErr := <-schedulerResult
		httpErr := <-httpResult
		if schedulerErr != nil {
			return fmt.Errorf("stop market scheduler: %w", schedulerErr)
		}
		return httpErr
	case err := <-httpResult:
		stopScheduler()
		<-schedulerResult
		return err
	case err := <-schedulerResult:
		stopHTTP()
		<-httpResult
		if err != nil {
			return fmt.Errorf("run market scheduler: %w", err)
		}
		return fmt.Errorf("market scheduler stopped unexpectedly")
	}
}

type acceptSignalingListener struct {
	net.Listener
	once  sync.Once
	ready chan<- struct{}
}

func (listener *acceptSignalingListener) Accept() (net.Conn, error) {
	listener.once.Do(func() { close(listener.ready) })
	return listener.Listener.Accept()
}

func logFailure(logger *slog.Logger, operation string, err error) {
	logger.Error("application failed",
		"module", "lifecycle",
		"operation", operation,
		"outcome", "failure",
		"error", err.Error(),
	)
}
