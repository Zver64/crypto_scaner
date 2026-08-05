// Command bootstrap-admin explicitly creates or re-enables the configured
// Telegram administrator.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"crypto-scanner/internal/bootstrapadmin"
	"crypto-scanner/internal/platform/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := bootstrapadmin.Run(ctx, config.LoadAdminBootstrap); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
