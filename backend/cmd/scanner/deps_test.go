package main

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// The CLI is installed with go install on machines without a C compiler, so
// none of its non-standard dependencies may use cgo (TA-Lib does), and it
// reaches the server only through the generated API client.
func TestCLIDependsOnlyOnTheAPIClientAndNoCgo(t *testing.T) {
	command := exec.Command("go", "list", "-deps", "-f", "{{if not .Standard}}{{.ImportPath}} {{len .CgoFiles}}{{end}}", ".")
	command.Env = append(command.Environ(), "CGO_ENABLED=1")
	output, err := command.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	allowed := []string{"crypto-scanner/cmd/scanner", "crypto-scanner/internal/apiclient"}
	for line := range strings.Lines(string(output)) {
		path, cgoFiles, _ := strings.Cut(strings.TrimSpace(line), " ")
		if cgoFiles != "0" || strings.HasPrefix(path, "crypto-scanner/") && !slices.Contains(allowed, path) {
			t.Errorf("the CLI depends on %s (%s cgo files)", path, cgoFiles)
		}
	}
}
