package strategy

import (
	"context"
	"log/slog"
	"testing"

	"crypto-scanner/internal/market"
	"crypto-scanner/internal/scannerindicator"

	"github.com/google/uuid"
)

func legacyTestService(t *testing.T) *Service {
	t.Helper()
	registry := testRegistry(t)
	indicators, err := scannerindicator.New(&struct{ scannerindicator.Store }{}, registry, []string{"yellow.5"}, slog.New(slog.DiscardHandler), func() {})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(newBacktestStore(market.IntervalHour, nil), indicators, registry, noTracker{}, slog.New(slog.DiscardHandler), 0, func([]uuid.UUID) {})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

// Formulas of docs/buy-strategies.md and the other functions of the old
// syntax become calls that compile, keeping their layout.
func TestRewriteLegacy(t *testing.T) {
	service := legacyTestService(t)
	for _, test := range []struct {
		old, new string
		kind     RuleKind
	}{
		{
			old: "d_low < prev(d_min_20_low)\n&& d_close > prev(d_min_20_low)",
			new: "low(\"1d\") < Prev(min(\"1d\", 20, low))\n&& close(\"1d\") > Prev(min(\"1d\", 20, low))",
		},
		{
			old: "d_high < prev(d_high)\n&& d_high - d_low < min(\n  prev(d_high - d_low, 1),\n  prev(d_high - d_low, 2))",
			new: "high(\"1d\") < Prev(high(\"1d\"))\n&& high(\"1d\") - low(\"1d\") < Min(\n  Prev(high(\"1d\") - low(\"1d\"), 1),\n  Prev(high(\"1d\") - low(\"1d\"), 2))",
		},
		{
			old: `of("BTCUSDT", h_roc_12) < 0 && h_roc_12 > 0 && crosses_above(h_close, prev(h_max_12_high, 1))`,
			new: `Of("BTCUSDT", roc("1h", 12)) < 0 && roc("1h", 12) > 0 && CrossesAbove(close("1h"), Prev(max("1h", 12, high), 1))`,
		},
		{
			old: `prev(h_adx) > 30 && h_ema_20 > prev(h_ema_20) && crosses_below(h_low, h_ema_20)`,
			new: `Prev(adx("1h")) > 30 && ema("1h", 20) > Prev(ema("1h", 20)) && CrossesBelow(low("1h"), ema("1h", 20))`,
		},
		{
			old:  `d_close < prev(d_min_20_low) || d_close < entry_price - 2 * d_atr_20`,
			new:  `close("1d") < Prev(min("1d", 20, low)) || close("1d") < entry_price - 2 * atr("1d", 20)`,
			kind: ExitRule,
		},
		{
			old:  `prev(h_close, h_pivot_low_bars - 1) > h_open || bars_held > 5`,
			new:  `Prev(close("1h"), pivot_low_bars("1h") - 1) > open("1h") || bars_held > 5`,
			kind: ExitRule,
		},
		{
			old: `w_rsi_21 > percentile(w_rsi_21, 50, 90) && m_close > m_bbands_20_2p5_upperband && h_quote_volume > 2 * h_sma_20_volume`,
			new: `rsi("1w", 21) > Percentile(rsi("1w", 21), 50, 90) && close("1M") > bbands_upperband("1M", 20, 2.5) && quote_volume("1h") > 2 * sma("1h", 20, volume)`,
		},
		{
			old:  `h_close - abs(mod(h_trades, 2)) * max(h_atr, 1)`,
			new:  `close("1h") - Abs(Mod(trades("1h"), 2)) * Max(atr("1h"), 1)`,
			kind: PriceRule,
		},
	} {
		got, err := service.rewriteLegacy(test.old)
		if err != nil || got != test.new {
			t.Errorf("rewriteLegacy(%q) = %q, %v, want %q", test.old, got, err, test.new)
			continue
		}
		if _, err := Compile(got, service.registry, test.kind); err != nil {
			t.Errorf("Compile(%q) error = %v", got, err)
		}
	}
	for _, old := range []string{`h_unknown > 1`, `h_close > foo(1)`, `h_close >`} {
		if got, err := service.rewriteLegacy(old); err == nil {
			t.Errorf("rewriteLegacy(%q) = %q, want an error", old, got)
		}
	}
}

// A rule that cannot be rewritten stays as it is; the others are rewritten.
func TestRewriteLegacyStrategyKeepsWhatItCannotRewrite(t *testing.T) {
	service := legacyTestService(t)
	got := service.rewriteLegacyStrategy(context.Background(), Strategy{Expression: `h_close > 1`, ExitExpression: `h_unknown > 1`})
	if got.Expression != `close("1h") > 1` || got.ExitExpression != `h_unknown > 1` || got.TakeProfitExpression != "" {
		t.Fatalf("rewriteLegacyStrategy() = %+v", got)
	}
}
