package postgres

import (
	"strings"
	"testing"
)

func TestValidateLegacyMigrationVersion(t *testing.T) {
	for _, version := range []int64{1, 2, 3} {
		if err := validateLegacyMigrationVersion(version); err != nil {
			t.Fatalf("validateLegacyMigrationVersion(%d) error = %v", version, err)
		}
	}
	for _, version := range []int64{0, -1, 4} {
		err := validateLegacyMigrationVersion(version)
		if err == nil || !strings.Contains(err.Error(), "legacy migration version") {
			t.Errorf("validateLegacyMigrationVersion(%d) error = %v, want clear rejection", version, err)
		}
	}
}
