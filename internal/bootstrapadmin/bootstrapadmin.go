// Package bootstrapadmin implements the standalone administrator bootstrap.
package bootstrapadmin

import (
	"context"
	"fmt"

	"crypto-scanner/internal/platform/config"
	"crypto-scanner/internal/storage/postgres"
)

// ConfigLoader loads the settings owned by the configuration boundary.
type ConfigLoader func() (config.AdminBootstrapConfig, error)

// Run loads command configuration, connects to a verified PostgreSQL schema,
// and creates or re-enables the configured administrator.
func Run(ctx context.Context, loadConfig ConfigLoader) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load administrator bootstrap configuration: %w", err)
	}
	database, err := postgres.OpenVerified(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("initialize PostgreSQL: %w", err)
	}
	defer database.Close()

	if err := postgres.NewStore(database).BootstrapAdministrator(ctx, cfg.AdminTelegramID); err != nil {
		return err
	}
	return nil
}
