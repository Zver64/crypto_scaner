// Package migrate implements the standalone database migration command.
package migrate

import (
	"context"
	"fmt"

	"crypto-scanner/internal/storage/postgres"
)

// Run validates command input, loads its database setting through the config
// boundary, and applies the requested migration direction.
func Run(ctx context.Context, args []string, loadDatabaseURL func() (string, error)) error {
	direction, err := parseDirection(args)
	if err != nil {
		return fmt.Errorf("usage: migrate <up|down>")
	}
	databaseURL, err := loadDatabaseURL()
	if err != nil {
		return err
	}
	if err := postgres.Migrate(ctx, databaseURL, direction); err != nil {
		return fmt.Errorf("migrate %s: %w", direction, err)
	}
	return nil
}

func parseDirection(args []string) (postgres.Direction, error) {
	if len(args) != 1 {
		return 0, fmt.Errorf("one direction is required")
	}
	switch args[0] {
	case "up":
		return postgres.DirectionUp, nil
	case "down":
		return postgres.DirectionDown, nil
	default:
		return 0, fmt.Errorf("unsupported direction")
	}
}
