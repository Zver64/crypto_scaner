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
	"time"

	"crypto-scanner/internal/analysis"
	marketcapcriterion "crypto-scanner/internal/analysis/criteria/market_cap"
	"crypto-scanner/internal/analysis/criteria/volatility"
	authtelegram "crypto-scanner/internal/auth/telegram"
	"crypto-scanner/internal/exchange/binance"
	"crypto-scanner/internal/httpapi"
	"crypto-scanner/internal/market"
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
	synchronizers := make(map[market.CandleInterval]marketsync.Runner)
	for _, interval := range market.CandleIntervals() {
		profile := marketsync.Profile(interval)
		synchronizers[interval] = marketsync.NewWithProfile(exchange, store, logger, cfg.SyncWorkers, profile)
	}
	scheduler := marketsync.NewSchedulerWithProfiles(synchronizers, logger)
	coinMetadataResolver := marketcap.New(store, marketcap.NewClient("", cfg.CoinGeckoDemoAPIKey))
	coinMetadataSynchronizer, err := marketcap.NewCoinMetadataSynchronizer(coinMetadataResolver, store, logger, time.Hour, time.Minute)
	if err != nil {
		return fmt.Errorf("initialize coin metadata synchronizer: %w", err)
	}
	criterionFactories := []analysis.Factory{volatility.New(), marketcapcriterion.New()}
	analysisService, err := analysis.NewService(store, criterionFactories...)
	if err != nil {
		return fmt.Errorf("initialize analysis service: %w", err)
	}
	authenticator := authtelegram.New(store, cfg.TelegramBotToken, cfg.TelegramInitDataMaxAge)
	botService, err := telegrambot.New(cfg.TelegramBotToken, cfg.AdminTelegramID, store, telegrambot.Options{Logger: logger})
	if err != nil {
		return fmt.Errorf("initialize Telegram bot: %w", err)
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
	if err := runServices(ctx, listener, httpapi.NewWithOptions(logger, store, analysisService, store, authenticator, httpapi.Options{APIDocsEnabled: cfg.APIDocsEnabled}), scheduler, botService, coinMetadataSynchronizer, logger, cfg.ShutdownTimeout); err != nil {
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
