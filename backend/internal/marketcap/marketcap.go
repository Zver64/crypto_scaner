// Package marketcap obtains and caches CoinGecko market-cap data.
package marketcap

import (
	"context"
	"fmt"
	"sync"
	"time"

	"crypto-scanner/internal/market"
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
	ReplaceStablecoinClassifications(context.Context, []string) error
	GetMapping(context.Context, string) (Mapping, error)
	SaveMapping(context.Context, Mapping) error
	GetCap(context.Context, string) (Cap, error)
	SaveCap(context.Context, Cap) error
}
type Provider interface {
	Tickers(context.Context, int) ([]Ticker, error)
	Markets(context.Context, []string) ([]Cap, error)
	StablecoinIDs(context.Context) ([]string, error)
}

// Ticker is one exchange pair as reported by the market-cap provider.
type Ticker struct {
	Base         string
	Target       string
	CoinID       string
	TargetCoinID string
	IsStale      bool
	IsAnomaly    bool
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
}

func New(store Store, provider Provider) *Resolver {
	return &Resolver{store: store, provider: provider, now: time.Now, unavailableAt: map[string]time.Time{}}
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

// RefreshStablecoinClassifications fetches one complete stablecoin category
// snapshot and classifies every persisted CoinGecko mapping from that snapshot.
func (r *Resolver) RefreshStablecoinClassifications(ctx context.Context) error {
	ids, err := r.provider.StablecoinIDs(ctx)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return fmt.Errorf("empty CoinGecko stablecoin snapshot")
	}
	return r.store.ReplaceStablecoinClassifications(ctx, ids)
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
		if r.mappingCacheValid(m, e) {
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
		if r.mappingCacheValid(m, e) {
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
func (r *Resolver) mappingCacheValid(mapping Mapping, err error) bool {
	return err == nil && (mapping.Status == "resolved" || mapping.ExpiresAt == nil || mapping.ExpiresAt.After(r.now()))
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
