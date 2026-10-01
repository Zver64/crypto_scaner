package chart_test

import (
	"context"
	"log/slog"
	"slices"
	"testing"
	"time"

	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/indicator"
	indicatortalib "crypto-scanner/internal/indicator/talib"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/market/kline"
	marketlive "crypto-scanner/internal/market/live"
)

// historyStub serves the newest candles of its history like storage does.
type historyStub struct {
	candles      []market.Candle
	resolveCalls int
	limits       []int
}

func (store *historyStub) GetActiveInstrumentBySymbol(_ context.Context, symbol string) (market.Instrument, error) {
	store.resolveCalls++
	return market.Instrument{ID: 1, Symbol: symbol, Active: true}, nil
}

func (store *historyStub) ListCandlePage(_ context.Context, _ int64, _ market.CandleInterval, _ *time.Time, limit int) (market.CandlePage, error) {
	store.limits = append(store.limits, limit)
	split := max(0, len(store.candles)-limit)
	return market.CandlePage{Candles: slices.Clone(store.candles[split:]), HasMore: split > 0}, nil
}

func TestLiveSessionReloadsOnlyTheTailOfContiguousHistory(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	store := &historyStub{candles: testCandles(start, 300)}
	service := newLiveTestService(t, store)
	session := service.NewLiveSession("BTCUSDT", market.IntervalHour)
	key := kline.Key{Symbol: "BTCUSDT", Interval: market.IntervalHour}
	selections := []indicator.Selection{{Type: indicatortalib.RSIType, Parameters: indicator.Parameters{"period": 14}}}

	trigger, _ := session.Apply(marketlive.Message{Kind: marketlive.KindSnapshot, Key: key})
	if _, ok, err := session.Next(context.Background(), trigger, chart.DefaultRange, selections); err != nil || !ok {
		t.Fatalf("first Next() ok=%v error=%v", ok, err)
	}
	// A correction and a new close reach storage, then the stream reports it.
	store.candles[len(store.candles)-1].Close += 5
	store.candles = append(store.candles, testCandles(start.Add(300*time.Hour), 1)...)
	closed := store.candles[len(store.candles)-1]
	trigger, _ = session.Apply(marketlive.Message{Kind: marketlive.KindUpdate, Key: key, Candle: &marketlive.CandleState{Candle: closed, Final: true}})
	frame, ok, err := session.Next(context.Background(), trigger, chart.DefaultRange, selections)
	if err != nil || !ok || !frame.Snapshot {
		t.Fatalf("stream Next() ok=%v snapshot=%v error=%v", ok, frame.Snapshot, err)
	}
	if store.resolveCalls != 1 || !slices.Equal(store.limits, []int{214, 16}) {
		t.Fatalf("store resolve calls=%d limits=%v, want one lookup, a full load, and a tail reload", store.resolveCalls, store.limits)
	}

	want, err := service.Build(context.Background(), chart.Request{Symbol: "BTCUSDT", Interval: market.IntervalHour, Limit: chart.DefaultRange, Indicators: selections})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(frame.Page.Candles, want.Candles) || !frame.Page.HasMore {
		t.Fatalf("tail reload candles differ from a full build")
	}
	got, expected := frame.Page.Indicators[0].Series[0].Points, want.Indicators[0].Series[0].Points
	if !slices.Equal(got, expected) {
		t.Fatalf("tail reload RSI differs from a full build: %d points vs %d", len(got), len(expected))
	}
}

func TestLiveSessionReloadsEverythingWhenTheTailIsNotEnough(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	gapped := testCandles(start, 300)
	tests := []struct {
		name    string
		history []market.Candle
		trigger func(*chart.LiveSession, []market.Candle) chart.Trigger
		limit   int
	}{
		// Sync commits and recovery may correct any stored candle.
		{name: "stream snapshot", history: testCandles(start, 300), trigger: applySnapshot, limit: 200},
		// A repaired gap is invisible in the newest candles.
		{name: "gap in range", history: append(gapped[:250:250], gapped[251:]...), trigger: applyClose, limit: 200},
		// Older candles may have been backfilled since the first load.
		{name: "short range", history: testCandles(start, 30), trigger: applyClose, limit: 200},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &historyStub{candles: test.history}
			session := newLiveTestService(t, store).NewLiveSession("BTCUSDT", market.IntervalHour)
			if _, _, err := session.Next(context.Background(), applySnapshot(session, nil), chart.DefaultRange, nil); err != nil {
				t.Fatal(err)
			}
			if _, _, err := session.Next(context.Background(), test.trigger(session, store.candles), chart.DefaultRange, nil); err != nil {
				t.Fatal(err)
			}
			if store.resolveCalls != 1 || !slices.Equal(store.limits, []int{test.limit, test.limit}) {
				t.Fatalf("store resolve calls=%d limits=%v, want two full loads after one lookup", store.resolveCalls, store.limits)
			}
		})
	}
}

func applySnapshot(session *chart.LiveSession, _ []market.Candle) chart.Trigger {
	trigger, _ := session.Apply(marketlive.Message{Kind: marketlive.KindSnapshot, Key: kline.Key{Symbol: "BTCUSDT", Interval: market.IntervalHour}})
	return trigger
}

func applyClose(session *chart.LiveSession, history []market.Candle) chart.Trigger {
	closed := history[len(history)-1]
	trigger, _ := session.Apply(marketlive.Message{Kind: marketlive.KindUpdate, Key: kline.Key{Symbol: "BTCUSDT", Interval: market.IntervalHour}, Candle: &marketlive.CandleState{Candle: closed, Final: true}})
	return trigger
}

func newLiveTestService(t *testing.T, store chart.Store) *chart.Service {
	t.Helper()
	registry, err := indicator.NewRegistry(indicatortalib.New()...)
	if err != nil {
		t.Fatal(err)
	}
	service, err := chart.NewService(store, registry, emptyCatalog{}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	return service
}
