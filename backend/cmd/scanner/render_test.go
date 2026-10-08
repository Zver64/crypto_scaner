package main

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"crypto-scanner/internal/apiclient"

	"github.com/rivo/uniseg"
)

func TestWrapCellsKeepsJoinedEmojiTogether(t *testing.T) {
	if got := wrapCells("A👨‍👩‍👧‍👦B", 4); got != "A👨‍👩‍👧‍👦B" {
		t.Fatalf("wrap = %q", got)
	}
	if got := wrapCells("A❤️BC", 4); got != "A❤️B\nC" {
		t.Fatalf("wrap = %q", got)
	}
	var output bytes.Buffer
	renderStrategiesWidth(&output, strategiesKind, []apiclient.Strategy{{Id: 1, Name: "A👨‍👩‍👧‍👦B❤️", Expression: "true", Valid: true}}, 80)
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	width := uniseg.StringWidth(lines[0])
	for _, line := range lines {
		if got := uniseg.StringWidth(line); got != width {
			t.Fatalf("table width = %d, want %d: %q", got, width, line)
		}
	}
}

func TestStrategiesFitTerminalWidth(t *testing.T) {
	strategies := []apiclient.Strategy{{
		Id: math.MaxInt64, Name: "A策略 Длинное название 策略 👨‍👩‍👧‍👦 ❤️", Enabled: true,
		Expression:     "h_close > 123456789012345678901234567890 && d_rsi < 30",
		ExitExpression: "pnl > 5", Message: strings.Repeat("alert ", 20),
	}}
	for _, width := range []int{20, 29, 35, 36, 40, 80, 120} {
		var output bytes.Buffer
		renderStrategiesWidth(&output, strategiesKind, strategies, width)
		for line := range strings.Lines(output.String()) {
			if got := uniseg.StringWidth(strings.TrimSuffix(line, "\n")); got > width {
				t.Fatalf("width %d: line is %d cells: %q", width, got, line)
			}
		}
		if output.Len() == 0 {
			t.Fatal("no strategies rendered")
		}
	}
}
