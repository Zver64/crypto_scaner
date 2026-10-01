package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"crypto-scanner/internal/alerts"
	"crypto-scanner/internal/analysis"
	marketcapcriterion "crypto-scanner/internal/analysis/criteria/marketcap"
	"crypto-scanner/internal/analysis/criteria/volatility"
	authtelegram "crypto-scanner/internal/auth/telegram"
	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/closedindicator"
	"crypto-scanner/internal/coingecko"
	"crypto-scanner/internal/exchange/binance"
	"crypto-scanner/internal/favorites"
	"crypto-scanner/internal/httpapi"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
	marketlive "crypto-scanner/internal/market/live"
	"crypto-scanner/internal/market/marketsync"
	"crypto-scanner/internal/market/retention"
	"crypto-scanner/internal/marketcap"
	"crypto-scanner/internal/markettable"
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
}

// listeners fans one notification out to listeners registered after the
// notifier was handed out. All registration happens before services run.
type listeners []func()

func (l *listeners) notify() {
	for _, listener := range *l {
		listener()
	}
}

func buildApp(ctx context.Context, cfg config.ServerConfig, logger *slog.Logger, store *postgres.Store) (app, error) {
	indicatorRegistry, err := indicator.NewRegistry(indicatorModules()...)
	if err != nil {
		return app{}, fmt.Errorf("initialize indicator registry: %w", err)
	}
	// The tracker is created after the configuration loads; changes before
	// that are covered by its initial targets.
	var closedIndicators *closedindicator.Tracker
	var tables []markettable.Catalog
	// Strategies read the configured indicators, which stay while in use.
	var strategies *strategy.Service
	var strategyMonitor *strategy.Monitor
	scannerIndicators, err := scannerindicator.New(store, indicatorRegistry, chartPalette, logger, func() {
		if closedIndicators == nil {
			return
		}
		if err := closedIndicators.SetTargets(tableTargets(tables)); err != nil {
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
	// Changes before the monitor exists are covered by its first evaluation.
	strategies, err = strategy.NewService(store, scannerIndicators, logger, func(baselines []int64) {
		if closedIndicators != nil {
			closedIndicators.Refresh()
		}
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
	marketTable, err := markettable.NewCatalog(indicatorRegistry, scannerIndicators, defaultTableSort, marketTableColumns()...)
	if err != nil {
		return app{}, fmt.Errorf("initialize market table: %w", err)
	}
	favoritesTable, err := markettable.NewCatalog(indicatorRegistry, scannerIndicators, defaultTableSort, favoritesTableColumns()...)
	if err != nil {
		return app{}, fmt.Errorf("initialize favorites table: %w", err)
	}
	// Tables calculate the values they show on demand; only the values
	// enabled strategies read stay current in the background.
	tables = []markettable.Catalog{marketTable, favoritesTable}
	closedIndicators, err = closedindicator.New(store, indicatorRegistry, tableTargets(tables), logger,
		strategySource{closedindicator.InstrumentSource(store.ListMonitoredInstrumentIDs), strategies})
	if err != nil {
		return app{}, fmt.Errorf("initialize closed indicator tracker: %w", err)
	}

	// Kline and trade connections share the process-wide Binance dial budget.
	dialLimiter := binance.NewDialLimiter()
	klineStream := binance.NewKlineStream(logger, dialLimiter)
	liveService := marketlive.New(klineStream, store, logger, marketlive.Options{})
	exchange := binance.New(binance.Options{RetryAttempts: cfg.SyncRetryAttempts})
	syncStore := marketsync.ObservableStore{Store: store, Changed: func(candles []market.Candle) {
		liveService.HistoryChanged(candles)
		closedIndicators.HistoryChanged(candles)
	}}
	synchronizers := make(map[market.CandleInterval]marketsync.Runner)
	for _, interval := range market.CandleIntervals() {
		synchronizers[interval] = marketsync.New(exchange, syncStore, logger, cfg.SyncWorkers, market.HistoryDepth, market.BinanceSpotSyncProfile(interval))
	}
	scheduler := marketsync.NewScheduler(synchronizers, logger)

	coinMetadataSynchronizer, err := marketcap.NewCoinMetadataSynchronizer(marketcap.New(store, coingecko.NewClient("", cfg.CoinGeckoDemoAPIKey)), store, logger, time.Hour, time.Minute)
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
	botService, err := telegrambot.New(cfg.TelegramBotToken, cfg.AdminTelegramID, store, logger, telegrambot.Options{AccessChanged: monitoredChanged.notify})
	if err != nil {
		return app{}, fmt.Errorf("initialize Telegram bot: %w", err)
	}
	tradeStream := binance.NewTradeStream(logger, dialLimiter)
	alertMonitor := alerts.NewMonitor(store, tradeStream, botService, logger)
	strategyMonitor = strategy.NewMonitor(store, closedIndicators, strategies, botService, cfg.AdminTelegramID, logger)
	monitoredChanged = append(monitoredChanged, alertMonitor.Changed, closedIndicators.Refresh, strategyMonitor.Changed)
	favoriteService := favorites.New(store, monitoredChanged.notify, analysisService, closedIndicators, favoritesTable)

	handler := httpapi.New(logger, httpapi.Dependencies{
		Readiness:     store,
		Analysis:      analysisService,
		MarketTables:  markettable.NewService(analysisService, marketTable),
		History:       store,
		Authenticator: authtelegram.New(store, cfg.TelegramBotToken, cfg.TelegramInitDataMaxAge, cfg.AdminTelegramID, authtelegram.Options{}),
		Chart:         chartService,
		LiveCandles:   liveService,
		Favorites:     favoriteService,
		Alerts:        alerts.New(store, alertMonitor),

		ScannerIndicators: scannerIndicators,
		IndicatorTypes:    indicatorRegistry,
		Users:             users.New(store, cfg.AdminTelegramID, monitoredChanged.notify),
		Strategies:        strategies,
	}, httpapi.Options{APIDocsEnabled: cfg.APIDocsEnabled})
	return app{handler: handler, services: []service{
		{"market scheduler", scheduler},
		{"live kline stream", klineStream},
		{"live candle service", liveService},
		{"live trade stream", tradeStream},
		{"alert monitor", alertMonitor},
		{"closed indicator tracker", closedIndicators},
		{"strategy monitor", strategyMonitor},
		{"Telegram bot", botService},
		{"coin metadata synchronizer", coinMetadataSynchronizer},
		{"market retention", retention.New(store, logger, market.HistoryDepth)},
	}}, nil
}

// strategySource keeps the values enabled strategies read current on every
// monitored instrument.
type strategySource struct {
	instruments closedindicator.InstrumentSource
	strategies  *strategy.Service
}

func (source strategySource) Subscriptions(ctx context.Context, _ []closedindicator.Target) ([]closedindicator.Subscription, error) {
	return source.instruments.Subscriptions(ctx, source.strategies.Targets())
}

// tableTargets returns the distinct closed indicator targets the tables read.
func tableTargets(tables []markettable.Catalog) []closedindicator.Target {
	var targets []closedindicator.Target
	for _, table := range tables {
		for _, target := range table.ClosedTargets() {
			if !slices.ContainsFunc(targets, target.Equal) {
				targets = append(targets, target)
			}
		}
	}
	return targets
}
