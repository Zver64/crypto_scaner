package strategy

import "testing"

func TestAlertTextUsesMessage(t *testing.T) {
	entry := Entry{Strategy: Strategy{Name: "Breakout", Expression: "h_close > 1", Message: "Buy the breakout\nStop below 1"}}
	got := alertText(entry, Instrument{ID: 1, Symbol: "BTCUSDT"}, snapshot{})
	if want := "🎯 Breakout: BTCUSDT\nBuy the breakout\nStop below 1"; got != want {
		t.Fatalf("alertText() = %q, want %q", got, want)
	}
}
