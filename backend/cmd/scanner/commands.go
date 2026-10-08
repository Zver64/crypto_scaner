package main

import (
	"errors"
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
				err = check(variables, variables.JSON200 != nil)
			}
			if err != nil {
				return err
			}
			c.print(command, variables.Body, func(w io.Writer) {
				needle := strings.ToLower(filter)
				for _, variable := range variables.JSON200.Items {
					if strings.Contains(strings.ToLower(variable.Name), needle) {
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
				err = check(validation, validation.JSON200 != nil, validation.JSON400)
			}
			if err != nil {
				return err
			}
			c.print(command, validation.Body, func(w io.Writer) { renderValidation(w, *validation.JSON200) })
			if len(validation.JSON200.Errors) > 0 {
				return errors.New("the expression is invalid")
			}
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
				err = check(favorites, favorites.JSON200 != nil)
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
	command := &cobra.Command{
		Use:   "strategies",
		Short: "Saved strategies: id, on/off, name, expression",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, err := c.client()
			if err != nil {
				return err
			}
			strategies, err := client.ListStrategiesWithResponse(command.Context())
			if err == nil {
				err = check(strategies, strategies.JSON200 != nil)
			}
			if err != nil {
				return err
			}
			c.print(command, strategies.Body, func(w io.Writer) { renderStrategies(w, strategies.JSON200.Items) })
			return nil
		},
	}
	command.AddCommand(c.createStrategyCommand(), c.updateStrategyCommand(), c.deleteStrategyCommand())
	return command
}

// strategyID parses the ID argument of a saved strategy.
func strategyID(arg string) (int64, error) {
	id, err := strconv.ParseInt(arg, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("strategy ID %q must be a positive int64", arg)
	}
	return id, nil
}

func (c *cli) deleteStrategyCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "delete ID",
		Short: "Delete a saved strategy (--json produces no output on success)",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := strategyID(args[0])
			if err != nil {
				return err
			}
			client, err := c.client()
			if err != nil {
				return err
			}
			deleted, err := client.DeleteStrategyWithResponse(command.Context(), id)
			if err == nil {
				err = check(deleted, deleted.StatusCode() == http.StatusNoContent, deleted.JSON404)
			}
			if err != nil {
				return err
			}
			if !c.json {
				command.Printf("deleted strategy %d\n", id)
			}
			return nil
		},
	}
}

