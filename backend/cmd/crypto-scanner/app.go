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

	"github.com/google/uuid"
)

// app is the composed process: the HTTP handler and its background services.
type app struct {
	handler  http.Handler
	services []service
	// notifier runs apart from services: run stops it after them and the HTTP
	// server, so it delivers the records they log while stopping.
	notifier *opsnotify.Notifier
}

// listeners fans one notification out to listeners registered after the
// notifier was handed out. All registration happens before services run.
type listeners []func()

func (l *listeners) notify() {
	for _, listener := range *l {
		listener()
	}
}

func buildApp(ctx context.Context, cfg config.ServerConfig, logger *slog.Logger, store *postgres.Store, queue *opsnotify.Queue, notifierLogger *slog.Logger) (app, error) {
	modules, err := indicatorModules()
	if err != nil {
		return app{}, fmt.Errorf("initialize indicator registry: %w", err)
	}
	indicatorRegistry, err := indicator.NewRegistry(modules...)
	if err != nil {
		return app{}, fmt.Errorf("initialize indicator registry: %w", err)
	}
	// The tracker is created after the configuration loads; changes before
	// that are covered by its initial targets.
	var closedIndicators *closedindicator.Tracker
	// Strategies read the configured indicators, which stay while in use.
	var strategies *strategy.Service
	var strategyMonitor *strategy.Monitor
	var alertMonitor *alerts.Monitor
	var scannerIndicators *scannerindicator.Service
	scannerIndicators, err = scannerindicator.New(store, indicatorRegistry, chartPalette, logger, func() {
		if closedIndicators == nil {
			return
		}
		if err := closedIndicators.SetTargets(scannerIndicators.TableTargets()); err != nil {
			logger.Error("apply table indicator targets failed", "module", "scanner_indicator", "error", err)
		}
	}, func() map[int64][]string {
		if strategies == nil {
			return nil
		}
		return strategies.IndicatorUsage()
	})
	if err != nil {
		return app{}, fmt.Errorf("initialize scanner indicators: %w", err)
	}
	if err := scannerIndicators.Load(ctx); err != nil {
		return app{}, err
	}
	// Tables calculate the values they show on demand; only the values
	// enabled strategies read stay current in the background.
	closedIndicators, err = closedindicator.New(store, indicatorRegistry, scannerIndicators.TableTargets(), logger)
	if err != nil {
		return app{}, fmt.Errorf("initialize closed indicator tracker: %w", err)
	}
	administrator, err := store.FindByTelegramID(ctx, cfg.AdminTelegramID)
	if err != nil {
		return app{}, fmt.Errorf("find administrator: %w", err)
	}
	// Changes before the monitor exists are covered by its first evaluation.
	strategies, err = strategy.NewService(store, scannerIndicators, indicatorRegistry, closedIndicators, logger, administrator.ID, func(baselines []uuid.UUID) {
		if strategyMonitor != nil {
			strategyMonitor.StrategiesChanged(baselines)
		}
	})
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
	liveService := marketlive.New(klineStream, store, logger, marketlive.Options{})
	exchange := binance.New(logger, binance.Options{RetryAttempts: cfg.SyncRetryAttempts})
	syncStore := &marketsync.ObservableStore{Store: store, Changed: func(candles []market.Candle) {
		liveService.HistoryChanged(candles)
		closedIndicators.HistoryChanged(candles)
	}, InstrumentsChanged: func(instruments []market.Instrument) {
		// Synchronization runs only once every service exists.
		strategies.InstrumentsChanged(instruments)
		strategyMonitor.Changed()
		alertMonitor.Changed()
	}}
	synchronizers := make(map[market.CandleInterval]marketsync.Runner)
	// History loads notify only when completing a partial live history;
	// pages beyond SyncDepth do not affect tracked values.
	historyExtenders := make(map[market.CandleInterval]marketsync.HistoryExtender)
	for _, interval := range market.CandleIntervals() {
		// The tracker recalculates once a round has committed all its changes.
		synchronizers[interval] = marketsync.ObservableRunner{
			Runner: marketsync.New(exchange, syncStore, logger, cfg.SyncWorkers, market.SyncDepth, market.BinanceSpotSyncProfile(interval)),
			Synced: func() { closedIndicators.HistorySynced(interval) },
		}
		historyExtenders[interval] = marketsync.New(exchange, store, logger, cfg.SyncWorkers, market.SyncDepth, market.BinanceSpotSyncProfile(interval))
	}
	scheduler := marketsync.NewScheduler(synchronizers, logger)
	historyLoader := marketsync.NewHistoryLoader(store, historyExtenders, logger, marketsync.HistoryLoaderOptions{
		Changed: syncStore.Changed,
		Synced:  closedIndicators.HistorySynced,
	})

	coinMetadataSynchronizer, err := marketcap.NewCoinMetadataSynchronizer(marketcap.New(store, coingecko.NewClient("", cfg.CoinGeckoDemoAPIKey)), store, logger, time.Hour, 15*time.Minute)
	if err != nil {
		return app{}, fmt.Errorf("initialize coin metadata synchronizer: %w", err)
	}
	analysisService, err := analysis.NewService(store, closedIndicators, volatility.New(), marketcapcriterion.New())
	if err != nil {
		return app{}, fmt.Errorf("initialize analysis service: %w", err)
	}
	chartService, err := chart.NewService(store, indicatorRegistry, scannerIndicators, logger)
	if err != nil {
		return app{}, fmt.Errorf("initialize chart service: %w", err)
	}

	// Access and favorite changes both change the monitored instrument set.
	var monitoredChanged listeners
	botService, err := telegrambot.New(ctx, cfg.TelegramBotToken, cfg.AdminTelegramID, store, logger, monitoredChanged.notify, telegrambot.Options{})
	if err != nil {
		return app{}, fmt.Errorf("initialize Telegram bot: %w", err)
	}
	notifier, err := opsnotify.New(queue, botService, botService.Coins(), notifierLogger)
	if err != nil {
		return app{}, err
	}
	tradeStream := binance.NewTradeStream(logger, dialLimiter)
	alertMonitor = alerts.NewMonitor(store, tradeStream, botService, logger)
	strategyMonitor = strategy.NewMonitor(store, store, indicatorRegistry, closedIndicators, strategies, botService, botService.Coins(), cfg.AdminTelegramID, logger)
	monitoredChanged = append(monitoredChanged, alertMonitor.Changed)
	// Favorites change the monitored instruments, and the administrator's
	// those signals of the favorites evaluate.
	favoritesChanged := func(ctx context.Context, userID int64) {
		monitoredChanged.notify()
		strategies.FavoritesChanged(ctx, userID)
	}
	favoriteService, err := favorites.New(store, favoritesChanged)
	if err != nil {
		return app{}, fmt.Errorf("initialize favorites service: %w", err)
	}
	coinMPerpetuals, err := perpetuals.NewSynchronizer(binance.NewCoinMPerpetuals(), store, logger, 24*time.Hour, time.Minute)
	if err != nil {
		return app{}, fmt.Errorf("initialize COIN-M perpetuals synchronizer: %w", err)
	}
	marketTables, err := markettable.NewService(analysisService, closedIndicators, store, marketTable, coinMTable, favoritesTable)
	if err != nil {
		return app{}, fmt.Errorf("initialize market table service: %w", err)
	}
	alertService, err := alerts.New(store, alertMonitor, favoritesChanged)
	if err != nil {
		return app{}, fmt.Errorf("initialize price alert service: %w", err)
	}

	sessions := auth.NewSessions(store, authtelegram.New(cfg.TelegramBotToken, cfg.TelegramInitDataMaxAge, authtelegram.Options{}),
		cfg.AdminTelegramID, cfg.SessionIdleTTL, cfg.SessionAbsoluteTTL, logger, auth.SessionOptions{})
	handler := httpapi.New(logger, httpapi.Dependencies{
		Readiness:    store,
		Analysis:     analysisService,
		MarketTables: marketTables,
		History:      store,
		GridLimits:   gridlimits.New(exchange, store, logger, gridlimits.Options{}),
		Sessions:     sessions,
		APITokens:    sessions,
		Chart:        chartService,
		LiveCandles:  liveService,
		Favorites:    favoriteService,
		Alerts:       alertService,

		ScannerIndicators: scannerIndicators,
		IndicatorTypes:    indicatorRegistry,
		Users:             users.New(store, cfg.AdminTelegramID, monitoredChanged.notify),
		Strategies:        strategies,
		HistoryLoads:      historyLoader,
	}, httpapi.Options{APIDocsEnabled: cfg.APIDocsEnabled})
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
