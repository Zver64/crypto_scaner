package strategy

import (
	"context"
	"testing"

	"crypto-scanner/internal/market"
)

func TestAlertTextUsesMessage(t *testing.T) {
	entry := Entry{Strategy: Strategy{Name: "Breakout", Expression: "h_close > 1", Message: "Buy the breakout\nStop below 1"}}
	got := alertText(entry, Instrument{ID: 1, Symbol: "BTCUSDT"}, TradeEvent{Kind: TradeBuy, Buy: 2, Close: 1.5}, snapshot{})
	if want := "🟢 Breakout: BTCUSDT buy #2 at 1.5\nBuy the breakout\nStop below 1"; got != want {
		t.Fatalf("alertText() = %q, want %q", got, want)
	}
}

// A halt disables only the revision that failed: a strategy changed since
// stays enabled.
func TestDisableAtRevisionKeepsAChangedStrategy(t *testing.T) {
	store := &disablingStore{backtestStore: newBacktestStore(market.IntervalHour, nil)}
	service := newBacktestService(t, store.backtestStore, Strategy{ID: 1, Name: "Hold", Direction: DirectionLong, Expression: "h_close > 5", Enabled: true, Revision: 3})
	service.store = store
	if disabled, err := service.DisableAtRevision(context.Background(), 1, 2); err != nil || disabled || store.calls != 0 || !service.List()[0].Enabled {
		t.Fatalf("stale revision: disabled %v, error %v, store calls %d", disabled, err, store.calls)
	}
	if disabled, err := service.DisableAtRevision(context.Background(), 1, 3); err != nil || !disabled || store.calls != 1 || service.List()[0].Enabled || service.List()[0].Revision != 4 {
		t.Fatalf("current revision: disabled %v, error %v, entry %+v", disabled, err, service.List()[0])
	}
}

type disablingStore struct {
	*backtestStore
	calls int
}

func (store *disablingStore) DisableStrategyAtRevision(_ context.Context, _, revision int64) (int64, bool, error) {
	store.calls++
	return revision + 1, true, nil
}
