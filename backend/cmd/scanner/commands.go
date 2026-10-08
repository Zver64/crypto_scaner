package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"crypto-scanner/internal/apiclient"

	"github.com/spf13/cobra"
)

// print writes the raw response with --json, or the text rendering.
func (c *cli) print(command *cobra.Command, raw []byte, text func(io.Writer)) {
	if c.json {
		command.Println(strings.TrimSpace(string(raw)))
		return
	}
	text(command.OutOrStdout())
}

// printJSON writes value as the --json output of a command whose API
// response has no body of its own.
func printJSON(command *cobra.Command, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	command.Println(string(raw))
	return nil
}

func (c *cli) varsCommand() *cobra.Command {
	var filter string
	command := &cobra.Command{
		Use:   "vars",
		Short: "Variables: configured indicators, candle fields, and the position variables of exit rules",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, err := c.client()
			if err != nil {
				return err
			}
			variables, err := client.ListStrategyVariablesWithResponse(command.Context())
			if err == nil {
				err = c.check(variables, variables.JSON200 != nil)
			}
			if err != nil {
				return err
			}
			// The filter applies to the JSON too.
			list := *variables.JSON200
			needle := strings.ToLower(filter)
			list.Items = slices.DeleteFunc(list.Items, func(variable apiclient.StrategyVariable) bool {
				return !strings.Contains(strings.ToLower(variable.Name), needle)
			})
			raw, err := json.Marshal(list)
			if err != nil {
				return err
			}
			c.print(command, raw, func(w io.Writer) {
				for _, variable := range list.Items {
					if variable.Position {
						fmt.Fprintln(w, variable.Name, "(exit rule only)")
					} else {
						fmt.Fprintln(w, variable.Name)
					}
				}
			})
			return nil
		},
	}
	command.Flags().StringVar(&filter, "filter", "", "list only names containing `TEXT`, ignoring case")
	return command
}

func (c *cli) validateCommand() *cobra.Command {
	var exit, price bool
	command := &cobra.Command{
		Use:   "validate EXPR [--exit | --price]",
		Short: "Check an entry rule, an exit rule, or a take profit or stop loss price",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, err := c.client()
			if err != nil {
				return err
			}
			kind := apiclient.StrategyValidationInputKindEntry
			switch {
			case exit:
				kind = apiclient.StrategyValidationInputKindExit
			case price:
				kind = apiclient.StrategyValidationInputKindPrice
			}
			validation, err := client.ValidateStrategyWithResponse(command.Context(), apiclient.StrategyValidationInput{Expression: args[0], Kind: &kind})
			if err == nil {
				err = c.check(validation, validation.JSON200 != nil, validation.JSON400)
			}
			if err != nil {
				return err
			}
			if problems := validation.JSON200.Errors; len(problems) > 0 {
				return &cliError{code: "invalid_expression", message: "the expression is invalid", details: problems}
			}
			c.print(command, validation.Body, func(w io.Writer) { renderValidation(w, *validation.JSON200) })
			return nil
		},
	}
	command.Flags().BoolVar(&exit, "exit", false, "check an exit rule, which may also read entry_price, pnl, and bars_held")
	command.Flags().BoolVar(&price, "price", false, "check a take profit or stop loss price, such as h_close * 1.05")
	command.MarkFlagsMutuallyExclusive("exit", "price")
	return command
}

func (c *cli) favoritesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "favorites",
		Short: "The administrator's favorites, the only coins of() reads",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, err := c.client()
			if err != nil {
				return err
			}
			favorites, err := client.ListFavoritesWithResponse(command.Context())
			if err == nil {
				err = c.check(favorites, favorites.JSON200 != nil)
			}
			if err != nil {
				return err
			}
			c.print(command, favorites.Body, func(w io.Writer) {
				symbols := make([]string, len(favorites.JSON200.Items))
				for i, favorite := range favorites.JSON200.Items {
					symbols[i] = favorite.Symbol
					if !favorite.Active {
						symbols[i] += "(inactive)"
					}
				}
				fmt.Fprintln(w, strings.Join(symbols, " "))
			})
			return nil
		},
	}
}