// createStrategyCommand saves a strategy the way the Mini App does, always
// disabled: only the administrator turns alerts on, in the Mini App.
func (c *cli) createStrategyCommand() *cobra.Command {
	var expression, exit, takeProfit, stopLoss, message string
	var addIndicators bool
	command := &cobra.Command{
		Use:   "create NAME --expr EXPR [--exit EXPR] [--take-profit EXPR] [--stop-loss EXPR] [--message TEXT] [--add-indicators]",
		Short: "Save a disabled strategy; it reads indicators that are not configured without adding them",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, err := c.client()
			if err != nil {
				return err
			}
			if _, err := prepareStrategyRules(command, client, []strategyRule{
				{expression, apiclient.StrategyValidationInputKindEntry, "the expression"},
				{exit, apiclient.StrategyValidationInputKindExit, "the exit rule"},
				{takeProfit, apiclient.StrategyValidationInputKindPrice, "the take profit"},
				{stopLoss, apiclient.StrategyValidationInputKindPrice, "the stop loss"},
			}, addIndicators); err != nil {
				return err
			}
			created, err := client.CreateStrategyWithResponse(command.Context(), apiclient.StrategyInput{
				Name: args[0], Expression: expression, ExitExpression: exit, TakeProfitExpression: takeProfit, StopLossExpression: stopLoss, Message: message,
			})
			if err == nil {
				err = check(created, created.JSON201 != nil, created.JSON400, created.JSON409)
			}
			if err != nil {
				return err
			}
			c.print(command, created.Body, func(w io.Writer) { renderStrategies(w, []apiclient.Strategy{*created.JSON201}) })
			return nil
		},
	}
	flags := command.Flags()
	flags.StringVar(&expression, "expr", "", "the entry rule `EXPR`ession, see docs/strategy-language.md")
	flags.StringVar(&exit, "exit", "", "the exit rule `EXPR`ession, which may also read entry_price, pnl, and bars_held")
	flags.StringVar(&takeProfit, "take-profit", "", "the take profit price `EXPR`ession, fixed at the entry signal, such as h_close * 1.05")
	flags.StringVar(&stopLoss, "stop-loss", "", "the stop loss price `EXPR`ession, fixed at the entry signal, such as h_close * 0.97")
	flags.StringVar(&message, "message", "", "Telegram alert `TEXT`; empty keeps the generated text")
	flags.BoolVar(&addIndicators, "add-indicators", false, addIndicatorsUsage)
	_ = command.MarkFlagRequired("expr")
	return command
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
// adds nothing. It returns how many indicators the rules read that are
// missing. An empty rule other than the entry rule is absent and needs
// nothing.
func prepareStrategyRules(command *cobra.Command, client *apiclient.ClientWithResponses, rules []strategyRule, add bool) (int, error) {
	var items []apiclient.ScannerIndicatorBatchItem
	var titles []string
	for _, rule := range rules {
		if rule.expression == "" && rule.kind != apiclient.StrategyValidationInputKindEntry {
			continue
		}
		validation, err := client.ValidateStrategyWithResponse(command.Context(), apiclient.StrategyValidationInput{Expression: rule.expression, Kind: &rule.kind})
		if err == nil {
			err = check(validation, validation.JSON200 != nil, validation.JSON400)
		}
		if err != nil {
			return 0, err
		}
		if len(validation.JSON200.Errors) > 0 {
			renderValidation(command.OutOrStdout(), *validation.JSON200)
			return 0, fmt.Errorf("%s is invalid", rule.name)
		}
		for _, indicator := range validation.JSON200.MissingIndicators {
			if !slices.Contains(titles, indicator.Title) {
				titles = append(titles, indicator.Title)
				items = append(items, apiclient.ScannerIndicatorBatchItem{Interval: indicator.Interval, Parameters: indicator.Parameters, Type: indicator.Type})
			}
		}
	}
	if !add || len(items) == 0 {
		return len(items), nil
	}
	added, err := client.CreateScannerIndicatorBatchWithResponse(command.Context(), apiclient.ScannerIndicatorBatch{Items: items})
	if err == nil {
		err = check(added, added.JSON201 != nil, added.JSON400, added.JSON409)
	}
	if err != nil {
		return 0, fmt.Errorf("add the indicators: %w", err)
	}
	return len(items), nil
}

