// Command migrate applies explicit PostgreSQL schema migrations.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"crypto-scanner/internal/platform/config"
	"crypto-scanner/internal/platform/envfile"
	"crypto-scanner/internal/storage/postgres"
)

func main() {
	if err := envfile.LoadRoot(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run applies the migration direction args name, up or down, to the
// configured database.
func run(ctx context.Context, args []string) error {
	var direction postgres.Direction
	switch {
	case len(args) == 1 && args[0] == "up":
		direction = postgres.DirectionUp
	case len(args) == 1 && args[0] == "down":
		direction = postgres.DirectionDown
	default:
		return fmt.Errorf("usage: migrate <up|down>")
	}
	databaseURL, err := config.LoadDatabaseURL()
	if err != nil {
		return err
	}
	if err := postgres.Migrate(ctx, databaseURL, direction); err != nil {
		return fmt.Errorf("migrate %s: %w", direction, err)
	}
	return nil
}
