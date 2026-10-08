package main

import (
	"net/http"
	"slices"
	"strings"

	"crypto-scanner/internal/apiclient"

	"github.com/spf13/cobra"
)

func (c *cli) indicatorsCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "indicators",
		Short: "Manage configured scanner indicators",
		// Runnable, so cobra reports an unknown subcommand as an error.
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error { return command.Help() },
	}
	command.AddCommand(c.pruneIndicatorsCommand())
	return command
}

func (c *cli) pruneIndicatorsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "prune",
		Short: "Delete unused indicators and report the observed removals",
		Long: `Delete every scanner indicator that no saved strategy reads, including
disabled strategies, and that no market table or chart shows. Deletes without
confirmation, then lists the observed removals; --json prints
{"pruned": true, "observed_removed": N}. The API returns no count, so the CLI
compares the indicators before and after. Concurrent deletions may be included.
If the final list fails, pruning still succeeds, but observed_removed is null
and warning explains why the removals could not be counted.`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, err := c.client()
			if err != nil {
				return err
			}
			before, err := c.indicators(command, client)
			if err != nil {
				return err
			}
			deleted, err := client.DeleteUnusedScannerIndicatorsWithResponse(command.Context())
			if err == nil {
				err = c.check(deleted, deleted.StatusCode() == http.StatusNoContent, deleted.JSON409)
			}
			if err != nil {
				return err
			}
			after, err := c.indicators(command, client)
			if err != nil {
				warning := "pruning succeeded, but could not list the remaining indicators: " + c.describe(err).Error()
				if c.json {
					return printJSON(command, map[string]any{"pruned": true, "observed_removed": nil, "warning": warning})
				}
				command.Println("pruned scanner indicators used by no strategy, table or chart; removal count unavailable")
				command.PrintErrln("scanner: " + warning)
				return nil
			}
			removed := slices.DeleteFunc(before, func(indicator apiclient.ScannerIndicator) bool {
				return slices.ContainsFunc(after, func(kept apiclient.ScannerIndicator) bool { return kept.Id == indicator.Id })
			})
			if c.json {
				return printJSON(command, map[string]any{"pruned": true, "observed_removed": len(removed)})
			}
			titles := make([]string, len(removed))
			for i, indicator := range removed {
				titles[i] = indicator.Title
			}
			if len(titles) == 0 {
				command.Println("pruning succeeded; no removals observed")
				return nil
			}
			command.Printf("pruning succeeded; observed %d removals (may include concurrent deletions): %s\n", len(titles), strings.Join(titles, ", "))
			return nil
		},
	}
}

// indicators lists the configured scanner indicators.
func (c *cli) indicators(command *cobra.Command, client *apiclient.ClientWithResponses) ([]apiclient.ScannerIndicator, error) {
	listed, err := client.ListScannerIndicatorsWithResponse(command.Context())
	if err == nil {
		err = c.check(listed, listed.JSON200 != nil)
	}
	if err != nil {
		return nil, err
	}
	return listed.JSON200.Items, nil
}