func (c *cli) strategiesCommand() *cobra.Command {
	command := c.listCommand(strategiesKind, "Saved strategies: ID, state, name with any problem and message, entry, exits, buys, market cap")
	command.AddCommand(c.createStrategyCommand(), c.updateStrategyCommand(), c.deleteCommand(strategiesKind))
	return command
}

func (c *cli) signalsCommand() *cobra.Command {
	command := c.listCommand(signalsKind, "Saved signals: ID, state, name with any problem and message, entry, expected move")
	command.AddCommand(c.createSignalCommand(), c.updateSignalCommand(), c.deleteCommand(signalsKind))
	return command
}

// listCommand lists the saved strategies of k.
func (c *cli) listCommand(k kind, short string) *cobra.Command {
	return &cobra.Command{
		Use:   k.plural,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, err := c.client()
			if err != nil {
				return err
			}
			strategies, err := client.ListStrategiesWithResponse(command.Context())
			if err == nil {
				err = c.check(strategies, strategies.JSON200 != nil)
			}
			if err != nil {
				return err
			}
			items := slices.DeleteFunc(strategies.JSON200.Items, func(strategy apiclient.Strategy) bool { return !k.holds(strategy) })
			raw, err := json.Marshal(apiclient.StrategyList{Items: items})
			if err != nil {
				return err
			}
			c.print(command, raw, func(w io.Writer) {
				if len(items) == 0 {
					fmt.Fprintf(w, "no %s; create one with scanner %s create\n", k.plural, k.plural)
					return
				}
				renderStrategies(w, k, items)
			})
			return nil
		},
	}
}

// strategyID parses the ID argument of a saved strategy.
func strategyID(arg string) (int64, error) {
	id, err := strconv.ParseInt(arg, 10, 64)
	if err != nil || id <= 0 {
		return 0, usageError("strategy ID %q must be a positive int64", arg)
	}
	return id, nil
}

func (c *cli) deleteCommand(k kind) *cobra.Command {
	return &cobra.Command{
		Use:               "delete ID",
		Short:             "Delete a saved " + k.noun,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: c.completeArg(k),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := strategyID(args[0])
			if err != nil {
				return err
			}
			client, err := c.client()
			if err != nil {
				return err
			}
			if _, err := c.saved(command, client, k, id); err != nil {
				return err
			}
			deleted, err := client.DeleteStrategyWithResponse(command.Context(), id)
			if err == nil {
				err = c.check(deleted, deleted.StatusCode() == http.StatusNoContent, deleted.JSON404)
			}
			if err != nil {
				return err
			}
			if c.json {
				return printJSON(command, map[string]int{"deleted": 1})
			}
			command.Printf("deleted %s %d\n", k.noun, id)
			return nil
		},
	}
}

