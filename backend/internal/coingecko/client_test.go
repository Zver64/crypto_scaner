package coingecko

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientStablecoinIDsFetchesCategoryByPageInsteadOfAsset(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		query := r.URL.Query()
		if query.Get("category") != "stablecoins" || query.Get("per_page") != "250" || query.Get("page") != fmt.Sprint(calls) {
			t.Errorf("query=%s", r.URL.RawQuery)
		}
		values := make([]map[string]string, 250)
		if calls == 1 {
			for index := range values {
				values[index] = map[string]string{"id": fmt.Sprintf("stable-%d", index)}
			}
		} else {
			values = []map[string]string{{"id": "usds"}}
		}
		if err := json.NewEncoder(w).Encode(values); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	ids, err := NewClient(server.URL, "").StablecoinIDs(context.Background())
	if err != nil || len(ids) != 251 || ids[250] != "usds" || calls != 2 {
		t.Fatalf("IDs=%v calls=%d err=%v", ids, calls, err)
	}
}

func TestClientMarketsSendsCompletePaginationAndRejectsPartialNullableResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("per_page") != "250" || q.Get("page") != "1" || q.Get("sparkline") != "false" {
			t.Errorf("query=%s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`[{"id":"id0","market_cap":null,"last_updated":"2026-01-01T00:00:00Z"}]`))
	}))
	defer server.Close()
	ids := make([]string, 250)
	for i := range ids {
		ids[i] = fmt.Sprintf("id%d", i)
	}
	values, err := NewClient(server.URL, "secret").Markets(context.Background(), ids)
	if err != nil || len(values) != 250 || values[0].Available {
		t.Fatalf("values=%d first=%+v err=%v", len(values), values[0], err)
	}
}
func TestClientMarketsRejectsNullResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`null`))
	}))
	defer server.Close()
	if _, err := NewClient(server.URL, "").Markets(context.Background(), []string{"bitcoin"}); err == nil {
		t.Fatal("Markets() accepted a null response")
	}
}
