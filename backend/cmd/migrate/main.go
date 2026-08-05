// Command migrate applies explicit PostgreSQL schema migrations.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"crypto-scanner/internal/migrate"
	"crypto-scanner/internal/platform/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := migrate.Run(ctx, os.Args[1:], config.LoadDatabaseURL); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
