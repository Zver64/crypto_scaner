package main

import (
	"net/http"
	"testing"
)

func TestPruneIndicators(t *testing.T) {
	api, server := newFakeAPI(t, map[string]string{"DELETE /api/v1/admin/scanner-indicators": ""})
	api.status = http.StatusNoContent
	home := writeProfile(t, server.URL)

	for _, jsonOutput := range []bool{false, true} {
		clear(api.bodies)
		args := []string{"indicators", "prune"}
		want := "pruned scanner indicators used by no strategy, table or chart\n"
		if jsonOutput {
			args = append(args, "--json")
			want = ""
		}
		got := runCLI(t, home, "", args...)
		if got.code != 0 || got.stdout != want || got.stderr != "" {
			t.Fatalf("prune = %+v, want stdout %q", got, want)
		}
		if body, ok := api.bodies["DELETE /api/v1/admin/scanner-indicators"]; !ok || body != "" || len(api.bodies) != 1 {
			t.Fatalf("prune requests = %v", api.bodies)
		}
	}
}
