package main

import (
	"slices"
	"strings"

	"crypto-scanner/internal/apiclient"

	"github.com/spf13/cobra"
)

// coinFlags choose the coins a strategy or signal evaluates: --coin,
// --favorites, and --coin-m add coins, and a signal's --all-coins evaluates
// every active coin instead.
type coinFlags struct {
	coins                      []string
	favorites, coinM, allCoins bool
}

// add registers the flags; only a signal may evaluate all coins.
func (f *coinFlags) add(command *cobra.Command, signal bool) {
	flags := command.Flags()
	flags.StringSliceVar(&f.coins, "coin", nil, "evaluate coin `SYM`; repeat or comma-separate")
	flags.BoolVar(&f.favorites, "favorites", false, "evaluate your active favorites")
	flags.BoolVar(&f.coinM, "coin-m", false, "evaluate the coins with a Binance COIN-M perpetual")
	if signal {
		flags.BoolVar(&f.allCoins, "all-coins", false, "evaluate every active coin")
	}
}

// symbols lists the --coin symbols, upper-cased, without empty ones.
func (f coinFlags) symbols() []string {
	symbols := make([]string, 0, len(f.coins))
	for _, coin := range f.coins {
		if coin = strings.ToUpper(strings.TrimSpace(coin)); coin != "" {
			symbols = append(symbols, coin)
		}
	}
	return symbols
}

// changed reports whether the flags choose coins; a flag set to false or an
// empty --coin chooses none.
func (f coinFlags) changed() bool {
	return len(f.symbols()) > 0 || f.favorites || f.coinM || f.allCoins
}

// resolveCoins returns the coins f chooses, ascending, or reports all coins.
func (c *cli) resolveCoins(command *cobra.Command, client *apiclient.ClientWithResponses, f coinFlags) ([]string, bool, error) {
	coins := f.symbols()
	if f.allCoins {
		if len(coins) > 0 || f.favorites || f.coinM {
			return nil, false, usageError("--all-coins evaluates every coin; drop --coin, --favorites, and --coin-m")
		}
		return []string{}, true, nil
	}
	if f.favorites {
		favorites, err := client.ListFavoritesWithResponse(command.Context())
		if err == nil {
			err = c.check(favorites, favorites.JSON200 != nil)
		}
		if err != nil {
			return nil, false, err
		}
		for _, favorite := range favorites.JSON200.Items {
			if favorite.Active {
				coins = append(coins, favorite.Symbol)
			}
		}
	}
	if f.coinM {
		universe := apiclient.CoinmPerpetual
		instruments, err := client.ListInstrumentsWithResponse(command.Context(), &apiclient.ListInstrumentsParams{Universe: &universe})
		if err == nil {
			err = c.check(instruments, instruments.JSON200 != nil, instruments.JSON503)
		}
		if err != nil {
			return nil, false, err
		}
		for _, instrument := range instruments.JSON200.Items {
			coins = append(coins, instrument.Symbol)
		}
	}
	if len(coins) == 0 {
		return nil, false, usageError("no coins chosen; use --coin, --favorites, or --coin-m, or a signal's --all-coins")
	}
	slices.Sort(coins)
	return slices.Compact(coins), false, nil
}