func (c *cli) updateStrategyCommand() *cobra.Command {
	var name, expression, exit, takeProfit, stopLoss, message string
	var addIndicators bool
	command := &cobra.Command{
		Use:   "update ID [--expr EXPR] [--exit EXPR] [--take-profit EXPR] [--stop-loss EXPR] [--name NAME] [--message TEXT] [--add-indicators]",
		Short: "Edit a disabled saved strategy, preserving unspecified fields",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := strategyID(args[0])
			if err != nil {
				return err
			}
			flags := command.Flags()
			// edits are the flags that change the strategy.
			edits := []string{"expr", "exit", "take-profit", "stop-loss", "name", "message"}
			if !slices.ContainsFunc(append(edits, "add-indicators"), flags.Changed) {
				return errors.New("specify at least one of --expr, --exit, --take-profit, --stop-loss, --name, --message, or --add-indicators")
			}
			client, err := c.client()
			if err != nil {
				return err
			}
			listed, err := client.ListStrategiesWithResponse(command.Context())
			if err == nil {
				err = check(listed, listed.JSON200 != nil)
			}
			if err != nil {
				return err
			}
			var current *apiclient.Strategy
			for _, entry := range listed.JSON200.Items {
				if entry.Id == id {
					current = &entry
					break
				}
			}
			if current == nil {
				return fmt.Errorf("no strategy %d; see scanner strategies", id)
			}
			// Enabled strategies alert; only the Mini App edits them.
			if current.Enabled {
				return fmt.Errorf("strategy %d is enabled; edit it in the Mini App", id)
			}
			body := apiclient.StrategyUpdate{
				Name: current.Name, Expression: current.Expression, ExitExpression: current.ExitExpression,
				TakeProfitExpression: current.TakeProfitExpression, StopLossExpression: current.StopLossExpression, Message: current.Message,
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
				{"expr", &body.Expression, strategyRule{expression, apiclient.StrategyValidationInputKindEntry, "the expression"}},
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
			missing, err := prepareStrategyRules(command, client, rules, addIndicators)
			if err != nil {
				return err
			}
			// Adding nothing changes nothing, so nothing is sent.
			if missing == 0 && !slices.ContainsFunc(edits, flags.Changed) {
				if !c.json {
					command.Println("no indicators to add")
				}
				return nil
			}
			updated, err := client.UpdateStrategyWithResponse(command.Context(), id, body)
			if err == nil {
				err = check(updated, updated.JSON200 != nil, updated.JSON400, updated.JSON404, updated.JSON409)
			}
			if err != nil {
				return err
			}
			c.print(command, updated.Body, func(w io.Writer) { renderStrategies(w, []apiclient.Strategy{*updated.JSON200}) })
			return nil
		},
	}
	command.Flags().StringVar(&expression, "expr", "", "the entry rule `EXPR`ession; omitted preserves the current one")
	command.Flags().StringVar(&exit, "exit", "", "the exit rule `EXPR`ession; empty removes it, omitted preserves it")
	command.Flags().StringVar(&takeProfit, "take-profit", "", "the take profit price `EXPR`ession; empty removes it, omitted preserves it")
	command.Flags().StringVar(&stopLoss, "stop-loss", "", "the stop loss price `EXPR`ession; empty removes it, omitted preserves it")
	command.Flags().StringVar(&name, "name", "", "strategy `NAME`; omitted preserves the current name")
	command.Flags().StringVar(&message, "message", "", "Telegram alert `TEXT`; empty restores generated text, omitted preserves it")
	command.Flags().BoolVar(&addIndicators, "add-indicators", false, addIndicatorsUsage)
	return command
}

func (c *cli) backtestCommand() *cobra.Command {
	var (
		strategyID int64
		symbol     string
		from, to   string
	)
	command := &cobra.Command{
		Use:   "backtest --strategy ID --symbol SYM [--from TIME] [--to TIME]",
		Short: "Simulate the trades of a saved strategy on one coin against its baselines",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
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
					return fmt.Errorf("--%s: %w", bound.flag, err)
				}
				*bound.target = &parsed
			}
			client, err := c.client()
			if err != nil {
				return err
			}
			backtest, err := client.BacktestStrategyWithResponse(command.Context(), strategyID, &params)
			if err == nil {
				err = check(backtest, backtest.JSON200 != nil, backtest.JSON400, backtest.JSON404, backtest.JSON503)
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
	flags.Int64Var(&strategyID, "strategy", 0, "the saved strategy `ID` (see scanner strategies)")
	flags.StringVar(&symbol, "symbol", "", "any coin `SYM`, such as BTCUSDT")
	flags.StringVar(&from, "from", "", "evaluate candles opening at or after `TIME`: a UTC date such as 2026-01-31, or RFC 3339")
	flags.StringVar(&to, "to", "", "evaluate candles opening at or before `TIME`: a UTC date, the whole day included, or RFC 3339")
	_ = command.MarkFlagRequired("strategy")
	_ = command.MarkFlagRequired("symbol")
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
