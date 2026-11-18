package main

import (
	"io"
	"slices"
	"strconv"
	"strings"

	"crypto-scanner/internal/apiclient"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// kind tells the strategies command group from the signals one: both are
// saved as strategies, a signal with the move it expects. Each group shows,
// edits, and deletes only its own.
type kind struct {
	signals bool
	// noun names one, plural the command group.
	noun, plural string
	// directions complete --direction: how a strategy trades or what a
	// signal expects. The server refuses the others.
	directions []string
}

var (
	strategiesKind = kind{noun: "strategy", plural: "strategies", directions: []string{string(apiclient.Long), string(apiclient.Short)}}
	signalsKind    = kind{signals: true, noun: "signal", plural: "signals", directions: []string{string(apiclient.Simple), string(apiclient.Long), string(apiclient.Short), string(apiclient.Sideways)}}
)

// holds reports whether strategy belongs to k.
func (k kind) holds(strategy apiclient.Strategy) bool { return strategy.Rules.Signal == k.signals }

// other is the group of the saved strategies k does not hold.
func (k kind) other() kind {
	if k.signals {
		return strategiesKind
	}
	return signalsKind
}

// saved returns the saved strategy id of k, refusing one of the other group.
func (c *cli) saved(command *cobra.Command, client *apiclient.ClientWithResponses, k kind, id uuid.UUID) (*apiclient.Strategy, error) {
	found, err := client.GetStrategyWithResponse(command.Context(), id)
	if err == nil {
		err = c.check(found, found.JSON200 != nil, found.JSON404)
	}
	if err != nil {
		return nil, err
	}
	if !k.holds(*found.JSON200) {
		other := k.other()
		return nil, failure("not_a_"+k.noun, "%s is a %s; use scanner %s", id, other.noun, other.plural)
	}
	return found.JSON200, nil
}

// editable returns the saved strategy id of k that is disabled: enabled ones
// alert, and only the Mini App edits them.
func (c *cli) editable(command *cobra.Command, client *apiclient.ClientWithResponses, k kind, id uuid.UUID) (*apiclient.Strategy, error) {
	current, err := c.saved(command, client, k, id)
	if err != nil {
		return nil, err
	}
	if current.Enabled {
		return nil, failure("strategy_enabled", "%s %s is enabled; edit it in the Mini App", k.noun, id)
	}
	return current, nil
}

// update sends body for the strategy id of k; the server checks it.
func (c *cli) update(command *cobra.Command, client *apiclient.ClientWithResponses, k kind, id uuid.UUID, body apiclient.StrategyUpdate) error {
	updated, err := client.UpdateStrategyWithResponse(command.Context(), id, body)
	if err == nil {
		err = c.check(updated, updated.JSON200 != nil, strategyInvalid(updated.JSON400), updated.JSON404, updated.JSON409)
	}
	if err != nil {
		return err
	}
	c.print(command, updated.Body, func(w io.Writer) { renderStrategies(w, k, []apiclient.Strategy{*updated.JSON200}) })
	return nil
}

// completeDirections completes --direction among the directions of k.
func (k kind) completeDirections() cobra.CompletionFunc {
	return cobra.FixedCompletions(k.directions, cobra.ShellCompDirectiveNoFileComp)
}

// signalFlags are the --target-ratio and --window flags of signals, which
// tell how backtests judge them.
type signalFlags struct{ ratio, window int }

var (
	signalTargetRatios = []apiclient.SignalTargetRatio{apiclient.SignalTargetRatioN2, apiclient.SignalTargetRatioN3, apiclient.SignalTargetRatioN4, apiclient.SignalTargetRatioN5}
	signalWindows      = []apiclient.SignalWindow{apiclient.SignalWindowN3, apiclient.SignalWindowN6, apiclient.SignalWindowN12, apiclient.SignalWindowN24}
)

// add registers the flags; a create command defaults them to 2 stops and 6
// candles, an update command preserves the current values when they are
// omitted.
func (f *signalFlags) add(command *cobra.Command, create bool) {
	flags := command.Flags()
	ratio, window, suffix := int(apiclient.SignalTargetRatioN2), int(apiclient.SignalWindowN6), ""
	if !create {
		ratio, window, suffix = 0, 0, "; omitted preserves it"
	}
	flags.IntVar(&f.ratio, "target-ratio", ratio, "the target distance in `STOPS` that backtests judge signals by: "+choices(signalTargetRatios)+suffix)
	flags.IntVar(&f.window, "window", window, "the `CANDLES` after a signal that backtests judge it over: "+choices(signalWindows)+suffix)
	_ = command.RegisterFlagCompletionFunc("target-ratio", cobra.FixedCompletions(texts(signalTargetRatios), cobra.ShellCompDirectiveNoFileComp))
	_ = command.RegisterFlagCompletionFunc("window", cobra.FixedCompletions(texts(signalWindows), cobra.ShellCompDirectiveNoFileComp))
}

// check refuses given or defaulted values outside the allowed ones.
func (f signalFlags) check(flags *pflag.FlagSet) error {
	if (f.ratio != 0 || flags.Changed("target-ratio")) && !apiclient.SignalTargetRatio(f.ratio).Valid() {
		return usageError("--target-ratio: %d is not %s", f.ratio, choices(signalTargetRatios))
	}
	if (f.window != 0 || flags.Changed("window")) && !apiclient.SignalWindow(f.window).Valid() {
		return usageError("--window: %d is not %s", f.window, choices(signalWindows))
	}
	return nil
}

// update replaces the settings of rules whose flags were given, preserving
// the others.
func (f signalFlags) update(flags *pflag.FlagSet, rules *apiclient.StrategyRules) {
	if flags.Changed("target-ratio") {
		rules.TargetRatio = new(apiclient.SignalTargetRatio(f.ratio))
	}
	if flags.Changed("window") {
		rules.Window = new(apiclient.SignalWindow(f.window))
	}
}

// choices lists values as "2, 3, 4, or 5".
func choices[T ~int](values []T) string {
	list := texts(values)
	return strings.Join(list[:len(list)-1], ", ") + ", or " + list[len(list)-1]
}

func texts[T ~int](values []T) []string {
	list := make([]string, len(values))
	for i, value := range values {
		list[i] = strconv.Itoa(int(value))
	}
	return list
}

// createSignalCommand saves a signal, always disabled like strategies.
func (c *cli) createSignalCommand() *cobra.Command {
	var rules signalRuleFlags
	var message string
	var coins coinFlags
	var marketCap marketCapFlags
	command := &cobra.Command{
		Use:   "create NAME --expr EXPR --direction simple|long|short|sideways (--track-favorites|--all-coins|(--coin SYM|--favorites|--coin-m)...) [--target-ratio STOPS] [--window CANDLES] [--min-market-cap USD] [--max-market-cap USD] [--message TEXT]",
		Short: "Save a disabled signal, which buys nothing and announces the move it expects",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			signal, err := rules.rules(command)
			if err != nil {
				return err
			}
			body := apiclient.StrategyInput{Name: args[0], Rules: signal, Message: message}
			if body.Coins.MinMarketCapUsd, body.Coins.MaxMarketCapUsd, err = marketCap.parse(); err != nil {
				return err
			}
			client, err := c.client()
			if err != nil {
				return err
			}
			if body.Coins.Symbols, body.Coins.Source, err = c.resolveCoins(command, client, coins); err != nil {
				return err
			}
			created, err := client.CreateStrategyWithResponse(command.Context(), body)
			if err == nil {
				err = c.check(created, created.JSON201 != nil, strategyInvalid(created.JSON400), created.JSON409)
			}
			if err != nil {
				return err
			}
			c.print(command, created.Body, func(w io.Writer) { renderStrategies(w, signalsKind, []apiclient.Strategy{*created.JSON201}) })
			return nil
		},
	}
	rules.add(command)
	coins.add(command, true)
	flags := command.Flags()
	marketCap.addCreate(flags)
	flags.StringVar(&message, "message", "", "`TEXT` sent in Telegram alerts under the title; empty sends none")
	return command
}

