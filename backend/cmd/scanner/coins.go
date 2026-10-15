package main

import (
	"slices"
	"strings"

	"crypto-scanner/internal/apiclient"

	"github.com/spf13/cobra"
)

// coinFlags choose the coins a strategy or signal evaluates: --coin,
// --favorites, and --coin-m add coins, and a signal's --track-favorites
// evaluates the administrator's favorites, whatever they are at the time,
// and its --all-coins every active coin instead.
type coinFlags struct {
	coins                                      []string
	favorites, coinM, trackFavorites, allCoins bool
}

// add registers the flags; only a signal may track the favorites or evaluate
// all coins.
func (f *coinFlags) add(command *cobra.Command, signal bool) {
	flags := command.Flags()
	flags.StringSliceVar(&f.coins, "coin", nil, "evaluate coin `SYM`; repeat or comma-separate")
	flags.BoolVar(&f.favorites, "favorites", false, "copy your active favorites into the chosen coins")
	flags.BoolVar(&f.coinM, "coin-m", false, "evaluate the coins with a Binance COIN-M perpetual")
	if signal {
		flags.BoolVar(&f.trackFavorites, "track-favorites", false, "evaluate the administrator's active favorites whenever they change")
		flags.BoolVar(&f.allCoins, "all-coins", false, "evaluate every active coin")
		command.MarkFlagsMutuallyExclusive("track-favorites", "all-coins")
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
	return len(f.symbols()) > 0 || f.favorites || f.coinM || f.trackFavorites || f.allCoins
}

// resolveCoins returns the coins f chooses, ascending, and their source.
func (c *cli) resolveCoins(command *cobra.Command, client *apiclient.ClientWithResponses, f coinFlags) ([]string, apiclient.StrategyCoinSource, error) {
	coins := f.symbols()
	if f.trackFavorites || f.allCoins {
		if len(coins) > 0 || f.favorites || f.coinM {
			return nil, "", usageError("--track-favorites and --all-coins choose no coins; drop --coin, --favorites, and --coin-m")
		}
		if f.trackFavorites {
			return []string{}, apiclient.CoinSourceFavorites, nil
		}
		return []string{}, apiclient.CoinSourceAll, nil
	}
	if f.favorites {
		favorites, err := client.ListFavoritesWithResponse(command.Context())
		if err == nil {
			err = c.check(favorites, favorites.JSON200 != nil)
		}
		if err != nil {
			return nil, "", err
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
			return nil, "", err
		}
		for _, instrument := range instruments.JSON200.Items {
			coins = append(coins, instrument.Symbol)
		}
	}
	if len(coins) == 0 {
		return nil, "", usageError("no coins chosen; use --coin, --favorites, or --coin-m, or a signal's --track-favorites or --all-coins")
	}
	slices.Sort(coins)
	return slices.Compact(coins), apiclient.CoinSourceSelected, nil
}
