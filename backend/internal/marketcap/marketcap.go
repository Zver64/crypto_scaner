// Package marketcap obtains and caches CoinGecko market-cap data.
package marketcap

import (
	"context"
	"crypto-scanner/internal/market"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const negativeTTL = 24 * time.Hour
const failureCooldown = time.Second

type Mapping struct {
	BaseAsset, CoinID, QuoteAsset, SourceSymbol, Status, Reason string
	ExpiresAt                                                   *time.Time
}
type Cap struct {
	CoinID                string
	USD                   float64
	Available             bool
	Reason                string
	FetchedAt, ObservedAt time.Time
}
type Store interface {
	BootstrapCompleted(context.Context) (bool, error)
	ReplaceSnapshot(context.Context, []Mapping) error
	GetMapping(context.Context, string) (Mapping, error)
	SaveMapping(context.Context, Mapping) error
	GetCap(context.Context, string) (Cap, error)
	SaveCap(context.Context, Cap) error
}
type Provider interface {
	Tickers(context.Context, int) ([]Ticker, error)
	Markets(context.Context, []string) ([]Cap, error)
}
type Ticker struct {
	Base         string `json:"base"`
	Target       string `json:"target"`
	CoinID       string `json:"coin_id"`
	TargetCoinID string `json:"target_coin_id"`
	IsStale      bool   `json:"is_stale"`
	IsAnomaly    bool   `json:"is_anomaly"`
}
type Fact struct {
	USD    *float64
	Reason string
}
type Batch struct {
	Facts           map[string]Fact
	ProviderWarning bool
}
type Resolver struct {
	store                         Store
	provider                      Provider
	now                           func() time.Time
	scan                          sync.Mutex
	refresh                       sync.Mutex
	scanFailedAt, refreshFailedAt time.Time
	unavailableAt                 map[string]time.Time
	retryDelay                    func(int) time.Duration
}

func New(store Store, provider Provider) *Resolver {
	return &Resolver{store: store, provider: provider, now: time.Now, unavailableAt: map[string]time.Time{}, retryDelay: func(attempt int) time.Duration {
		delay := time.Second * time.Duration(1<<min(attempt, 6))
		if delay > time.Minute {
			return time.Minute
		}
		return delay
	}}
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// BootstrapUntilComplete retries the initial immutable mapping snapshot without blocking HTTP startup.
func (r *Resolver) BootstrapUntilComplete(ctx context.Context) error {
	for attempt := 0; ; attempt++ {
		if err := r.Bootstrap(ctx); err == nil {
			return nil
		}
		delay := r.retryDelay(attempt)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
func (r *Resolver) Bootstrap(ctx context.Context) error {
	done, err := r.store.BootstrapCompleted(ctx)
	if err != nil || done {
		return err
	}
	mappings, err := r.allMappings(ctx)
	if err != nil {
		return err
	}
	return r.store.ReplaceSnapshot(ctx, mappings)
}

// ResolveBatch performs at most one mapping scan and one market request per 250 IDs.
func (r *Resolver) ResolveBatch(ctx context.Context, instruments []market.Instrument) (Batch, error) {
	done, err := r.store.BootstrapCompleted(ctx)
	if err != nil {
		return Batch{}, err
	}
	if !done {
		return Batch{}, fmt.Errorf("%w: mapping bootstrap incomplete", ErrBootstrapIncomplete)
	}
	bases := map[string]market.Instrument{}
	for _, i := range instruments {
		bases[i.BaseAsset] = i
	}
	mappings := map[string]Mapping{}
	missing := map[string]market.Instrument{}
	for base, i := range bases {
		m, e := r.store.GetMapping(ctx, base)
		if e == nil && (m.Status == "resolved" || m.ExpiresAt == nil || m.ExpiresAt.After(r.now())) {
			mappings[base] = m
		} else {
			missing[base] = i
		}
	}
	mappingProviderWarning := false
	if len(missing) > 0 {
		var mappingErr error
		mappingProviderWarning, mappingErr = r.resolveMappings(ctx, missing, mappings)
		if mappingErr != nil {
			return Batch{}, mappingErr
		}
	}
	result := Batch{Facts: map[string]Fact{}}
	r.refresh.Lock()
	defer r.refresh.Unlock()
	cooldown := !r.refreshFailedAt.IsZero() && r.now().Sub(r.refreshFailedAt) < failureCooldown
	caps := map[string]Cap{}
	fetch := map[string]bool{}
	for base, m := range mappings {
		if m.Status != "resolved" {
			result.Facts[base] = Fact{Reason: m.Reason}
			continue
		}
		cap, e := r.store.GetCap(ctx, m.CoinID)
		if e == nil && cap.Available {
			caps[m.CoinID] = cap
			if r.now().Sub(cap.FetchedAt) > time.Hour && !cooldown && (r.unavailableAt[m.CoinID].IsZero() || r.now().Sub(r.unavailableAt[m.CoinID]) >= time.Hour) {
				fetch[m.CoinID] = true
			}
		} else if !cooldown && (r.unavailableAt[m.CoinID].IsZero() || r.now().Sub(r.unavailableAt[m.CoinID]) >= time.Hour) {
			fetch[m.CoinID] = true
		}
	}
	fetched := map[string]Cap{}
	providerFailed := cooldown
	ids := make([]string, 0, len(fetch))
	for id := range fetch {
		ids = append(ids, id)
	}
	for start := 0; start < len(ids); start += 250 {
		end := start + 250
		if end > len(ids) {
			end = len(ids)
		}
		values, e := r.provider.Markets(ctx, ids[start:end])
		if e != nil {
			providerFailed = true
			continue
		}
		for _, cap := range values {
			if cap.Available {
				cap.FetchedAt = r.now()
				if saveErr := r.store.SaveCap(ctx, cap); saveErr != nil {
					providerFailed = true
				}
				delete(r.unavailableAt, cap.CoinID)
				fetched[cap.CoinID] = cap
			} else {
				r.unavailableAt[cap.CoinID] = r.now()
			}
		}
	}
	if providerFailed {
		r.refreshFailedAt = r.now()
	} else {
		r.refreshFailedAt = time.Time{}
	}
	for base, m := range mappings {
		if m.Status != "resolved" {
			continue
		}
		cap, ok := fetched[m.CoinID]
		if !ok {
			cap, ok = caps[m.CoinID]
		}
		if !ok || !cap.Available {
			result.Facts[base] = Fact{Reason: "market_cap_missing"}
			continue
		}
		value := cap.USD
		result.Facts[base] = Fact{USD: &value}
	}
	result.ProviderWarning = providerFailed || mappingProviderWarning
	return result, nil
}

var ErrBootstrapIncomplete = fmt.Errorf("market cap bootstrap incomplete")

func (r *Resolver) resolveMappings(ctx context.Context, missing map[string]market.Instrument, mappings map[string]Mapping) (bool, error) {
	r.scan.Lock()
	defer r.scan.Unlock()
	still := map[string]market.Instrument{}
	for base, i := range missing {
		m, e := r.store.GetMapping(ctx, base)
		if e == nil && (m.Status == "resolved" || m.ExpiresAt == nil || m.ExpiresAt.After(r.now())) {
			mappings[base] = m
		} else {
			still[base] = i
		}
	}
	if len(still) == 0 {
		return false, nil
	}
	if !r.scanFailedAt.IsZero() && r.now().Sub(r.scanFailedAt) < failureCooldown {
		for base, instrument := range still {
			mappings[base] = Mapping{BaseAsset: base, QuoteAsset: instrument.QuoteAsset, SourceSymbol: base + instrument.QuoteAsset, Status: "unresolved", Reason: "mapping_provider_unavailable"}
		}
		return true, nil
	}
	all, err := r.allMappings(ctx)
	if err != nil {
		r.scanFailedAt = r.now()
		for base, instrument := range still {
			unresolved := Mapping{BaseAsset: base, QuoteAsset: instrument.QuoteAsset, SourceSymbol: base + instrument.QuoteAsset, Status: "unresolved", Reason: "mapping_provider_unavailable"}
			mappings[base] = unresolved
		}
		return true, nil
	}
	r.scanFailedAt = time.Time{}
	for base, i := range still {
		var selected Mapping
		for _, m := range all {
			if m.BaseAsset == base && (m.Status != "resolved" || m.QuoteAsset == i.QuoteAsset) {
				selected = m
				break
			}
		}
		if selected.BaseAsset == "" {
			selected = Mapping{BaseAsset: base, QuoteAsset: i.QuoteAsset, SourceSymbol: base + i.QuoteAsset, Status: "unresolved", Reason: "mapping_not_found", ExpiresAt: ptr(r.now().Add(negativeTTL))}
		}
		if err := r.store.SaveMapping(ctx, selected); err != nil {
			return false, err
		}
		mappings[base] = selected
	}
	return false, nil
}
func (r *Resolver) allMappings(ctx context.Context) ([]Mapping, error) {
	byBase := map[string]Mapping{}
	for page := 1; ; page++ {
		tickers, err := r.provider.Tickers(ctx, page)
		if err != nil {
			return nil, err
		}
		for _, t := range tickers {
			if t.Base == "" || t.Target == "" || t.CoinID == "" || t.IsStale || t.IsAnomaly {
				continue
			}
			candidate := Mapping{BaseAsset: t.Base, QuoteAsset: t.Target, SourceSymbol: t.Base + t.Target, CoinID: t.CoinID, Status: "resolved"}
			previous, exists := byBase[t.Base]
			if exists && previous.CoinID != candidate.CoinID {
				byBase[t.Base] = Mapping{BaseAsset: t.Base, QuoteAsset: previous.QuoteAsset, SourceSymbol: previous.SourceSymbol, Status: "unresolved", Reason: "mapping_conflict", ExpiresAt: ptr(r.now().Add(negativeTTL))}
			} else if !exists || (candidate.QuoteAsset == "USDT" && previous.QuoteAsset != "USDT") {
				byBase[t.Base] = candidate
			}
		}
		if len(tickers) < 100 {
			if page == 1 && len(byBase) == 0 {
				return nil, fmt.Errorf("empty CoinGecko ticker snapshot")
			}
			all := make([]Mapping, 0, len(byBase))
			for _, m := range byBase {
				all = append(all, m)
			}
			return all, nil
		}
	}
}
func ptr(t time.Time) *time.Time { return &t }

type Client struct {
	baseURL, key string
	http         *http.Client
	keyAllowed   bool
}

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
func (c *Client) Tickers(ctx context.Context, page int) ([]Ticker, error) {
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
	result := make([]Ticker, 0, len(raw))
	for _, item := range raw {
		if string(item) == "null" {
			return nil, fmt.Errorf("invalid CoinGecko ticker")
		}
		var ticker Ticker
		if err := json.Unmarshal(item, &ticker); err != nil {
			return nil, fmt.Errorf("invalid CoinGecko ticker")
		}
		if ticker.Base == "" || ticker.Target == "" {
			return nil, fmt.Errorf("invalid CoinGecko ticker")
		}
		result = append(result, ticker)
	}
	return result, nil
}
func (c *Client) Markets(ctx context.Context, ids []string) ([]Cap, error) {
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
	invalid := map[string]bool{}
	result := make([]Cap, 0, len(*body))
	for _, v := range *body {
		if v.ID == "" || !requested[v.ID] || seen[v.ID] {
			continue
		}
		if v.MarketCap == nil || math.IsNaN(*v.MarketCap) || math.IsInf(*v.MarketCap, 0) || *v.MarketCap < 0 || v.LastUpdated.IsZero() {
			invalid[v.ID] = true
			continue
		}
		seen[v.ID] = true
		result = append(result, Cap{CoinID: v.ID, USD: *v.MarketCap, Available: true, ObservedAt: v.LastUpdated})
	}
	for id := range requested {
		if !seen[id] {
			reason := "market_cap_missing"
			if invalid[id] {
				reason = "market_cap_missing"
			}
			result = append(result, Cap{CoinID: id, Reason: reason})
		}
	}
	return result, err
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
