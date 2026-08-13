package envfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRootLoadsRepositoryEnvironmentWithoutOverridingProcess(t *testing.T) {
	root := t.TempDir()
	backend := filepath.Join(root, "backend")
	if err := os.Mkdir(backend, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backend, "go.mod"), []byte("module example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("BASE_VALUE=loaded\nFROM_FILE=${BASE_VALUE}\nPROCESS_VALUE=from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	withWorkingDirectory(t, backend)
	t.Setenv("FROM_FILE", "")
	if err := os.Unsetenv("FROM_FILE"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PROCESS_VALUE", "from-process")

	if err := LoadRoot(); err != nil {
		t.Fatalf("LoadRoot() error = %v", err)
	}
	if got := os.Getenv("FROM_FILE"); got != "loaded" {
		t.Fatalf("expanded FROM_FILE = %q, want loaded", got)
	}
	if got := os.Getenv("PROCESS_VALUE"); got != "from-process" {
		t.Fatalf("PROCESS_VALUE = %q, want process precedence", got)
	}
	t.Cleanup(func() {
		_ = os.Unsetenv("BASE_VALUE")
		_ = os.Unsetenv("FROM_FILE")
	})
}

func TestLoadRootAllowsMissingFile(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "backend"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "backend", "go.mod"), []byte("module example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	withWorkingDirectory(t, root)

	if err := LoadRoot(); err != nil {
		t.Fatalf("LoadRoot() error = %v", err)
	}
}

func TestLoadRootRejectsInvalidFile(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "backend"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "backend", "go.mod"), []byte("module example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("BROKEN='unterminated\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	withWorkingDirectory(t, root)

	if err := LoadRoot(); err == nil {
		t.Fatal("LoadRoot() error = nil, want invalid file error")
	}
}

func withWorkingDirectory(t *testing.T, directory string) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(directory); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
}