// createStrategyCommand saves a strategy the way the Mini App does, always
// disabled: only the administrator turns alerts on, in the Mini App.
func (c *cli) createStrategyCommand() *cobra.Command {
	var expression, direction, exit, takeProfit, stopLoss, minMarketCap, maxMarketCap, message string
	var addIndicators bool
	command := &cobra.Command{
		Use:   "create NAME --expr EXPR [--direction long|short] [--exit EXPR] [--take-profit EXPR] [--stop-loss EXPR] [--min-market-cap USD] [--max-market-cap USD] [--message TEXT] [--add-indicators]",
		Short: "Save a disabled strategy; it reads indicators that are not configured without adding them",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			body := apiclient.StrategyInput{
				Name: args[0], Expression: expression, ExitExpression: exit, TakeProfitExpression: takeProfit, StopLossExpression: stopLoss, Message: message,
			}
			var err error
			if body.Direction, err = strategiesKind.directionFlag(direction); err != nil {
				return err
			}
			if body.MinMarketCapUsd, err = marketCapFlag("min-market-cap", minMarketCap); err != nil {
				return err
			}
			if body.MaxMarketCapUsd, err = marketCapFlag("max-market-cap", maxMarketCap); err != nil {
				return err
			}
			client, err := c.client()
			if err != nil {
				return err
			}
			if _, err := c.prepareStrategyRules(command, client, []strategyRule{
				{expression, apiclient.StrategyValidationInputKindEntry, "the entry rule"},
				{exit, apiclient.StrategyValidationInputKindExit, "the exit rule"},
				{takeProfit, apiclient.StrategyValidationInputKindPrice, "the take profit"},
				{stopLoss, apiclient.StrategyValidationInputKindPrice, "the stop loss"},
			}, addIndicators); err != nil {
				return err
			}
			created, err := client.CreateStrategyWithResponse(command.Context(), body)
			if err == nil {
				err = c.check(created, created.JSON201 != nil, created.JSON400, created.JSON409)
			}
			if err != nil {
				return err
			}
			c.print(command, created.Body, func(w io.Writer) { renderStrategies(w, strategiesKind, []apiclient.Strategy{*created.JSON201}) })
			return nil
		},
	}
	flags := command.Flags()
	flags.StringVar(&expression, "expr", "", "the entry rule `EXPR`ession, see docs/strategy-language.md")
	flags.StringVar(&direction, "direction", string(apiclient.Long), "how the strategy trades, `long` or short, fixed once saved; a short one needs --take-profit and --stop-loss")
	flags.StringVar(&exit, "exit", "", "the exit rule `EXPR`ession, which may also read entry_price, pnl, and bars_held")
	flags.StringVar(&takeProfit, "take-profit", "", "the take profit price `EXPR`ession, fixed at the entry signal, such as h_close * 1.05")
	flags.StringVar(&stopLoss, "stop-loss", "", "the stop loss price `EXPR`ession, fixed at the entry signal, such as h_close * 0.97")
	flags.StringVar(&minMarketCap, "min-market-cap", "", "buy only coins whose market cap is at least `USD`, such as 50M; backtests ignore it")
	flags.StringVar(&maxMarketCap, "max-market-cap", "", "buy only coins whose market cap is at most `USD`, such as 2B; backtests ignore it")
	flags.StringVar(&message, "message", "", "Telegram alert `TEXT`; empty keeps the generated text")
	flags.BoolVar(&addIndicators, "add-indicators", false, addIndicatorsUsage)
	_ = command.MarkFlagRequired("expr")
	_ = command.RegisterFlagCompletionFunc("direction", strategiesKind.completeDirections())
	return command
}

// marketCapFlag parses the market cap bound of flag, nil when empty.
func marketCapFlag(flag, value string) (*float64, error) {
	usd, err := parseUSD(value)
	if err != nil {
		return nil, usageError("--%s: %v", flag, err)
	}
	return usd, nil
}

// strategyRule is an expression of a strategy, of kind, that name names in
// errors.
type strategyRule struct {
	expression string
	kind       apiclient.StrategyValidationInputKind
	name       string
}

// addIndicatorsUsage describes the flag that adds the indicators a strategy
// reads but nobody configured, which it reads all the same.
const addIndicatorsUsage = "also add the indicators the rules read that are not configured, with no table columns or chart lines, so the Mini App builder can show the rules"

// prepareStrategyRules validates every rule before any write and, with add,
// then adds the indicators they read that are missing, so an invalid rule
// adds nothing. Every invalid rule is reported at once. It returns the
// titles of the indicators the rules read that are missing. An empty rule
// other than the entry rule is absent and needs nothing.
func (c *cli) prepareStrategyRules(command *cobra.Command, client *apiclient.ClientWithResponses, rules []strategyRule, add bool) ([]string, error) {
	var items []apiclient.ScannerIndicatorBatchItem
	var titles, problems []string
	for _, rule := range rules {
		if rule.expression == "" && rule.kind != apiclient.StrategyValidationInputKindEntry {
			continue
		}
		validation, err := client.ValidateStrategyWithResponse(command.Context(), apiclient.StrategyValidationInput{Expression: rule.expression, Kind: &rule.kind})
		if err == nil {
			err = c.check(validation, validation.JSON200 != nil, validation.JSON400)
		}
		if err != nil {
			return nil, err
		}
		for _, problem := range validation.JSON200.Errors {
			problems = append(problems, rule.name+": "+problem)
		}
		for _, indicator := range validation.JSON200.MissingIndicators {
			if !slices.Contains(titles, indicator.Title) {
				titles = append(titles, indicator.Title)
				items = append(items, apiclient.ScannerIndicatorBatchItem{Interval: indicator.Interval, Parameters: indicator.Parameters, Type: indicator.Type})
			}
		}
	}
	if len(problems) > 0 {
		return nil, &cliError{code: "invalid_expression", message: "invalid rules; nothing was saved", details: problems}
	}
	if !add || len(items) == 0 {
		return titles, nil
	}
	added, err := client.CreateScannerIndicatorBatchWithResponse(command.Context(), apiclient.ScannerIndicatorBatch{Items: items})
	if err == nil {
		err = c.check(added, added.JSON201 != nil, added.JSON400, added.JSON409)
	}
	if err != nil {
		return nil, err
	}
	if !c.json {
		command.PrintErrf("added indicators: %s\n", strings.Join(titles, ", "))
	}
	return titles, nil
}

