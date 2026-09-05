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

	"crypto-scanner/internal/analysis"
	marketcapcriterion "crypto-scanner/internal/analysis/criteria/market_cap"
	"crypto-scanner/internal/analysis/criteria/volatility"
	authtelegram "crypto-scanner/internal/auth/telegram"
	"crypto-scanner/internal/exchange/binance"
	"crypto-scanner/internal/httpapi"
	marketsync "crypto-scanner/internal/market/sync"
	"crypto-scanner/internal/marketcap"
	"crypto-scanner/internal/platform/config"
	"crypto-scanner/internal/platform/envfile"
	"crypto-scanner/internal/platform/logging"
	"crypto-scanner/internal/storage/postgres"
	"crypto-scanner/internal/telegrambot"
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
	exchange := binance.NewWithOptions(binance.Options{RetryAttempts: cfg.SyncRetryAttempts})
	dailySynchronizer := marketsync.NewWithProfile(exchange, store, logger, cfg.SyncWorkers, marketsync.MVPProfile())
	hourlySynchronizer := marketsync.NewWithProfile(exchange, store, logger, cfg.SyncWorkers, marketsync.HourlyProfile())
	scheduler := marketsync.NewSchedulerWithHourly(dailySynchronizer, hourlySynchronizer, logger)
	marketCapResolver := marketcap.New(store, marketcap.NewClient("", cfg.CoinGeckoDemoAPIKey))
	criterionFactories := []analysis.Factory{volatility.New(), marketcapcriterion.New(marketCapResolver)}
	analysisService, err := analysis.NewService(store, criterionFactories...)
	if err != nil {
		return fmt.Errorf("initialize analysis service: %w", err)
	}
	authenticator := authtelegram.New(store, cfg.TelegramBotToken, cfg.TelegramInitDataMaxAge)
	botService, err := telegrambot.New(cfg.TelegramBotToken, cfg.AdminTelegramID, store, telegrambot.Options{Logger: logger})
	if err != nil {
		return fmt.Errorf("initialize Telegram bot: %w", err)
	}
	go func() {
		if err := marketCapResolver.BootstrapUntilComplete(ctx); err != nil && ctx.Err() == nil {
			logger.Warn("CoinGecko mapping bootstrap failed", "module", "market_cap", "error", err.Error())
		}
	}()

	listener, err := net.Listen("tcp", cfg.HTTPAddress)
	if err != nil {
		return fmt.Errorf("listen for HTTP: %w", err)
	}

	logger.InfoContext(ctx, "HTTP server starting",
		"module", "lifecycle",
		"operation", "start",
		"address", listener.Addr().String(),
	)
	if err := runServices(ctx, listener, httpapi.New(logger, store, analysisService, authenticator), scheduler, botService, logger, cfg.ShutdownTimeout); err != nil {
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
	botService scheduledService,
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
	botCtx, stopBot := context.WithCancel(context.Background())
	defer stopBot()
	botResult := make(chan error, 1)
	go func() { botResult <- botService.Run(botCtx) }()

	select {
	case <-ctx.Done():
		stopScheduler()
		stopBot()
		stopHTTP()
		schedulerErr := <-schedulerResult
		botErr := <-botResult
		httpErr := <-httpResult
		if schedulerErr != nil {
			return fmt.Errorf("stop market scheduler: %w", schedulerErr)
		}
		if botErr != nil {
			return fmt.Errorf("stop Telegram bot: %w", botErr)
		}
		return httpErr
	case err := <-httpResult:
		stopScheduler()
		stopBot()
		<-schedulerResult
		<-botResult
		return err
	case err := <-schedulerResult:
		stopBot()
		stopHTTP()
		<-botResult
		<-httpResult
		if err != nil {
			return fmt.Errorf("run market scheduler: %w", err)
		}
		return fmt.Errorf("market scheduler stopped unexpectedly")
	case err := <-botResult:
		stopScheduler()
		stopHTTP()
		<-schedulerResult
		<-httpResult
		if err != nil {
			return fmt.Errorf("run Telegram bot: %w", err)
		}
		return fmt.Errorf("Telegram bot stopped unexpectedly")
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
