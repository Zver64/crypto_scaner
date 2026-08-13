// Command bootstrap-admin explicitly creates or re-enables one administrator.
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

const bootstrapAdministrator = `
INSERT INTO app.users (telegram_id, is_enabled)
VALUES ($1, TRUE)
ON CONFLICT (telegram_id) DO UPDATE
SET is_enabled = TRUE,
    updated_at = now()
WHERE app.users.is_enabled = FALSE
`

func main() {
	if err := envfile.LoadRoot(); err != nil {
		fail(err)
	}
	if len(os.Args) != 1 {
		fail(fmt.Errorf("usage: bootstrap-admin"))
	}
	cfg, err := config.LoadBootstrap()
	if err != nil {
		fail(fmt.Errorf("load configuration: %w", err))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	database, err := postgres.OpenVerified(ctx, cfg.DatabaseURL)
	if err != nil {
		fail(fmt.Errorf("initialize PostgreSQL: %w", err))
	}
	defer database.Close()
	if _, err := database.Exec(ctx, bootstrapAdministrator, cfg.AdminTelegramID); err != nil {
		fail(fmt.Errorf("bootstrap administrator: %w", err))
	}
}

func fail(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
