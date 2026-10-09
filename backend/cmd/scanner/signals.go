package main

import (
	"encoding/json"
	"io"
	"slices"
	"strconv"
	"strings"

	"crypto-scanner/internal/apiclient"

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
	// directions are those of --direction: how a strategy trades or what a
	// signal expects.
	directions []string
}

var (
	strategiesKind = kind{noun: "strategy", plural: "strategies", directions: []string{string(apiclient.Long), string(apiclient.Short)}}
	signalsKind    = kind{signals: true, noun: "signal", plural: "signals", directions: []string{string(apiclient.Long), string(apiclient.Short), string(apiclient.Sideways)}}
)

// holds reports whether strategy belongs to k.
func (k kind) holds(strategy apiclient.Strategy) bool { return strategy.Signal == k.signals }

// other is the group of the saved strategies k does not hold.
func (k kind) other() kind {
	if k.signals {
		return strategiesKind
	}
	return signalsKind
}

// saved returns the saved strategy id of k, refusing one of the other group.
func (c *cli) saved(command *cobra.Command, client *apiclient.ClientWithResponses, k kind, id int64) (*apiclient.Strategy, error) {
	listed, err := client.ListStrategiesWithResponse(command.Context())
	if err == nil {
		err = c.check(listed, listed.JSON200 != nil)
	}
	if err != nil {
		return nil, err
	}
	index := slices.IndexFunc(listed.JSON200.Items, func(strategy apiclient.Strategy) bool { return strategy.Id == id })
	if index < 0 {
		return nil, failure("strategy_not_found", "no %s %d; see scanner %s", k.noun, id, k.plural)
	}
	current := listed.JSON200.Items[index]
	if !k.holds(current) {
		other := k.other()
		return nil, failure("not_a_"+k.noun, "%d is a %s; use scanner %s", id, other.noun, other.plural)
	}
	return &current, nil
}

// editable returns the saved strategy id of k that is disabled: enabled ones
// alert, and only the Mini App edits them.
func (c *cli) editable(command *cobra.Command, client *apiclient.ClientWithResponses, k kind, id int64) (*apiclient.Strategy, error) {
	current, err := c.saved(command, client, k, id)
	if err != nil {
		return nil, err
	}
	if current.Enabled {
		return nil, failure("strategy_enabled", "%s %d is enabled; edit it in the Mini App", k.noun, id)
	}
	return current, nil
}

// update validates rules, adding the indicators they miss with add, and sends
// body for current; with nothing edited and nothing to add it sends nothing
// and prints current as it is.
func (c *cli) update(command *cobra.Command, client *apiclient.ClientWithResponses, k kind, current apiclient.Strategy, body apiclient.StrategyUpdate, rules []strategyRule, add, edited bool) error {
	missing, err := c.prepareStrategyRules(command, client, rules, add)
	if err != nil {
		return err
	}
	if len(missing) == 0 && !edited {
		if !c.json {
			command.PrintErrln("no indicators to add")
		}
		raw, err := json.Marshal(current)
		if err != nil {
			return err
		}
		c.print(command, raw, func(w io.Writer) { renderStrategies(w, k, []apiclient.Strategy{current}) })
		return nil
	}
	updated, err := client.UpdateStrategyWithResponse(command.Context(), current.Id, body)
	if err == nil {
		err = c.check(updated, updated.JSON200 != nil, updated.JSON400, updated.JSON404, updated.JSON409)
	}
	if err != nil {
		return err
	}
	c.print(command, updated.Body, func(w io.Writer) { renderStrategies(w, k, []apiclient.Strategy{*updated.JSON200}) })
	return nil
}

// directionFlag parses --direction among the directions of k.
func (k kind) directionFlag(value string) (apiclient.Direction, error) {
	if slices.Contains(k.directions, value) {
		return apiclient.Direction(value), nil
	}
	last := len(k.directions) - 1
	list := strings.Join(k.directions[:last], ", ")
	if last > 1 {
		list += ","
	}
	return "", usageError("--direction: %q is not %s or %s", value, list, k.directions[last])
}

// completeDirections completes --direction among the directions of k.
func (k kind) completeDirections() cobra.CompletionFunc {
	return cobra.FixedCompletions(k.directions, cobra.ShellCompDirectiveNoFileComp)
}

// signalFlags are the --target-ratio and --window flags of signals, which
// tell how backtests judge them.
type signalFlags struct{ ratio, window int }

var (
	signalTargetRatios = []int{2, 3, 4, 5}
	signalWindows      = []int{3, 6, 12, 24}
)

// add registers the flags; a create command defaults them to 2 stops and 6
// candles, an update command preserves the current values when they are
// omitted.
func (f *signalFlags) add(command *cobra.Command, create bool) {
	flags := command.Flags()
	ratio, window, suffix := 2, 6, ""
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
	if (f.ratio != 0 || flags.Changed("target-ratio")) && !slices.Contains(signalTargetRatios, f.ratio) {
		return usageError("--target-ratio: %d is not %s", f.ratio, choices(signalTargetRatios))
	}
	if (f.window != 0 || flags.Changed("window")) && !slices.Contains(signalWindows, f.window) {
		return usageError("--window: %d is not %s", f.window, choices(signalWindows))
	}
	return nil
}

// update replaces the settings of body whose flags were given, preserving
// the others.
func (f signalFlags) update(flags *pflag.FlagSet, body *apiclient.StrategyUpdate) {
	if flags.Changed("target-ratio") {
		body.TargetRatio = new(apiclient.SignalTargetRatio(f.ratio))
	}
	if flags.Changed("window") {
		body.Window = new(apiclient.SignalWindow(f.window))
	}
}

