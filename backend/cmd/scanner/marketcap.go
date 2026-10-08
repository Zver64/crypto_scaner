package main

import (
	"errors"
	"math"
	"strconv"
	"strings"

	"crypto-scanner/internal/apiclient"
)

// usdScales are the suffixes of compact USD amounts, largest first.
var usdScales = []struct {
	suffix string
	size   float64
}{{"T", 1e12}, {"B", 1e9}, {"M", 1e6}, {"K", 1e3}}

// parseUSD reads a positive amount such as 150M, 1.5B, or 2000000; empty
// is nil, an open bound.
func parseUSD(text string) (*float64, error) {
	text = strings.TrimPrefix(strings.TrimSpace(text), "$")
	if text == "" {
		return nil, nil
	}
	size := 1.0
	for _, scale := range usdScales {
		if number, ok := strings.CutSuffix(strings.ToUpper(text), scale.suffix); ok {
			text, size = number, scale.size
			break
		}
	}
	value, err := strconv.ParseFloat(text, 64)
	value *= size
	if err != nil || !(value > 0 && value <= math.MaxFloat64) {
		return nil, errors.New("want a positive USD amount such as 150M, 1.5B, or 2000000")
	}
	return &value, nil
}

// formatUSD writes an amount compactly, such as $150M or $1.5B.
func formatUSD(usd float64) string {
	for _, scale := range usdScales {
		// Rounding first lets 999,995,000 read $1B rather than $1000M.
		if rounded := math.Round(usd/scale.size*100) / 100; rounded >= 1 {
			return "$" + strconv.FormatFloat(rounded, 'f', -1, 64) + scale.suffix
		}
	}
	return "$" + strconv.FormatFloat(usd, 'f', -1, 64)
}

// marketCapRange writes the market cap range of a strategy, "-" without
// bounds.
func marketCapRange(strategy apiclient.Strategy) string {
	switch minimum, maximum := strategy.MinMarketCapUsd, strategy.MaxMarketCapUsd; {
	case minimum != nil && maximum != nil:
		return formatUSD(*minimum) + " – " + formatUSD(*maximum)
	case minimum != nil:
		return "≥ " + formatUSD(*minimum)
	case maximum != nil:
		return "≤ " + formatUSD(*maximum)
	}
	return "-"
}
