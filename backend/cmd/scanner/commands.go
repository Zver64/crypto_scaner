package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

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
		Short: "Variables of the configured indicators and candle fields",
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
	return &cobra.Command{
		Use:   "validate EXPR",
		Short: "Check an expression",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, err := c.client()
			if err != nil {
				return err
			}
			validation, err := client.ValidateStrategyWithResponse(command.Context(), apiclient.StrategyValidationInput{Expression: args[0]})
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
	var expression, message string
	command := &cobra.Command{
		Use:   "create NAME --expr EXPR [--message TEXT]",
		Short: "Save a disabled strategy, adding the indicators it reads that are not configured",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, err := c.client()
			if err != nil {
				return err
			}
			if err := prepareStrategyExpression(command, client, expression); err != nil {
				return err
			}
			created, err := client.CreateStrategyWithResponse(command.Context(), apiclient.StrategyInput{Name: args[0], Expression: expression, Message: message})
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
	command.Flags().StringVar(&expression, "expr", "", "the strategy `EXPR`ession, see docs/strategy-language.md")
	command.Flags().StringVar(&message, "message", "", "Telegram alert `TEXT`; empty keeps the generated text")
	_ = command.MarkFlagRequired("expr")
	return command
}

// prepareStrategyExpression validates before any write and adds missing indicators.
func prepareStrategyExpression(command *cobra.Command, client *apiclient.ClientWithResponses, expression string) error {
	validation, err := client.ValidateStrategyWithResponse(command.Context(), apiclient.StrategyValidationInput{Expression: expression})
	if err == nil {
		err = check(validation, validation.JSON200 != nil, validation.JSON400)
	}
	if err != nil {
		return err
	}
	if len(validation.JSON200.Errors) > 0 {
		renderValidation(command.OutOrStdout(), *validation.JSON200)
		return errors.New("the expression is invalid")
	}
	if missing := validation.JSON200.MissingIndicators; len(missing) > 0 {
		items := make([]apiclient.ScannerIndicatorBatchItem, len(missing))
		for i, indicator := range missing {
			items[i] = apiclient.ScannerIndicatorBatchItem{Interval: indicator.Interval, Parameters: indicator.Parameters, Type: indicator.Type}
		}
		added, err := client.CreateScannerIndicatorBatchWithResponse(command.Context(), apiclient.ScannerIndicatorBatch{Items: items})
		if err == nil {
			err = check(added, added.JSON201 != nil, added.JSON400, added.JSON409)
		}
		if err != nil {
			return fmt.Errorf("add the indicators: %w", err)
		}
	}
	return nil
}

func (c *cli) updateStrategyCommand() *cobra.Command {
	var name, expression, message string
	command := &cobra.Command{
		Use:   "update ID [--expr EXPR] [--name NAME] [--message TEXT]",
		Short: "Edit a disabled saved strategy, preserving unspecified fields",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := strategyID(args[0])
			if err != nil {
				return err
			}
			flags := command.Flags()
			if !flags.Changed("expr") && !flags.Changed("name") && !flags.Changed("message") {
				return errors.New("specify at least one of --expr, --name, or --message")
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
			body := apiclient.StrategyUpdate{Name: current.Name, Expression: current.Expression, Message: current.Message}
			if flags.Changed("name") {
				body.Name = name
			}
			if flags.Changed("message") {
				body.Message = message
			}
			if flags.Changed("expr") {
				body.Expression = expression
				if err := prepareStrategyExpression(command, client, expression); err != nil {
					return err
				}
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
	command.Flags().StringVar(&expression, "expr", "", "the strategy `EXPR`ession; omitted preserves the current expression")
	command.Flags().StringVar(&name, "name", "", "strategy `NAME`; omitted preserves the current name")
	command.Flags().StringVar(&message, "message", "", "Telegram alert `TEXT`; empty restores generated text, omitted preserves it")
	return command
}

func (c *cli) backtestCommand() *cobra.Command {
	var (
		strategyID int64
		symbol     string
		hold       int
	)
	command := &cobra.Command{
		Use:   "backtest --strategy ID --symbol SYM [--hold N]",
		Short: "Simulate the trades of a saved strategy on one coin against its baselines",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, err := c.client()
			if err != nil {
				return err
			}
			params := apiclient.BacktestStrategyParams{Symbol: symbol}
			if command.Flags().Changed("hold") {
				params.Hold = &hold
			}
			backtest, err := client.BacktestStrategyWithResponse(command.Context(), strategyID, &params)
			if err == nil {
				err = check(backtest, backtest.JSON200 != nil, backtest.JSON400, backtest.JSON404, backtest.JSON503)
			}
			if err != nil {
				return err
			}
			c.print(command, backtest.Body, func(w io.Writer) { renderBacktest(w, *backtest.JSON200) })
			return nil
		},
	}
	flags := command.Flags()
	flags.Int64Var(&strategyID, "strategy", 0, "the saved strategy `ID` (see scanner strategies)")
	flags.StringVar(&symbol, "symbol", "", "any coin `SYM`, such as BTCUSDT")
	flags.IntVar(&hold, "hold", 0, "candles every trade holds, 1 to 1000 (`N`; default: 1h 24, 1d 7, 1w 4, 1M 3)")
	_ = command.MarkFlagRequired("strategy")
	_ = command.MarkFlagRequired("symbol")
	return command
}