func (c *cli) updateStrategyCommand() *cobra.Command {
	var name, expression, exit, takeProfit, stopLoss, minMarketCap, maxMarketCap, message string
	var addIndicators bool
	command := &cobra.Command{
		Use:               "update ID [--expr EXPR] [--exit EXPR] [--take-profit EXPR] [--stop-loss EXPR] [--min-market-cap USD] [--max-market-cap USD] [--name NAME] [--message TEXT] [--add-indicators]",
		Short:             "Edit a disabled saved strategy, preserving unspecified fields",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: c.completeArg(strategiesKind),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := strategyID(args[0])
			if err != nil {
				return err
			}
			flags := command.Flags()
			// edits are the flags that change the strategy.
			edits := []string{"expr", "exit", "take-profit", "stop-loss", "min-market-cap", "max-market-cap", "name", "message"}
			if !slices.ContainsFunc(append(edits, "add-indicators"), flags.Changed) {
				return usageError("specify at least one of --expr, --exit, --take-profit, --stop-loss, --min-market-cap, --max-market-cap, --name, --message, or --add-indicators")
			}
			minimum, err := marketCapFlag("min-market-cap", minMarketCap)
			if err != nil {
				return err
			}
			maximum, err := marketCapFlag("max-market-cap", maxMarketCap)
			if err != nil {
				return err
			}
			client, err := c.client()
			if err != nil {
				return err
			}
			current, err := c.editable(command, client, strategiesKind, id)
			if err != nil {
				return err
			}
			body := apiclient.StrategyUpdate{
				Name: current.Name, Signal: current.Signal, Direction: current.Direction, Expression: current.Expression, ExitExpression: current.ExitExpression,
				TakeProfitExpression: current.TakeProfitExpression, StopLossExpression: current.StopLossExpression,
				MinMarketCapUsd: current.MinMarketCapUsd, MaxMarketCapUsd: current.MaxMarketCapUsd, Message: current.Message,
			}
			if flags.Changed("min-market-cap") {
				body.MinMarketCapUsd = minimum
			}
			if flags.Changed("max-market-cap") {
				body.MaxMarketCapUsd = maximum
			}
			if flags.Changed("name") {
				body.Name = name
			}
			if flags.Changed("message") {
				body.Message = message
			}
			var rules []strategyRule
			for _, change := range []struct {
				flag   string
				target *string
				rule   strategyRule
			}{
				{"expr", &body.Expression, strategyRule{expression, apiclient.StrategyValidationInputKindEntry, "the entry rule"}},
				{"exit", &body.ExitExpression, strategyRule{exit, apiclient.StrategyValidationInputKindExit, "the exit rule"}},
				{"take-profit", &body.TakeProfitExpression, strategyRule{takeProfit, apiclient.StrategyValidationInputKindPrice, "the take profit"}},
				{"stop-loss", &body.StopLossExpression, strategyRule{stopLoss, apiclient.StrategyValidationInputKindPrice, "the stop loss"}},
			} {
				// --add-indicators covers the preserved rules too.
				if flags.Changed(change.flag) {
					*change.target = change.rule.expression
				} else if !addIndicators {
					continue
				}
				change.rule.expression = *change.target
				rules = append(rules, change.rule)
			}
			return c.update(command, client, strategiesKind, *current, body, rules, addIndicators, slices.ContainsFunc(edits, flags.Changed))
		},
	}
	command.Flags().StringVar(&expression, "expr", "", "the entry rule `EXPR`ession; omitted preserves the current one")
	command.Flags().StringVar(&exit, "exit", "", "the exit rule `EXPR`ession; empty removes it, omitted preserves it")
	command.Flags().StringVar(&takeProfit, "take-profit", "", "the take profit price `EXPR`ession; empty removes it, omitted preserves it")
	command.Flags().StringVar(&stopLoss, "stop-loss", "", "the stop loss price `EXPR`ession; empty removes it, omitted preserves it")
	command.Flags().StringVar(&minMarketCap, "min-market-cap", "", "the minimum market cap in `USD`, such as 50M; empty removes it, omitted preserves it")
	command.Flags().StringVar(&maxMarketCap, "max-market-cap", "", "the maximum market cap in `USD`, such as 2B; empty removes it, omitted preserves it")
	command.Flags().StringVar(&name, "name", "", "strategy `NAME`; omitted preserves the current name")
	command.Flags().StringVar(&message, "message", "", "Telegram alert `TEXT`; empty restores generated text, omitted preserves it")
	command.Flags().BoolVar(&addIndicators, "add-indicators", false, addIndicatorsUsage)
	return command
}

