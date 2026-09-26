package main

import (
	"fmt"
	"log/slog"
	"net/http"
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
	indicatortalib "crypto-scanner/internal/indicator/talib"
	"crypto-scanner/internal/market"
	marketlive "crypto-scanner/internal/market/live"
	"crypto-scanner/internal/market/marketsync"
	"crypto-scanner/internal/marketcap"
	"crypto-scanner/internal/platform/config"
	"crypto-scanner/internal/storage/postgres"
	"crypto-scanner/internal/telegrambot"
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

func buildApp(cfg config.ServerConfig, logger *slog.Logger, store *postgres.Store) (app, error) {
	indicatorRegistry, err := indicator.NewRegistry(indicatortalib.NewRSI())
	if err != nil {
		return app{}, fmt.Errorf("initialize indicator registry: %w", err)
	}
	// Values shown in tables; favorites keep them current without clients.
	closedTargets := []closedindicator.Target{{
		Interval:  market.IntervalDay,
		Selection: indicator.Selection{Type: indicatortalib.RSIType, Parameters: indicator.Parameters{"period": indicatortalib.DefaultRSIPeriod}},
	}}
	closedIndicators, err := closedindicator.New(store, indicatorRegistry, closedTargets, logger,
		closedindicator.InstrumentSource(store.ListMonitoredInstrumentIDs))
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
		synchronizers[interval] = marketsync.New(exchange, syncStore, logger, cfg.SyncWorkers, market.BinanceSpotSyncProfile(interval))
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
	chartService, err := chart.NewService(store, indicatorRegistry, logger)
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
	monitoredChanged = append(monitoredChanged, alertMonitor.Changed, closedIndicators.Refresh)
	favoriteService := favorites.New(store, monitoredChanged.notify, analysisService, closedIndicators)

	handler := httpapi.New(logger, httpapi.Dependencies{
		Readiness:     store,
		Analysis:      analysisService,
		History:       store,
		Authenticator: authtelegram.New(store, cfg.TelegramBotToken, cfg.TelegramInitDataMaxAge, authtelegram.Options{}),
		Chart:         chartService,
		LiveCandles:   liveService,
		Favorites:     favoriteService,
		Alerts:        alerts.New(store, alertMonitor),
	}, httpapi.Options{APIDocsEnabled: cfg.APIDocsEnabled})
	return app{handler: handler, services: []service{
		{"market scheduler", scheduler},
		{"live kline stream", klineStream},
		{"live candle service", liveService},
		{"live trade stream", tradeStream},
		{"alert monitor", alertMonitor},
		{"closed indicator tracker", closedIndicators},
		{"Telegram bot", botService},
		{"coin metadata synchronizer", coinMetadataSynchronizer},
	}}, nil
}
