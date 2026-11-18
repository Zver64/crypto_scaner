package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"crypto-scanner/internal/alerts"
	"crypto-scanner/internal/analysis"
	marketcapcriterion "crypto-scanner/internal/analysis/criteria/marketcap"
	"crypto-scanner/internal/analysis/criteria/volatility"
	"crypto-scanner/internal/auth"
	authtelegram "crypto-scanner/internal/auth/telegram"
	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/closedindicator"
	"crypto-scanner/internal/coingecko"
	"crypto-scanner/internal/exchange/binance"
	"crypto-scanner/internal/favorites"
	"crypto-scanner/internal/httpapi"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/market/gridlimits"
	marketlive "crypto-scanner/internal/market/live"
	"crypto-scanner/internal/market/marketsync"
	"crypto-scanner/internal/market/perpetuals"
	"crypto-scanner/internal/market/retention"
	"crypto-scanner/internal/marketcap"
	"crypto-scanner/internal/markettable"
	"crypto-scanner/internal/opsnotify"
	"crypto-scanner/internal/platform/config"
	"crypto-scanner/internal/scannerindicator"
	"crypto-scanner/internal/storage/postgres"
	"crypto-scanner/internal/strategy"
	"crypto-scanner/internal/telegrambot"
	"crypto-scanner/internal/users"
)

// app is the composed process: the HTTP handler and its background services.
type app struct {
	handler  http.Handler
	services []service
	// notifier runs apart from services: run stops it after them and the HTTP
	// server, so it delivers the records they log while stopping.
	notifier *opsnotify.Notifier
}

