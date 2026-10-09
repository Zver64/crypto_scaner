package main

import (
	"errors"
	"math"
	"strconv"
	"strings"

	"crypto-scanner/internal/apiclient"

	"github.com/spf13/pflag"
)

// marketCapFlags are the --min-market-cap and --max-market-cap flags that
// strategies and signals share.
type marketCapFlags struct{ minimum, maximum string }

// addCreate registers the flags of a create command, whose strategy or
// signal does what verb says, such as buy, only within the range.
func (f *marketCapFlags) addCreate(flags *pflag.FlagSet, verb string) {
	flags.StringVar(&f.minimum, "min-market-cap", "", verb+" only coins whose market cap is at least `USD`, such as 50M; backtests ignore it")
	flags.StringVar(&f.maximum, "max-market-cap", "", verb+" only coins whose market cap is at most `USD`, such as 2B; backtests ignore it")
}

// addUpdate registers the flags of an update command.
func (f *marketCapFlags) addUpdate(flags *pflag.FlagSet) {
	flags.StringVar(&f.minimum, "min-market-cap", "", "the minimum market cap in `USD`, such as 50M; empty removes it, omitted preserves it")
	flags.StringVar(&f.maximum, "max-market-cap", "", "the maximum market cap in `USD`, such as 2B; empty removes it, omitted preserves it")
}

// parse reads both bounds, nil when empty.
func (f marketCapFlags) parse() (minimum, maximum *float64, err error) {
	if minimum, err = marketCapFlag("min-market-cap", f.minimum); err != nil {
		return nil, nil, err
	}
	if maximum, err = marketCapFlag("max-market-cap", f.maximum); err != nil {
		return nil, nil, err
	}
	return minimum, maximum, nil
}

// update replaces the bounds of body whose flags were given with minimum and
// maximum, as parse read them, preserving the others.
func (f marketCapFlags) update(flags *pflag.FlagSet, body *apiclient.StrategyUpdate, minimum, maximum *float64) {
	if flags.Changed("min-market-cap") {
		body.MinMarketCapUsd = minimum
	}
	if flags.Changed("max-market-cap") {
		body.MaxMarketCapUsd = maximum
	}
}

// marketCapFlag parses the market cap bound of flag, nil when empty.
func marketCapFlag(flag, value string) (*float64, error) {
	usd, err := parseUSD(value)
	if err != nil {
		return nil, usageError("--%s: %v", flag, err)
	}
	return usd, nil
}

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
