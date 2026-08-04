package migrate_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"crypto-scanner/internal/migrate"
)

func TestCommandRequiresOneSupportedDirection(t *testing.T) {
	tests := [][]string{nil, {}, {"sideways"}, {"up", "down"}}
	for _, args := range tests {
		err := migrate.Run(context.Background(), args, func() (string, error) { return "postgres://localhost/test", nil })
		if err == nil || !strings.Contains(err.Error(), "usage: migrate <up|down>") {
			t.Errorf("Run(%q) error = %v, want usage", args, err)
		}
	}
}

func TestCommandRequiresDatabaseURLWithoutEchoingCredentials(t *testing.T) {
	err := migrate.Run(context.Background(), []string{"up"}, func() (string, error) {
		return "", fmt.Errorf("DATABASE_URL is required")
	})
	if err == nil || err.Error() != "DATABASE_URL is required" {
		t.Fatalf("Run() error = %v, want missing DATABASE_URL", err)
	}
}
