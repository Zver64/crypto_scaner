// Package envfile loads the repository's optional root environment file.
package envfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// LoadRoot loads the root .env when the command is run from this repository.
// Existing process environment variables take precedence. A missing file is
// valid so deployed binaries can rely entirely on their process environment.
func LoadRoot() error {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("find working directory for root .env: %w", err)
	}

	root, found := findRoot(workingDirectory)
	if !found {
		return nil
	}
	path := filepath.Join(root, ".env")
	if err := godotenv.Load(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("load root .env: %w", err)
	}
	return nil
}

func findRoot(start string) (string, bool) {
	for directory := filepath.Clean(start); ; directory = filepath.Dir(directory) {
		if regularFile(filepath.Join(directory, "backend", "go.mod")) {
			return directory, true
		}
		if filepath.Base(directory) == "backend" && regularFile(filepath.Join(directory, "go.mod")) {
			return filepath.Dir(directory), true
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", false
		}
	}
}

func regularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
