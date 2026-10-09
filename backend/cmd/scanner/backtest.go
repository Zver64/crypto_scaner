package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"crypto-scanner/internal/apiclient"

	"github.com/gofrs/flock"
	"github.com/spf13/cobra"
)

func (c *cli) backtestCommand() *cobra.Command {
	var strategy string
	var target backtestFlags
	command := &cobra.Command{
		Use:   "backtest --strategy ID --symbol SYM [--from TIME] [--to TIME]",
		Short: "Simulate the trades of a saved strategy on one coin against its baselines",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			id, err := strategyID(strategy)
			if err != nil {
				return err
			}
			params := apiclient.BacktestStrategyParams{Symbol: target.symbol}
			if params.From, params.To, err = target.period(); err != nil {
				return err
			}
			client, err := c.client()
			if err != nil {
				return err
			}
			unlock, err := lockBacktest()
			if err != nil {
				return err
			}
			defer unlock()
			backtest, err := client.BacktestStrategyWithResponse(command.Context(), id, &params)
			if err == nil {
				err = c.check(backtest, backtest.JSON200 != nil, backtest.JSON400, backtest.JSON404, backtest.JSON429, backtest.JSON503)
			}
			if err != nil {
				return err
			}
			c.printBacktest(command, backtest.Body, *backtest.JSON200, target)
			return nil
		},
	}
	command.Flags().StringVar(&strategy, "strategy", "", "the saved strategy `ID` (see scanner strategies)")
	_ = command.MarkFlagRequired("strategy")
	_ = command.RegisterFlagCompletionFunc("strategy", c.completeStrategies)
	target.add(c, command)
	return command
}

// backtestStrategyDraftCommand backtests strategy rules without saving them.
func (c *cli) backtestStrategyDraftCommand() *cobra.Command {
	var rules strategyRuleFlags
	var target backtestFlags
	command := &cobra.Command{
		Use:   "backtest --expr EXPR --symbol SYM [--direction long|short] [--exit EXPR] [--take-profit EXPR] [--stop-loss EXPR] [--from TIME] [--to TIME]",
		Short: "Simulate the trades of an unsaved strategy on one coin, like scanner backtest a saved one",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			body := apiclient.StrategyBacktestInput{
				Expression: rules.expression, ExitExpression: rules.exit, TakeProfitExpression: rules.takeProfit, StopLossExpression: rules.stopLoss, Symbol: target.symbol,
			}
			var err error
			if body.Direction, err = strategiesKind.directionFlag(rules.direction); err != nil {
				return err
			}
			return c.backtestDraft(command, body, rules.rules(), target)
		},
	}
	rules.add(command)
	target.add(c, command)
	return command
}

// backtestSignalDraftCommand backtests a signal rule without saving it.
func (c *cli) backtestSignalDraftCommand() *cobra.Command {
	var rules signalRuleFlags
	var target backtestFlags
	command := &cobra.Command{
		Use:   "backtest --expr EXPR --direction long|short|sideways --symbol SYM [--target-ratio STOPS] [--window CANDLES] [--from TIME] [--to TIME]",
		Short: "Judge the signals of an unsaved signal on one coin, like scanner backtest a saved one",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			move, err := rules.parse(command)
			if err != nil {
				return err
			}
			return c.backtestDraft(command, apiclient.StrategyBacktestInput{
				Signal: true, Direction: move, Expression: rules.expression, Symbol: target.symbol,
				TargetRatio: new(apiclient.SignalTargetRatio(rules.settings.ratio)), Window: new(apiclient.SignalWindow(rules.settings.window)),
			}, rules.rules(), target)
		},
	}
	rules.add(command)
	target.add(c, command)
	return command
}

// backtestDraft validates rules, reporting every invalid one at once, then
// sends body with the period of target and prints the result.
func (c *cli) backtestDraft(command *cobra.Command, body apiclient.StrategyBacktestInput, rules []strategyRule, target backtestFlags) error {
	var err error
	if body.From, body.To, err = target.period(); err != nil {
		return err
	}
	client, err := c.client()
	if err != nil {
		return err
	}
	if _, err := c.prepareStrategyRules(command, client, rules, false); err != nil {
		return err
	}
	unlock, err := lockBacktest()
	if err != nil {
		return err
	}
	defer unlock()
	backtest, err := client.BacktestStrategyDraftWithResponse(command.Context(), body)
	if err == nil {
		err = c.check(backtest, backtest.JSON200 != nil, backtest.JSON400, backtest.JSON404, backtest.JSON429, backtest.JSON503)
	}
	if err != nil {
		return err
	}
	c.printBacktest(command, backtest.Body, *backtest.JSON200, target)
	return nil
}

func (c *cli) printBacktest(command *cobra.Command, raw []byte, backtest apiclient.StrategyBacktest, target backtestFlags) {
	period := target.from != "" || target.to != ""
	c.print(command, raw, func(w io.Writer) { renderBacktest(w, backtest, period) })
}

// backtestFlags are the coin and the optional period of a backtest.
type backtestFlags struct{ symbol, from, to string }

func (f *backtestFlags) add(c *cli, command *cobra.Command) {
	flags := command.Flags()
	flags.StringVar(&f.symbol, "symbol", "", "any coin `SYM`, such as BTCUSDT")
	flags.StringVar(&f.from, "from", "", "evaluate candles opening at or after `TIME`: a UTC date such as 2026-01-31, or RFC 3339")
	flags.StringVar(&f.to, "to", "", "evaluate candles opening at or before `TIME`: a UTC date, the whole day included, or RFC 3339")
	_ = command.MarkFlagRequired("symbol")
	_ = command.RegisterFlagCompletionFunc("symbol", c.completeFavorites)
}

// period parses --from and --to, nil when absent.
func (f backtestFlags) period() (from, to *time.Time, err error) {
	for _, bound := range []struct {
		flag, value string
		end         bool
		target      **time.Time
	}{{"from", f.from, false, &from}, {"to", f.to, true, &to}} {
		if bound.value == "" {
			continue
		}
		parsed, err := parseTime(bound.value, bound.end)
		if err != nil {
			return nil, nil, usageError("--%s: %v", bound.flag, err)
		}
		*bound.target = &parsed
	}
	return from, to, nil
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

// lockBacktest takes the lock that lets one backtest at a time run on this
// machine, whatever its profile, beside the configuration file, failing at
// once while another holds it. The returned function releases it; the
// operating system releases it too when the process ends.
func lockBacktest() (func(), error) {
	path, err := configPath()
	if err != nil {
		return nil, failure("config", "%v", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, failure("config", "create %s: %v", dir, err)
	}
	lock := flock.New(filepath.Join(dir, "backtest.lock"))
	locked, err := lock.TryLock()
	if err != nil {
		return nil, failure("config", "lock %s: %v", lock.Path(), err)
	}
	if !locked {
		return nil, failure("backtest_running", "another backtest runs on this machine; retry once it ends")
	}
	return func() { _ = lock.Unlock() }, nil
}
