package main

import (
	"fmt"
	"strings"

	"crypto-scanner/internal/apiclient"
	"github.com/spf13/cobra"
)

type backtestCoinResult struct {
	Symbol string                      `json:"symbol"`
	Status string                      `json:"status"`
	Result *apiclient.StrategyBacktest `json:"result,omitempty"`
	Error  *apiclient.ErrorResponse    `json:"error,omitempty"`
}

func backtestSymbols(values []string) ([]string, error) {
	var symbols []string
	seen := map[string]bool{}
	for _, value := range values {
		symbol := strings.ToUpper(strings.TrimSpace(value))
		if symbol == "" {
			return nil, usageError("--symbol must not be empty")
		}
		if !seen[symbol] {
			symbols = append(symbols, symbol)
			seen[symbol] = true
		}
	}
	if len(symbols) == 0 {
		return nil, usageError("--symbol is required")
	}
	return symbols, nil
}

// runBacktests holds the machine lock for the entire sequential run.
func (c *cli) runBacktests(command *cobra.Command, target backtestFlags, request func(string) ([]byte, *apiclient.StrategyBacktest, error)) error {
	symbols, err := backtestSymbols(target.symbols)
	if err != nil {
		return err
	}
	unlock, err := lockBacktest()
	if err != nil {
		return err
	}
	defer unlock()
	if len(symbols) == 1 {
		raw, result, err := request(symbols[0])
		if err != nil {
			return err
		}
		c.printBacktest(command, raw, *result, target)
		return nil
	}
	items := make([]backtestCoinResult, len(symbols))
	for i, symbol := range symbols {
		items[i] = backtestCoinResult{Symbol: symbol, Status: "not_run"}
	}
	failed := false
	for i, symbol := range symbols {
		if command.Context().Err() != nil {
			break
		}
		fmt.Fprintf(command.ErrOrStderr(), "Backtest %d/%d: %s\n", i+1, len(symbols), symbol)
		_, result, err := request(symbol)
		if command.Context().Err() != nil {
			items[i].Status = "cancelled"
			break
		}
		if err != nil {
			failed = true
			failure := c.describe(err)
			// Request/read errors are not Cobra argument errors, even when
			// describe has no more specific classification for them.
			if !failure.api && failure.code == "usage" {
				failure = &cliError{code: "unexpected_response", message: fmt.Sprintf("cannot read backtest response: %v", err)}
			}
			response := failure.response()
			items[i].Status, items[i].Error = "failed", &response
			if !c.json {
				fmt.Fprintf(command.OutOrStdout(), "%s: %s\n", symbol, failure.Error())
			}
			if !failure.api || failure.status == 401 || failure.status == 403 || failure.code == "backtest_busy" || failure.code == "strategy_not_found" || failure.status >= 500 && failure.code != "backtest_too_heavy" {
				break
			}
			continue
		}
		items[i].Status, items[i].Result = "completed", result
		if !c.json {
			renderBacktestSummary(command.OutOrStdout(), *result, target.from != "" || target.to != "")
		}
	}
	if !c.json {
		for _, item := range items {
			if item.Status == "not_run" || item.Status == "cancelled" {
				fmt.Fprintf(command.OutOrStdout(), "%s: %s\n", item.Symbol, item.Status)
			}
		}
	}
	status := "completed"
	if failed {
		status = "failed"
	}
	if command.Context().Err() != nil {
		status = "cancelled"
	}
	if c.json {
		if err := printJSON(command, struct {
			Status string               `json:"status"`
			Items  []backtestCoinResult `json:"items"`
		}{status, items}); err != nil {
			return err
		}
	}
	if command.Context().Err() != nil {
		return command.Context().Err()
	}
	if failed {
		return failure("backtest_failed", "one or more coin backtests failed; see per-coin results")
	}
	return nil
}

func (c *cli) completeBacktestSymbols(command *cobra.Command, args []string, value string) ([]cobra.Completion, cobra.ShellCompDirective) {
	prefix := ""
	if index := strings.LastIndex(value, ","); index >= 0 {
		prefix = value[:index+1]
	}
	values, directive := c.completeFavorites(command, args, value[len(prefix):])
	for i := range values {
		values[i] = prefix + values[i]
	}
	return values, directive
}
