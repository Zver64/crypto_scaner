package main

import (
	"net/http"

	"github.com/spf13/cobra"
)

func (c *cli) indicatorsCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "indicators",
		Short: "Manage configured scanner indicators",
		Args:  cobra.NoArgs,
	}
	command.AddCommand(c.pruneIndicatorsCommand())
	return command
}

func (c *cli) pruneIndicatorsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "prune",
		Short: "Delete unused indicators (--json produces no output on success)",
		Long: `Delete every scanner indicator that no saved strategy reads, including
disabled strategies, and that no market table or chart shows. Deletes without
confirmation.
With --json, successful deletion produces no output (HTTP 204 has no body).`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, err := c.client()
			if err != nil {
				return err
			}
			deleted, err := client.DeleteUnusedScannerIndicatorsWithResponse(command.Context())
			if err == nil {
				err = check(deleted, deleted.StatusCode() == http.StatusNoContent, deleted.JSON409)
			}
			if err != nil {
				return err
			}
			if !c.json {
				command.Println("pruned scanner indicators used by no strategy, table or chart")
			}
			return nil
		},
	}
}