// signalRuleFlags are the rule, the move, and the backtest settings of a new
// signal, which signals create saves and signals backtest tries unsaved.
type signalRuleFlags struct {
	expression, direction string
	settings              signalFlags
}

func (f *signalRuleFlags) add(command *cobra.Command) {
	flags := command.Flags()
	flags.StringVar(&f.expression, "expr", "", "the entry rule `EXPR`ession, see docs/strategy-language.md")
	flags.StringVar(&f.direction, "direction", "", "the `MOVE` the signal expects: simple (none in particular, never judged), long, short, or sideways")
	f.settings.add(command, true)
	_ = command.MarkFlagRequired("expr")
	_ = command.MarkFlagRequired("direction")
	_ = command.RegisterFlagCompletionFunc("direction", signalsKind.completeDirections())
}

// rules checks the settings and returns the rules of the signal. A simple
// signal is never judged, so the defaults of --target-ratio and --window
// are not sent for it; given ones are, and the server refuses them.
func (f signalRuleFlags) rules(command *cobra.Command) (apiclient.StrategyRules, error) {
	flags := command.Flags()
	if err := f.settings.check(flags); err != nil {
		return apiclient.StrategyRules{}, err
	}
	rules := apiclient.StrategyRules{Signal: true, Direction: apiclient.Direction(f.direction), Expression: f.expression}
	if rules.Direction == apiclient.Simple {
		f.settings.update(flags, &rules)
		return rules, nil
	}
	rules.TargetRatio, rules.Window = new(apiclient.SignalTargetRatio(f.settings.ratio)), new(apiclient.SignalWindow(f.settings.window))
	return rules, nil
}

