// Package coingecko is the CoinGecko HTTP adapter for market-cap data.
package coingecko

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"crypto-scanner/internal/marketcap"
	"crypto-scanner/internal/platform/numeric"
)

var _ marketcap.Provider = (*Client)(nil)

type wireTicker struct {
	Base         string `json:"base"`
	Target       string `json:"target"`
	CoinID       string `json:"coin_id"`
	TargetCoinID string `json:"target_coin_id"`
	IsStale      bool   `json:"is_stale"`
	IsAnomaly    bool   `json:"is_anomaly"`
}

// Client is the CoinGecko HTTP adapter for marketcap.Provider.
type Client struct {
	baseURL, key string
	http         *http.Client
	keyAllowed   bool
}

// NewClient creates a CoinGecko client. The API key is only sent to official
// CoinGecko HTTPS hosts; an empty base selects the public API.
func NewClient(base, key string) *Client {
	if base == "" {
		base = "https://api.coingecko.com"
	}
	u, _ := url.Parse(base)
	host := ""
	scheme := ""
	if u != nil {
		host = strings.ToLower(u.Hostname())
		scheme = u.Scheme
	}
	allowed := scheme == "https" && (host == "api.coingecko.com" || host == "pro-api.coingecko.com")
	return &Client{baseURL: strings.TrimRight(base, "/"), key: key, keyAllowed: allowed, http: &http.Client{Timeout: 15 * time.Second}}
}
func (c *Client) Tickers(ctx context.Context, page int) ([]marketcap.Ticker, error) {
	var body struct {
		Tickers *json.RawMessage `json:"tickers"`
	}
	err := c.get(ctx, "/api/v3/exchanges/binance/tickers?page="+fmt.Sprint(page)+"&order=base_target", &body)
	if err != nil {
		return nil, err
	}
	if body.Tickers == nil || string(*body.Tickers) == "null" {
		return nil, fmt.Errorf("invalid CoinGecko tickers response")
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(*body.Tickers, &raw); err != nil {
		return nil, fmt.Errorf("invalid CoinGecko tickers response")
	}
	result := make([]marketcap.Ticker, 0, len(raw))
	for _, item := range raw {
		if string(item) == "null" {
			return nil, fmt.Errorf("invalid CoinGecko ticker")
		}
		var wire wireTicker
		if err := json.Unmarshal(item, &wire); err != nil {
			return nil, fmt.Errorf("invalid CoinGecko ticker")
		}
		ticker := marketcap.Ticker(wire)
		if ticker.Base == "" || ticker.Target == "" {
			return nil, fmt.Errorf("invalid CoinGecko ticker")
		}
		result = append(result, ticker)
	}
	return result, nil
}

// StablecoinIDs returns a complete snapshot of CoinGecko IDs in the stablecoins
// category. The category is fetched once per page instead of once per asset.
func (c *Client) StablecoinIDs(ctx context.Context) ([]string, error) {
	const pageSize = 250
	ids := make([]string, 0, pageSize)
	seen := make(map[string]struct{})
	for page := 1; page <= 100; page++ {
		var values *[]struct {
			ID string `json:"id"`
		}
		path := "/api/v3/coins/markets?vs_currency=usd&category=stablecoins&order=market_cap_desc&per_page=250&page=" + fmt.Sprint(page) + "&sparkline=false"
		if err := c.get(ctx, path, &values); err != nil {
			return nil, err
		}
		if values == nil {
			return nil, fmt.Errorf("invalid CoinGecko stablecoin response")
		}
		for _, value := range *values {
			if value.ID == "" {
				return nil, fmt.Errorf("invalid CoinGecko stablecoin response")
			}
			if _, duplicate := seen[value.ID]; duplicate {
				return nil, fmt.Errorf("duplicate CoinGecko stablecoin ID %q", value.ID)
			}
			seen[value.ID] = struct{}{}
			ids = append(ids, value.ID)
		}
		if len(*values) < pageSize {
			if len(ids) == 0 {
				return nil, fmt.Errorf("empty CoinGecko stablecoin response")
			}
			return ids, nil
		}
	}
	return nil, fmt.Errorf("CoinGecko stablecoin response exceeded pagination limit")
}

func (c *Client) Markets(ctx context.Context, ids []string) ([]marketcap.Cap, error) {
	if len(ids) == 0 || len(ids) > 250 {
		return nil, fmt.Errorf("CoinGecko market ID batch must contain 1 to 250 IDs")
	}
	var body *[]struct {
		ID          string    `json:"id"`
		MarketCap   *float64  `json:"market_cap"`
		LastUpdated time.Time `json:"last_updated"`
	}
	err := c.get(ctx, "/api/v3/coins/markets?vs_currency=usd&ids="+url.QueryEscape(strings.Join(ids, ","))+"&per_page="+fmt.Sprint(len(ids))+"&page=1&sparkline=false", &body)
	if err != nil {
		return nil, err
	}
	if body == nil {
		return nil, fmt.Errorf("invalid CoinGecko markets response")
	}
	requested := map[string]bool{}
	for _, id := range ids {
		if id == "" || requested[id] {
			return nil, fmt.Errorf("invalid requested CoinGecko ID")
		}
		requested[id] = true
	}
	seen := map[string]bool{}
	result := make([]marketcap.Cap, 0, len(*body))
	for _, v := range *body {
		if v.ID == "" || !requested[v.ID] || seen[v.ID] {
			continue
		}
		if v.MarketCap == nil || !numeric.Finite(*v.MarketCap) || *v.MarketCap < 0 || v.LastUpdated.IsZero() {
			continue
		}
		seen[v.ID] = true
		result = append(result, marketcap.Cap{CoinID: v.ID, USD: *v.MarketCap, Available: true, ObservedAt: v.LastUpdated})
	}
	for id := range requested {
		if !seen[id] {
			result = append(result, marketcap.Cap{CoinID: id, Reason: "market_cap_missing"})
		}
	}
	return result, nil
}
func (c *Client) get(ctx context.Context, path string, destination any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	if c.key != "" && c.keyAllowed {
		req.Header.Set("x-cg-demo-api-key", c.key)
	}
	response, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode/100 != 2 {
		return fmt.Errorf("CoinGecko status %d", response.StatusCode)
	}
	return json.NewDecoder(response.Body).Decode(destination)
}
