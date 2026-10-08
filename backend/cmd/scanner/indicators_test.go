package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A failed final list must not turn confirmed deletion into a failure.
func TestPruneReportsSuccessfulDeletionWhenFinalListFails(t *testing.T) {
	for _, jsonOutput := range []bool{false, true} {
		t.Run(fmt.Sprint(jsonOutput), func(t *testing.T) {
			pruned := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodDelete {
					pruned = true
					w.WriteHeader(http.StatusNoContent)
				} else if pruned {
					w.WriteHeader(http.StatusServiceUnavailable)
				} else {
					fmt.Fprint(w, `{"items":[]}`)
				}
			}))
			t.Cleanup(server.Close)
			args := []string{"indicators", "prune"}
			if jsonOutput {
				args = append(args, "--json")
			}
			got := runCLI(t, writeProfile(t, server.URL), "", args...)
			if got.code != 0 || !pruned {
				t.Fatalf("prune = %+v", got)
			}
			if jsonOutput {
				var result struct {
					Pruned          bool   `json:"pruned"`
					ObservedRemoved *int   `json:"observed_removed"`
					Warning         string `json:"warning"`
				}
				if err := json.Unmarshal([]byte(got.stdout), &result); err != nil || !result.Pruned || result.ObservedRemoved != nil || !strings.Contains(result.Warning, "pruning succeeded") || got.stderr != "" {
					t.Fatalf("prune JSON = %+v", got)
				}
			} else if !strings.Contains(got.stdout, "removal count unavailable") || !strings.Contains(got.stderr, "pruning succeeded") {
				t.Fatalf("prune text = %+v", got)
			}
		})
	}
}

func TestPruneIndicators(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"indicators", "prune"}, want: "pruning succeeded; observed 1 removals (may include concurrent deletions): h-atr-100\n"},
		{args: []string{"indicators", "prune", "--json"}, want: `{"observed_removed":1,"pruned":true}` + "\n"},
	} {
		pruned := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.Method + " " + r.URL.Path {
			case "GET /api/v1/admin/scanner-indicators":
				unused := `,{"id":2,"title":"h-atr-100"}`
				if pruned {
					unused = ""
				}
				fmt.Fprintf(w, `{"items":[{"id":1,"title":"d-rsi"}%s]}`, unused)
			case "DELETE /api/v1/admin/scanner-indicators":
				pruned = true
				w.WriteHeader(http.StatusNoContent)
			default:
				t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		t.Cleanup(server.Close)

		got := runCLI(t, writeProfile(t, server.URL), "", test.args...)

		if got.code != 0 || got.stdout != test.want || got.stderr != "" || !pruned {
			t.Fatalf("scanner %v = %+v, want stdout %q", test.args, got, test.want)
		}
	}
}