func buildApp(ctx context.Context, cfg config.ServerConfig, logger *slog.Logger, store *postgres.Store, queue *opsnotify.Queue, notifierLogger *slog.Logger) (app, error) {
	indicatorRegistry, err := indicator.NewRegistry(indicatorModules()...)
	if err != nil {
		return app{}, fmt.Errorf("initialize indicator registry: %w", err)
	}
	drawers, err := chartDrawers(indicatorRegistry)
	if err != nil {
		return app{}, fmt.Errorf("initialize chart drawers: %w", err)
	}
	scannerIndicators, err := scannerindicator.New(store, indicatorRegistry, drawers, logger)
	if err != nil {
		return app{}, fmt.Errorf("initialize scanner indicators: %w", err)
	}
	if err := scannerIndicators.Load(ctx); err != nil {
		return app{}, err
	}
	// Tables calculate the values they show on demand; only the values
	// enabled strategies read stay current in the background.
	closedIndicators, err := closedindicator.New(store, indicatorRegistry, scannerIndicators.TableTargets(), logger)
	if err != nil {
		return app{}, fmt.Errorf("initialize closed indicator tracker: %w", err)
	}
	administrator, err := store.FindByTelegramID(ctx, cfg.AdminTelegramID)
	if err != nil {
		return app{}, fmt.Errorf("find administrator: %w", err)
	}
	strategies, err := strategy.NewService(store, scannerIndicators, indicatorRegistry, drawers, closedIndicators, logger, administrator.ID)
	if err != nil {
		return app{}, fmt.Errorf("initialize strategies: %w", err)
	}
	if err := strategies.Load(ctx); err != nil {
		return app{}, err
	}
	marketTable, err := markettable.NewCatalog(scannerIndicators, defaultTableSort, marketTableColumns(markettable.ExchangeLink{})...)
	if err != nil {
		return app{}, fmt.Errorf("initialize market table: %w", err)
	}
	coinMTable, err := markettable.NewCatalog(scannerIndicators, defaultTableSort, marketTableColumns(markettable.CoinMPerpetualLink{})...)
	if err != nil {
		return app{}, fmt.Errorf("initialize COIN-M table: %w", err)
	}
	favoritesTable, err := markettable.NewCatalog(scannerIndicators, defaultTableSort, favoritesTableColumns()...)
	if err != nil {
		return app{}, fmt.Errorf("initialize favorites table: %w", err)
	}
	// Kline and trade connections share the process-wide Binance dial budget.
	dialLimiter := binance.NewDialLimiter()
	klineStream := binance.NewKlineStream(logger, dialLimiter)
	exchange := binance.New(logger, binance.Options{RetryAttempts: cfg.SyncRetryAttempts})
	liveService := marketlive.New(klineStream, exchange, store, logger, marketlive.Options{})
	// Synchronization and history loads write candles only through syncStore,
	// which reports their changes.
	syncStore := marketsync.NewObservableStore(store)
	synchronizers := make(map[market.CandleInterval]marketsync.Runner)
	historyExtenders := make(map[market.CandleInterval]marketsync.HistoryExtender)
	for _, interval := range market.CandleIntervals() {
		profile := market.BinanceSpotSyncProfile(interval)
		synchronizers[interval] = marketsync.New(exchange, syncStore, logger, cfg.SyncWorkers, market.SyncDepth, profile)
		historyExtenders[interval] = marketsync.NewExtender(exchange, syncStore, market.SyncDepth, profile)
	}
	scheduler := marketsync.NewScheduler(synchronizers, logger)
	historyLoader := marketsync.NewHistoryLoader(store, historyExtenders, logger)

	coinMetadataSynchronizer, err := marketcap.NewCoinMetadataSynchronizer(marketcap.New(store, coingecko.NewClient("", cfg.CoinGeckoDemoAPIKey)), store, logger, time.Hour, 15*time.Minute)
	if err != nil {
		return app{}, fmt.Errorf("initialize coin metadata synchronizer: %w", err)
	}
	analysisService, err := analysis.NewService(store, volatility.New(), marketcapcriterion.New())
	if err != nil {
		return app{}, fmt.Errorf("initialize analysis service: %w", err)
	}
	chartService, err := chart.NewService(store, indicatorRegistry, drawers, scannerIndicators, logger)
	if err != nil {
		return app{}, fmt.Errorf("initialize chart service: %w", err)
	}

	userService := users.New(store, cfg.AdminTelegramID)
	botService, err := telegrambot.New(ctx, cfg.TelegramBotToken, cfg.AdminTelegramID, store, userService, logger, telegrambot.Options{})
	if err != nil {
		return app{}, fmt.Errorf("initialize Telegram bot: %w", err)
	}
	telegramNotifier := botService.Notifier()
	notifier, err := opsnotify.New(queue, telegramNotifier, botService.Coins(), notifierLogger)
	if err != nil {
		return app{}, err
	}
	tradeStream := binance.NewTradeStream(logger, dialLimiter)
	alertMonitor := alerts.NewMonitor(store, tradeStream, telegramNotifier, botService.Coins(), logger)
	strategyMonitor := strategy.NewMonitor(store, store, indicatorRegistry, closedIndicators, strategies, telegramNotifier, botService.Coins(), cfg.AdminTelegramID, logger)
	favoriteService, err := favorites.New(store)
	if err != nil {
		return app{}, fmt.Errorf("initialize favorites service: %w", err)
	}
	coinM := binance.NewCoinM()
	coinMPerpetuals, err := perpetuals.NewSynchronizer(coinM, store, logger, 24*time.Hour, time.Minute)
	if err != nil {
		return app{}, fmt.Errorf("initialize COIN-M perpetuals synchronizer: %w", err)
	}
	marketTables, err := markettable.NewService(analysisService, closedIndicators, store, store, store, favoriteService,
		markettable.Catalogs{Market: marketTable, CoinM: coinMTable, Favorites: favoritesTable})
	if err != nil {
		return app{}, fmt.Errorf("initialize market table service: %w", err)
	}
	alertService, err := alerts.New(store, alertMonitor)
	if err != nil {
		return app{}, fmt.Errorf("initialize price alert service: %w", err)
	}

	// Events, once every service exists and before any runs. Listeners never
	// block, since HTTP handlers raise access and favorite changes.
	// Configuration changes before the tracker existed are covered by its
	// initial targets.
	scannerIndicators.Listen(func() {
		if err := closedIndicators.SetTargets(scannerIndicators.TableTargets()); err != nil {
			logger.Error("apply table indicator targets failed", "module", "scanner_indicator", "error", err)
		}
	})
	// Strategy changes before the monitor existed are covered by its first
	// evaluation.
	strategies.Listen(strategyMonitor.StrategiesChanged)
	syncStore.OnCandlesChanged(func(candles []market.Candle) {
		liveService.HistoryChanged(candles)
		closedIndicators.HistoryChanged(candles)
	})
	// The tracker recalculates once a round or a history load of the live
	// history has committed all its changes.
	syncStore.OnHistorySynced(closedIndicators.HistorySynced)
	syncStore.OnInstrumentsChanged(func(instruments []market.Instrument) {
		strategies.InstrumentsChanged(instruments)
		strategyMonitor.Changed()
		alertMonitor.Changed()
	})
	// Access changes change the monitored instruments through the favorites
	// of the user.
	userService.OnChanged(alertMonitor.Changed)
	// Favorites change the monitored instruments, and the administrator's
	// those signals of the favorites evaluate.
	favoritesChanged := func(ctx context.Context, userID int64) {
		alertMonitor.Changed()
		strategies.FavoritesChanged(ctx, userID)
	}
	favoriteService.OnChanged(favoritesChanged)
	alertService.OnFavoritesChanged(favoritesChanged)

	sessions := auth.NewSessions(store, authtelegram.New(cfg.TelegramBotToken, cfg.TelegramInitDataMaxAge, authtelegram.Options{}),
		cfg.AdminTelegramID, cfg.SessionIdleTTL, cfg.SessionAbsoluteTTL, logger, auth.SessionOptions{})
	handler, err := httpapi.New(logger, httpapi.Dependencies{
		Readiness:    store,
		Analysis:     analysisService,
		MarketTables: marketTables,
		History:      store,
		GridLimits:   gridlimits.New(exchange, binance.NewUsdM(), coinM, store, logger, gridlimits.Options{}),
		Sessions:     sessions,
		APITokens:    sessions,
		Chart:        chartService,
		LiveCandles:  liveService,
		Favorites:    favoriteService,
		Alerts:       alertService,

		ScannerIndicators: scannerIndicators,
		IndicatorTypes:    indicatorRegistry,
		Users:             userService,
		Strategies:        strategies,
		HistoryLoads:      historyLoader,
	}, httpapi.Options{APIDocsEnabled: cfg.APIDocsEnabled})
	if err != nil {
		return app{}, fmt.Errorf("initialize HTTP API: %w", err)
	}
	return app{handler: handler, notifier: notifier, services: []service{
		{"market scheduler", scheduler},
		{"live kline stream", klineStream},
		{"live candle service", liveService},
		{"live trade stream", tradeStream},
		{"alert monitor", alertMonitor},
		{"closed indicator tracker", closedIndicators},
		{"strategy monitor", strategyMonitor},
		{"Telegram bot", botService},
		{"coin metadata synchronizer", coinMetadataSynchronizer},
		{"COIN-M perpetuals synchronizer", coinMPerpetuals},
		{"market history loader", historyLoader},
		{"market retention", retention.New(store, logger, market.RetentionDepth)},
		{"session pruner", sessions},
	}}, nil
}
