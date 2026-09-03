package postgres_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"crypto-scanner/internal/auth"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/migrate"
	"crypto-scanner/internal/storage/postgres"
)

func TestPostgresStoreContracts(t *testing.T) {
	databaseURL := os.Getenv("CRYPTO_SCANNER_TEST_DATABASE_URL")
	if databaseURL == "" || os.Getenv("CRYPTO_SCANNER_TEST_DATABASE_RESET_OK") != "1" {
		t.Skip("set CRYPTO_SCANNER_TEST_DATABASE_URL to a disposable empty database and CRYPTO_SCANNER_TEST_DATABASE_RESET_OK=1")
	}
	ctx := context.Background()
	db, err := postgres.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open disposable PostgreSQL: %v", err)
	}
	t.Cleanup(db.Close)
	lockDisposablePostgres(t, db)
	var appSchema, marketSchema bool
	if err := db.QueryRow(ctx, `SELECT to_regnamespace('app') IS NOT NULL, to_regnamespace('binance_spot') IS NOT NULL`).Scan(&appSchema, &marketSchema); err != nil {
		t.Fatalf("inspect disposable database: %v", err)
	}
	if appSchema || marketSchema {
		t.Fatalf("integration database is not empty: app=%t binance_spot=%t", appSchema, marketSchema)
	}
	loadURL := func() (string, error) { return databaseURL, nil }
	unmigratedStore := postgres.NewStore(db)
	if !unmigratedStore.DatabaseReady(ctx) || unmigratedStore.MigrationsReady(ctx) || unmigratedStore.SuccessfulMarketSyncExists(ctx) {
		t.Fatal("unmigrated database readiness checks did not distinguish connectivity from schema and synchronization")
	}
	if err := migrate.Run(ctx, []string{"up"}, loadURL); err != nil {
		t.Fatalf("migrate disposable PostgreSQL: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(context.Background(), `DROP SCHEMA IF EXISTS app CASCADE; DROP SCHEMA IF EXISTS binance_spot CASCADE; DROP TABLE IF EXISTS public.crypto_scanner_schema_versions`); err != nil {
			t.Errorf("reset disposable PostgreSQL: %v", err)
		}
	})
	store := postgres.NewStore(db)
	if !store.DatabaseReady(ctx) || !store.MigrationsReady(ctx) || store.SuccessfulMarketSyncExists(ctx) {
		t.Fatal("fresh migrated database should be reachable and current but have no successful sync")
	}

	t.Run("enabled user lookup", func(t *testing.T) {
		if _, err := db.Exec(ctx, `INSERT INTO app.users (telegram_id, username, display_name, is_enabled) VALUES (101, 'alice', 'Alice', true), (102, 'bob', 'Bob', false)`); err != nil {
			t.Fatalf("seed users: %v", err)
		}
		user, err := store.FindEnabledByTelegramID(ctx, 101)
		if err != nil {
			t.Fatalf("FindEnabledByTelegramID() error = %v", err)
		}
		if user.TelegramID != 101 || user.Username != "alice" || user.DisplayName != "Alice" || !user.Enabled {
			t.Fatalf("user = %#v", user)
		}
		if _, err := store.FindEnabledByTelegramID(ctx, 102); !errors.Is(err, auth.ErrUserNotFound) {
			t.Fatalf("disabled user error = %v, want ErrUserNotFound", err)
		}
		if _, err := store.FindEnabledByTelegramID(ctx, 999); !errors.Is(err, auth.ErrUserNotFound) {
			t.Fatalf("unknown user error = %v, want ErrUserNotFound", err)
		}
	})

	t.Run("instrument snapshots deactivate and reactivate atomically", func(t *testing.T) {
		btc := market.Instrument{Symbol: "BTCUSDT", BaseAsset: "BTC", QuoteAsset: "USDT", Status: "TRADING", Active: true}
		eth := market.Instrument{Symbol: "ETHUSDT", BaseAsset: "ETH", QuoteAsset: "USDT", Status: "TRADING", Active: true}
		if err := store.ApplyInstrumentSnapshot(ctx, []market.Instrument{btc, eth}); err != nil {
			t.Fatalf("apply initial snapshot: %v", err)
		}
		if err := store.ApplyInstrumentSnapshot(ctx, []market.Instrument{btc, {Symbol: "", BaseAsset: "BAD", QuoteAsset: "USDT", Status: "TRADING", Active: true}}); err == nil {
			t.Fatal("invalid snapshot unexpectedly succeeded")
		}
		active, err := store.ListActiveInstruments(ctx)
		if err != nil || len(active) != 2 {
			t.Fatalf("failed snapshot changed the previous active set: instruments = %#v, error = %v", active, err)
		}
		originalIDs := map[string]int64{active[0].Symbol: active[0].ID, active[1].Symbol: active[1].ID}
		btc.Status = "BREAK"
		btc.Active = false
		if err := store.ApplyInstrumentSnapshot(ctx, []market.Instrument{btc, eth}); err != nil {
			t.Fatalf("apply status change snapshot: %v", err)
		}
		active, err = store.ListActiveInstruments(ctx)
		if err != nil || len(active) != 1 || active[0].Symbol != "ETHUSDT" {
			t.Fatalf("active instruments after status change = %#v, error = %v", active, err)
		}
		var btcStatus string
		var btcActive bool
		if err := db.QueryRow(ctx, `SELECT exchange_status, is_active FROM binance_spot.instruments WHERE symbol = 'BTCUSDT'`).Scan(&btcStatus, &btcActive); err != nil {
			t.Fatalf("inspect inactive status change: %v", err)
		}
		if btcStatus != "BREAK" || btcActive {
			t.Fatalf("persisted BTC status = %q, active = %t", btcStatus, btcActive)
		}
		if err := store.ApplyInstrumentSnapshot(ctx, []market.Instrument{eth}); err != nil {
			t.Fatalf("apply reduced snapshot: %v", err)
		}
		active, err = store.ListActiveInstruments(ctx)
		if err != nil {
			t.Fatalf("list reduced snapshot: %v", err)
		}
		if len(active) != 1 || active[0].Symbol != "ETHUSDT" {
			t.Fatalf("active instruments = %#v, want only ETHUSDT", active)
		}
		btc.Status = "TRADING"
		btc.Active = true
		if err := store.ApplyInstrumentSnapshot(ctx, []market.Instrument{btc, eth}); err != nil {
			t.Fatalf("reactivate instrument: %v", err)
		}
		active, err = store.ListActiveInstruments(ctx)
		if err != nil || len(active) != 2 || active[0].ID != originalIDs[active[0].Symbol] || active[1].ID != originalIDs[active[1].Symbol] {
			t.Fatalf("reactivated instruments = %#v, error = %v", active, err)
		}
	})

	t.Run("price history is bounded by instruments, hourly interval and closed window", func(t *testing.T) {
		instruments, err := store.ListActiveInstruments(ctx)
		if err != nil {
			t.Fatal(err)
		}
		start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
		end := start.Add(168 * time.Hour)
		t.Cleanup(func() {
			if _, err := db.Exec(ctx, `DELETE FROM binance_spot.candles WHERE open_time BETWEEN $1 AND $2`, start.Add(-time.Hour), end.Add(time.Hour)); err != nil {
				t.Errorf("clean price history fixture: %v", err)
			}
		})
		for _, instrument := range instruments {
			for _, hour := range []int{-1, 0, 168, 169} {
				open := start.Add(time.Duration(hour) * time.Hour)
				candle := market.Candle{InstrumentID: instrument.ID, Interval: "1h", OpenTime: open, CloseTime: open.Add(time.Hour - time.Millisecond), Open: 10, High: 12, Low: 9, Close: 10.12345678}
				if err := store.UpsertCandles(ctx, []market.Candle{candle, candle}); err != nil {
					t.Fatal(err)
				}
				candle.Interval = "1d"
				candle.CloseTime = open.Add(24*time.Hour - time.Millisecond)
				if err := store.UpsertCandles(ctx, []market.Candle{candle}); err != nil {
					t.Fatal(err)
				}
			}
		}
		prices, err := store.ListHourlyPrices(ctx, []int64{instruments[0].ID}, start, end)
		if err != nil {
			t.Fatal(err)
		}
		if len(prices) != 2 || !prices[0].OpenTime.Equal(start) || !prices[1].OpenTime.Equal(end) || prices[0].Close != 10.12345678 || prices[1].InstrumentID != instruments[0].ID {
			t.Fatalf("wrong bounded prices: %+v", prices)
		}
		empty, err := store.ListHourlyPrices(ctx, nil, start, end)
		if err != nil || len(empty) != 0 {
			t.Fatalf("empty instrument selection: %+v / %v", empty, err)
		}
	})

	t.Run("candle upsert is idempotent and numerics are checked", func(t *testing.T) {
		instruments, err := store.ListActiveInstruments(ctx)
		if err != nil {
			t.Fatalf("list active instruments: %v", err)
		}
		instrumentID := instruments[0].ID
		openTime := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
		candle := market.Candle{InstrumentID: instrumentID, Interval: "1d", OpenTime: openTime, CloseTime: openTime.Add(24*time.Hour - time.Millisecond), Open: 100.125, High: 110.5, Low: 90.25, Close: 105.75, Volume: 1234.5, QuoteAssetVolume: 130000.25, TradeCount: 42}
		if err := store.UpsertCandles(ctx, []market.Candle{candle}); err != nil {
			t.Fatalf("first candle upsert: %v", err)
		}
		candle.Close = 106.25
		if err := store.UpsertCandles(ctx, []market.Candle{candle}); err != nil {
			t.Fatalf("second candle upsert: %v", err)
		}
		candles, err := store.ListLatestCandlesByInterval(ctx, instrumentID, "1d", 30)
		if err != nil {
			t.Fatalf("ListLatestCandles() error = %v", err)
		}
		if len(candles) != 1 || candles[0].Close != 106.25 || candles[0].Open != 100.125 {
			t.Fatalf("candles = %#v, want one updated precision-preserving value", candles)
		}

		if _, err := db.Exec(ctx, `UPDATE binance_spot.candles SET high = 1e10000 WHERE instrument_id = $1`, instrumentID); err != nil {
			t.Fatalf("seed out-of-range numeric: %v", err)
		}
		if _, err := store.ListLatestCandlesByInterval(ctx, instrumentID, "1d", 30); err == nil {
			t.Fatal("ListLatestCandles() accepted a NUMERIC outside float64 range")
		}
	})

	t.Run("sync state round trips independently of the latest outcome", func(t *testing.T) {
		started := time.Date(2026, time.August, 4, 1, 2, 3, 0, time.UTC)
		succeeded := started.Add(time.Minute)
		closed := time.Date(2026, time.August, 3, 0, 0, 0, 0, time.UTC)
		profile := market.SyncProfile{Exchange: "binance", Market: "spot", QuoteAsset: "USDT", Interval: "1d", TimeZone: "UTC"}
		neverRun, err := store.GetSyncState(ctx, profile)
		if err != nil || neverRun.Status != market.SyncStatusNeverRun {
			t.Fatalf("initial sync state = %#v, error = %v", neverRun, err)
		}
		state := market.SyncState{Profile: profile, LastStartedAt: &started, LastSucceededAt: &succeeded, LastClosedOpenTime: &closed, Status: market.SyncStatusSucceeded}
		if err := store.SaveSyncState(ctx, state); err != nil {
			t.Fatalf("SaveSyncState() error = %v", err)
		}
		got, err := store.GetSyncState(ctx, profile)
		if err != nil {
			t.Fatalf("GetSyncState() error = %v", err)
		}
		if got.Status != market.SyncStatusSucceeded || got.LastSucceededAt == nil || !got.LastSucceededAt.Equal(succeeded) {
			t.Fatalf("sync state = %#v", got)
		}
		// Readiness requires both independently synchronized datasets. A daily
		// success alone must not make the market ready.
		if _, err := db.Exec(ctx, `DELETE FROM binance_spot.sync_state WHERE profile_key = $1`, market.SyncProfile{Exchange: "binance", Market: "spot", QuoteAsset: "USDT", Interval: "1h", TimeZone: "UTC"}.Key()); err != nil {
			t.Fatalf("clear hourly state: %v", err)
		}
		if store.SuccessfulMarketSyncExists(ctx) {
			t.Fatal("SuccessfulMarketSyncExists() = true after only a daily sync")
		}
		hourly := market.SyncProfile{Exchange: "binance", Market: "spot", QuoteAsset: "USDT", Interval: "1h", TimeZone: "UTC"}
		if err := store.SaveSyncState(ctx, market.SyncState{Profile: hourly, LastSucceededAt: &succeeded, Status: market.SyncStatusSucceeded}); err != nil {
			t.Fatalf("save hourly success: %v", err)
		}
		if !store.SuccessfulMarketSyncExists(ctx) {
			t.Fatal("SuccessfulMarketSyncExists() = false after both profile syncs succeeded")
		}
		failureMessage := "temporary exchange failure"
		state.Status = market.SyncStatusFailed
		state.ErrorMessage = failureMessage
		state.LastStartedAt = timePointer(started.Add(time.Hour))
		state.LastSucceededAt = nil
		state.LastClosedOpenTime = nil
		if err := store.SaveSyncState(ctx, state); err != nil {
			t.Fatalf("save later failed sync: %v", err)
		}
		if !store.SuccessfulMarketSyncExists(ctx) {
			t.Fatal("a later failure hid the previously successful dataset")
		}
		got, err = store.GetSyncState(ctx, profile)
		if err != nil || got.LastSucceededAt == nil || !got.LastSucceededAt.Equal(succeeded) || got.LastClosedOpenTime == nil || !got.LastClosedOpenTime.Equal(closed) {
			t.Fatalf("later failure discarded successful progress: state = %#v, error = %v", got, err)
		}
	})
}

func timePointer(value time.Time) *time.Time { return &value }