func (c *cli) backtestCommand() *cobra.Command {
	var strategy, symbol, from, to string
	command := &cobra.Command{
		Use:   "backtest --strategy ID --symbol SYM [--from TIME] [--to TIME]",
		Short: "Simulate the trades of a saved strategy on one coin against its baselines",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			id, err := strategyID(strategy)
			if err != nil {
				return err
			}
			params := apiclient.BacktestStrategyParams{Symbol: symbol}
			for _, bound := range []struct {
				flag, value string
				end         bool
				target      **time.Time
			}{{"from", from, false, &params.From}, {"to", to, true, &params.To}} {
				if bound.value == "" {
					continue
				}
				parsed, err := parseTime(bound.value, bound.end)
				if err != nil {
					return usageError("--%s: %v", bound.flag, err)
				}
				*bound.target = &parsed
			}
			client, err := c.client()
			if err != nil {
				return err
			}
			backtest, err := client.BacktestStrategyWithResponse(command.Context(), id, &params)
			if err == nil {
				err = c.check(backtest, backtest.JSON200 != nil, backtest.JSON400, backtest.JSON404, backtest.JSON503)
			}
			if err != nil {
				return err
			}
			period := params.From != nil || params.To != nil
			c.print(command, backtest.Body, func(w io.Writer) { renderBacktest(w, *backtest.JSON200, period) })
			return nil
		},
	}
	flags := command.Flags()
	flags.StringVar(&strategy, "strategy", "", "the saved strategy `ID` (see scanner strategies)")
	flags.StringVar(&symbol, "symbol", "", "any coin `SYM`, such as BTCUSDT")
	flags.StringVar(&from, "from", "", "evaluate candles opening at or after `TIME`: a UTC date such as 2026-01-31, or RFC 3339")
	flags.StringVar(&to, "to", "", "evaluate candles opening at or before `TIME`: a UTC date, the whole day included, or RFC 3339")
	_ = command.MarkFlagRequired("strategy")
	_ = command.MarkFlagRequired("symbol")
	_ = command.RegisterFlagCompletionFunc("strategy", c.completeStrategies)
	_ = command.RegisterFlagCompletionFunc("symbol", c.completeFavorites)
	return command
}

// parseTime reads a UTC date, its start or, with end, the last second of
// that day, or an RFC 3339 time.
func parseTime(value string, end bool) (time.Time, error) {
	if parsed, err := time.Parse(time.DateOnly, value); err == nil {
		if end {
			parsed = parsed.AddDate(0, 0, 1).Add(-time.Second)
		}
		return parsed, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%q is neither a date such as 2026-01-31 nor an RFC 3339 time", value)
	}
	return parsed.UTC(), nil
}
