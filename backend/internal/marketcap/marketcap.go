// Package marketcap obtains and caches CoinGecko market-cap data.
package marketcap

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"crypto-scanner/internal/market"
)

const (
	negativeTTL     = 24 * time.Hour
	failureCooldown = time.Second
	// mappingRescanInterval spaces full ticker scans, which page through every
	// Binance ticker, however many new or expired bases appear in between.
	mappingRescanInterval = 6 * time.Hour
	// mappingScanRetryDelay spaces scan attempts after a provider failure.
	mappingScanRetryDelay = 10 * time.Minute
)

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
	// ListMappings returns the persisted mappings of bases; absent ones are omitted.
	ListMappings(context.Context, []string) (map[string]Mapping, error)
	SaveMappings(context.Context, []Mapping) error
	// ListCaps returns the persisted caps of coin IDs; absent ones are omitted.
	ListCaps(context.Context, []string) (map[string]Cap, error)
	SaveCaps(context.Context, []Cap) error
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
	store                   Store
	provider                Provider
	now                     func() time.Time
	scan                    sync.Mutex
	refresh                 sync.Mutex
	scannedAt, scanFailedAt time.Time
	refreshFailedAt         time.Time
	unavailableAt           map[string]time.Time
}

func New(store Store, provider Provider) *Resolver {
	return &Resolver{store: store, provider: provider, now: time.Now, unavailableAt: map[string]time.Time{}}
}

func (r *Resolver) Bootstrap(ctx context.Context) error {
	done, err := r.store.BootstrapCompleted(ctx)
	if err != nil || done {
		return err
	}
	r.scan.Lock()
	defer r.scan.Unlock()
	mappings, err := r.allMappings(ctx)
	if err != nil {
		return err
	}
	if err := r.store.ReplaceSnapshot(ctx, mappings); err != nil {
		return err
	}
	r.scannedAt = r.now()
	return nil
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
		return Batch{}, errors.New("market cap mapping bootstrap incomplete")
	}
	bases := map[string]market.Instrument{}
	for _, i := range instruments {
		bases[i.BaseAsset] = i
	}
	mappings := map[string]Mapping{}
	missing, err := r.cachedMappings(ctx, bases, mappings)
	if err != nil {
		return Batch{}, err
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
	coinIDs := make([]string, 0, len(mappings))
	for _, m := range mappings {
		if m.Status == "resolved" {
			coinIDs = append(coinIDs, m.CoinID)
		}
	}
	caps, err := r.store.ListCaps(ctx, coinIDs)
	if err != nil {
		return Batch{}, err
	}
	fetch := map[string]bool{}
	for base, m := range mappings {
		if m.Status != "resolved" {
			result.Facts[base] = Fact{Reason: m.Reason}
			continue
		}
		if cap, ok := caps[m.CoinID]; ok && cap.Available {
			if r.now().Sub(cap.FetchedAt) > time.Hour && !cooldown && (r.unavailableAt[m.CoinID].IsZero() || r.now().Sub(r.unavailableAt[m.CoinID]) >= time.Hour) {
				fetch[m.CoinID] = true
			}
		} else if !cooldown && (r.unavailableAt[m.CoinID].IsZero() || r.now().Sub(r.unavailableAt[m.CoinID]) >= time.Hour) {
			fetch[m.CoinID] = true
		}
	}
	fetched := map[string]Cap{}
	var available []Cap
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
				available = append(available, cap)
				delete(r.unavailableAt, cap.CoinID)
				fetched[cap.CoinID] = cap
			} else {
				r.unavailableAt[cap.CoinID] = r.now()
			}
		}
	}
	if len(available) > 0 {
		if err := r.store.SaveCaps(ctx, available); err != nil {
			providerFailed = true
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

// cachedMappings adds the valid persisted mappings of bases to mappings and
// returns the bases that still need a scan.
func (r *Resolver) cachedMappings(ctx context.Context, bases map[string]market.Instrument, mappings map[string]Mapping) (map[string]market.Instrument, error) {
	keys := make([]string, 0, len(bases))
	for base := range bases {
		keys = append(keys, base)
	}
	stored, err := r.store.ListMappings(ctx, keys)
	if err != nil {
		return nil, err
	}
	missing := map[string]market.Instrument{}
	for base, i := range bases {
		if m, ok := stored[base]; ok && r.mappingCacheValid(m) {
			mappings[base] = m
		} else {
			missing[base] = i
		}
	}
	return missing, nil
}

func (r *Resolver) resolveMappings(ctx context.Context, missing map[string]market.Instrument, mappings map[string]Mapping) (bool, error) {
	r.scan.Lock()
	defer r.scan.Unlock()
	// Another batch may have scanned while this one waited for the lock.
	still, err := r.cachedMappings(ctx, missing, mappings)
	if err != nil {
		return false, err
	}
	if len(still) == 0 {
		return false, nil
	}
	if !r.scannedAt.IsZero() && r.now().Sub(r.scannedAt) < mappingRescanInterval {
		// New or expired bases wait for the next scheduled scan.
		for base, instrument := range still {
			mappings[base] = Mapping{BaseAsset: base, QuoteAsset: instrument.QuoteAsset, SourceSymbol: base + instrument.QuoteAsset, Status: "unresolved", Reason: "mapping_not_found"}
		}
		return false, nil
	}
	if !r.scanFailedAt.IsZero() && r.now().Sub(r.scanFailedAt) < mappingScanRetryDelay {
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
	r.scannedAt = r.now()
	selections := make([]Mapping, 0, len(still))
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
		selections = append(selections, selected)
		mappings[base] = selected
	}
	return false, r.store.SaveMappings(ctx, selections)
}
func (r *Resolver) mappingCacheValid(mapping Mapping) bool {
	return mapping.Status == "resolved" || mapping.ExpiresAt == nil || mapping.ExpiresAt.After(r.now())
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