// choices lists values as "2, 3, 4, or 5".
func choices(values []int) string {
	list := texts(values)
	return strings.Join(list[:len(list)-1], ", ") + ", or " + list[len(list)-1]
}

func texts(values []int) []string {
	list := make([]string, len(values))
	for i, value := range values {
		list[i] = strconv.Itoa(value)
	}
	return list
}

// createSignalCommand saves a signal, always disabled like strategies.
func (c *cli) createSignalCommand() *cobra.Command {
	var rules signalRuleFlags
	var message string
	var marketCap marketCapFlags
	var addIndicators bool
	command := &cobra.Command{
		Use:   "create NAME --expr EXPR --direction long|short|sideways [--target-ratio STOPS] [--window CANDLES] [--min-market-cap USD] [--max-market-cap USD] [--message TEXT] [--add-indicators]",
		Short: "Save a disabled signal, which buys nothing and announces the move it expects",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			move, err := rules.parse(command)
			if err != nil {
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
			if _, err := c.prepareStrategyRules(command, client, rules.rules(), addIndicators); err != nil {
				return err
			}
			created, err := client.CreateStrategyWithResponse(command.Context(), apiclient.StrategyInput{
				Name: args[0], Signal: true, Direction: move, Expression: rules.expression,
				TargetRatio: new(apiclient.SignalTargetRatio(rules.settings.ratio)), Window: new(apiclient.SignalWindow(rules.settings.window)),
				MinMarketCapUsd: minimum, MaxMarketCapUsd: maximum, Message: message,
			})
			if err == nil {
				err = c.check(created, created.JSON201 != nil, created.JSON400, created.JSON409)
			}
			if err != nil {
				return err
			}
			c.print(command, created.Body, func(w io.Writer) { renderStrategies(w, signalsKind, []apiclient.Strategy{*created.JSON201}) })
			return nil
		},
	}
	rules.add(command)
	flags := command.Flags()
	marketCap.addCreate(flags, "signal")
	flags.StringVar(&message, "message", "", "Telegram alert `TEXT`; empty keeps the generated text")
	flags.BoolVar(&addIndicators, "add-indicators", false, addIndicatorsUsage)
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
	flags.StringVar(&f.direction, "direction", "", "the `MOVE` the signal expects: long, short, or sideways")
	f.settings.add(command, true)
	_ = command.MarkFlagRequired("expr")
	_ = command.MarkFlagRequired("direction")
	_ = command.RegisterFlagCompletionFunc("direction", signalsKind.completeDirections())
}

// parse checks the move and the settings and returns the move.
func (f signalRuleFlags) parse(command *cobra.Command) (apiclient.Direction, error) {
	move, err := signalsKind.directionFlag(f.direction)
	if err != nil {
		return "", err
	}
	return move, f.settings.check(command.Flags())
}

// rules lists the expression for prepareStrategyRules.
func (f signalRuleFlags) rules() []strategyRule {
	return []strategyRule{{f.expression, apiclient.StrategyValidationInputKindEntry, "the entry rule"}}
}

func (c *cli) updateSignalCommand() *cobra.Command {
	var name, expression, message string
	var marketCap marketCapFlags
	var settings signalFlags
	var addIndicators bool
	command := &cobra.Command{
		Use:               "update ID [--expr EXPR] [--target-ratio STOPS] [--window CANDLES] [--min-market-cap USD] [--max-market-cap USD] [--name NAME] [--message TEXT] [--add-indicators]",
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
			if !slices.ContainsFunc(append(edits, "add-indicators"), flags.Changed) {
				return usageError("specify at least one of --expr, --target-ratio, --window, --min-market-cap, --max-market-cap, --name, --message, or --add-indicators")
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
			body := apiclient.StrategyUpdate{
				Name: current.Name, Signal: current.Signal, Direction: current.Direction, Expression: current.Expression,
				TargetRatio: current.TargetRatio, Window: current.Window,
				MinMarketCapUsd: current.MinMarketCapUsd, MaxMarketCapUsd: current.MaxMarketCapUsd, Message: current.Message,
			}
			settings.update(flags, &body)
			marketCap.update(flags, &body, minimum, maximum)
			if flags.Changed("name") {
				body.Name = name
			}
			if flags.Changed("message") {
				body.Message = message
			}
			var rules []strategyRule
			// --add-indicators covers the preserved rule too.
			if flags.Changed("expr") || addIndicators {
				if flags.Changed("expr") {
					body.Expression = expression
				}
				rules = append(rules, strategyRule{body.Expression, apiclient.StrategyValidationInputKindEntry, "the entry rule"})
			}
			return c.update(command, client, signalsKind, *current, body, rules, addIndicators, slices.ContainsFunc(edits, flags.Changed))
		},
	}
	command.Flags().StringVar(&expression, "expr", "", "the entry rule `EXPR`ession; omitted preserves the current one")
	settings.add(command, false)
	marketCap.addUpdate(command.Flags())
	command.Flags().StringVar(&name, "name", "", "signal `NAME`; omitted preserves the current name")
	command.Flags().StringVar(&message, "message", "", "Telegram alert `TEXT`; empty restores generated text, omitted preserves it")
	command.Flags().BoolVar(&addIndicators, "add-indicators", false, addIndicatorsUsage)
	return command
}
