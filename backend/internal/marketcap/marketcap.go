// Package marketcap obtains and caches CoinGecko market-cap data.
package marketcap

import (
	"context"
	"errors"
	"fmt"
	"time"

	"crypto-scanner/internal/market"
)

const (
	negativeTTL = 24 * time.Hour
	// resolvedMappingTTL bounds how long a resolved mapping is trusted before a
	// scan re-resolves it, so retired CoinGecko IDs and reused tickers are remapped.
	resolvedMappingTTL = 7 * 24 * time.Hour
	failureCooldown    = time.Second
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
	// DeleteCaps removes the persisted caps of coin IDs the provider no longer reports.
	DeleteCaps(context.Context, []string) error
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

// Batch is the outcome of a market-cap refresh that completed with what the
// provider gave.
type Batch struct {
	// ProviderErr joins the provider failures, and the failures to persist
	// their results, that left some caps or mappings stale; nil when none did.
	ProviderErr error
}

// Resolver is not safe for concurrent use: CoinMetadataSynchronizer is its
// only caller.
type Resolver struct {
	store                   Store
	provider                Provider
	now                     func() time.Time
	scannedAt, scanFailedAt time.Time
	// scanErr is the failure of the latest scan, kept while its retry waits.
	scanErr         error
	refreshFailedAt time.Time
	unavailableAt   map[string]time.Time
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
	var providerErrs []error
	if len(missing) > 0 {
		scanErr, err := r.resolveMappings(ctx, missing, mappings)
		if err != nil {
			return Batch{}, err
		}
		if scanErr != nil {
			providerErrs = append(providerErrs, scanErr)
		}
	}
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
	for _, m := range mappings {
		if m.Status != "resolved" {
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
	var available []Cap
	var unavailable []string
	refreshErrs := []error{}
	if cooldown {
		refreshErrs = append(refreshErrs, fmt.Errorf("market caps were not fetched within %s of the last failure", failureCooldown))
	}
	ids := make([]string, 0, len(fetch))
	for id := range fetch {
		ids = append(ids, id)
	}
	for start := 0; start < len(ids); start += 250 {
		end := start + 250
		if end > len(ids) {
			end = len(ids)
		}
		values, err := r.provider.Markets(ctx, ids[start:end])
		if err != nil {
			refreshErrs = append(refreshErrs, fmt.Errorf("fetch market caps: %w", err))
			continue
		}
		for _, cap := range values {
			if cap.Available {
				cap.FetchedAt = r.now()
				available = append(available, cap)
				delete(r.unavailableAt, cap.CoinID)
			} else {
				r.unavailableAt[cap.CoinID] = r.now()
				unavailable = append(unavailable, cap.CoinID)
			}
		}
	}
	if len(available) > 0 {
		if err := r.store.SaveCaps(ctx, available); err != nil {
			refreshErrs = append(refreshErrs, fmt.Errorf("save market caps: %w", err))
		}
	}
	// A cap the provider no longer reports is unknown, not its last value.
	if len(unavailable) > 0 {
		if err := r.store.DeleteCaps(ctx, unavailable); err != nil {
			refreshErrs = append(refreshErrs, fmt.Errorf("delete market caps: %w", err))
		}
	}
	if len(refreshErrs) > 0 {
		r.refreshFailedAt = r.now()
	} else {
		r.refreshFailedAt = time.Time{}
	}
	return Batch{ProviderErr: errors.Join(append(providerErrs, refreshErrs...)...)}, nil
}

// cachedMappings adds the persisted mappings of bases to mappings and returns
// the bases that still need a scan. An expired resolved mapping stays in use
// until a scan replaces it.
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
		m, ok := stored[base]
		if ok && (m.Status == "resolved" || r.mappingCacheValid(m)) {
			mappings[base] = m
		}
		if !ok || !r.mappingCacheValid(m) {
			missing[base] = i
		}
	}
	return missing, nil
}

// resolveMappings scans the provider's tickers for the missing bases, at most
// once per mappingRescanInterval. scanErr reports a failed scan, or the one
// whose retry still waits, which leaves the bases unmapped; err a failure to
// persist the mappings.
func (r *Resolver) resolveMappings(ctx context.Context, missing map[string]market.Instrument, mappings map[string]Mapping) (scanErr, err error) {
	if !r.scannedAt.IsZero() && r.now().Sub(r.scannedAt) < mappingRescanInterval {
		// New or expired bases wait for the next scheduled scan.
		return nil, nil
	}
	if !r.scanFailedAt.IsZero() && r.now().Sub(r.scanFailedAt) < mappingScanRetryDelay {
		return r.scanErr, nil
	}
	all, err := r.allMappings(ctx)
	if err != nil {
		r.scanFailedAt, r.scanErr = r.now(), fmt.Errorf("scan CoinGecko tickers: %w", err)
		return r.scanErr, nil
	}
	r.scanFailedAt, r.scanErr = time.Time{}, nil
	r.scannedAt = r.now()
	byBase := make(map[string]Mapping, len(all))
	for _, m := range all {
		byBase[m.BaseAsset] = m
	}
	selections := make([]Mapping, 0, len(missing))
	for base, i := range missing {
		// The scan selects as the bootstrap does, whatever the quote.
		selected, found := byBase[base]
		switch {
		case found:
		case mappings[base].Status == "resolved":
			// A ticker missing from one scan does not unmap a resolved coin.
			selected = mappings[base]
			selected.ExpiresAt = ptr(r.now().Add(resolvedMappingTTL))
		default:
			selected = Mapping{BaseAsset: base, QuoteAsset: i.QuoteAsset, SourceSymbol: base + i.QuoteAsset, Status: "unresolved", Reason: "mapping_not_found", ExpiresAt: ptr(r.now().Add(negativeTTL))}
		}
		selections = append(selections, selected)
		mappings[base] = selected
	}
	return nil, r.store.SaveMappings(ctx, selections)
}
func (r *Resolver) mappingCacheValid(mapping Mapping) bool {
	return mapping.ExpiresAt != nil && mapping.ExpiresAt.After(r.now())
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
			candidate := Mapping{BaseAsset: t.Base, QuoteAsset: t.Target, SourceSymbol: t.Base + t.Target, CoinID: t.CoinID, Status: "resolved", ExpiresAt: ptr(r.now().Add(resolvedMappingTTL))}
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