func (c *cli) updateSignalCommand() *cobra.Command {
	var name, expression, message string
	var coins coinFlags
	var marketCap marketCapFlags
	var settings signalFlags
	command := &cobra.Command{
		Use:               "update ID [--expr EXPR] [--track-favorites|--all-coins|(--coin SYM|--favorites|--coin-m)...] [--target-ratio STOPS] [--window CANDLES] [--min-market-cap USD] [--max-market-cap USD] [--name NAME] [--message TEXT]",
		Short:             "Edit a disabled saved signal, preserving unspecified fields; its direction never changes",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: c.completeArg(signalsKind),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := strategyID(args[0])
			if err != nil {
				return err
			}
			flags := command.Flags()
			// edits are the flags that change the signal.
			edits := []string{"expr", "target-ratio", "window", "min-market-cap", "max-market-cap", "name", "message"}
			edited := slices.ContainsFunc(edits, flags.Changed) || coins.changed()
			if !edited {
				return usageError("specify at least one of --expr, --coin, --favorites, --coin-m, --track-favorites, --all-coins, --target-ratio, --window, --min-market-cap, --max-market-cap, --name, or --message")
			}
			if err := settings.check(flags); err != nil {
				return err
			}
			minimum, maximum, err := marketCap.parse()
			if err != nil {
				return err
			}
			client, err := c.client()
			if err != nil {
				return err
			}
			current, err := c.editable(command, client, signalsKind, id)
			if err != nil {
				return err
			}
			body := apiclient.StrategyUpdate{Name: current.Name, Rules: current.Rules, Coins: current.Coins, Message: current.Message}
			// Chosen coins replace the current ones.
			if coins.changed() {
				if body.Coins.Symbols, body.Coins.Source, err = c.resolveCoins(command, client, coins); err != nil {
					return err
				}
			}
			settings.update(flags, &body.Rules)
			marketCap.update(flags, &body.Coins, minimum, maximum)
			for _, change := range []struct {
				flag   string
				target *string
				value  string
			}{{"name", &body.Name, name}, {"message", &body.Message, message}, {"expr", &body.Rules.Expression, expression}} {
				if flags.Changed(change.flag) {
					*change.target = change.value
				}
			}
			return c.update(command, client, signalsKind, id, body)
		},
	}
	command.Flags().StringVar(&expression, "expr", "", "the entry rule `EXPR`ession; omitted preserves the current one")
	coins.add(command, true)
	settings.add(command, false)
	marketCap.addUpdate(command.Flags())
	command.Flags().StringVar(&name, "name", "", "signal `NAME`; omitted preserves the current name")
	command.Flags().StringVar(&message, "message", "", "`TEXT` sent in Telegram alerts under the title; empty removes it, omitted preserves it")
	return command
}
