package postgres

import (
	"context"
	"fmt"
	"time"

	"crypto-scanner/internal/marketcap"
	"crypto-scanner/internal/platform/numeric"
	generated "crypto-scanner/internal/storage/postgres/sqlc"

	"github.com/jackc/pgx/v5/pgtype"
)

func (store *Store) BootstrapCompleted(ctx context.Context) (bool, error) {
	value, err := store.queries.MappingBootstrapCompleted(ctx)
	if err != nil {
		return false, err
	}
	completed, ok := value.(bool)
	return completed && ok, nil
}

func (store *Store) ReplaceSnapshot(ctx context.Context, mappings []marketcap.Mapping) error {
	tx, err := store.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	q := store.queries.WithTx(tx)
	if err = q.ClearMappings(ctx); err != nil {
		return err
	}
	if err = upsertMappings(ctx, q, mappings); err != nil {
		return err
	}
	if err = q.ReplaceMappingsAndCompleteBootstrap(ctx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (store *Store) ReplaceStablecoinClassifications(ctx context.Context, stablecoinIDs []string) error {
	if len(stablecoinIDs) == 0 {
		return fmt.Errorf("stablecoin classification snapshot is empty")
	}
	return store.queries.ReplaceStablecoinClassifications(ctx, stablecoinIDs)
}

func (store *Store) ListMappings(ctx context.Context, bases []string) (map[string]marketcap.Mapping, error) {
	rows, err := store.queries.ListCoinGeckoMappings(ctx, bases)
	if err != nil {
		return nil, fmt.Errorf("list CoinGecko mappings: %w", err)
	}
	mappings := make(map[string]marketcap.Mapping, len(rows))
	for _, row := range rows {
		mappings[row.BaseAsset] = marketcap.Mapping{BaseAsset: row.BaseAsset, CoinID: row.CoinID.String, QuoteAsset: row.QuoteAsset, SourceSymbol: row.SourceSymbol, Status: row.Status, Reason: row.Reason.String, ExpiresAt: timePointer(row.ExpiresAt)}
	}
	return mappings, nil
}

func (store *Store) SaveMappings(ctx context.Context, mappings []marketcap.Mapping) error {
	return upsertMappings(ctx, store.queries, mappings)
}

// upsertMappings writes all mappings in one round trip.
func upsertMappings(ctx context.Context, q *generated.Queries, mappings []marketcap.Mapping) error {
	if len(mappings) == 0 {
		return nil
	}
	observedAt := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	params := make([]generated.UpsertCoinGeckoMappingParams, len(mappings))
	for index, m := range mappings {
		params[index] = generated.UpsertCoinGeckoMappingParams{BaseAsset: m.BaseAsset, CoinID: pgtype.Text{String: m.CoinID, Valid: m.CoinID != ""}, QuoteAsset: m.QuoteAsset, SourceSymbol: m.SourceSymbol, Status: m.Status, Reason: pgtype.Text{String: m.Reason, Valid: m.Reason != ""}, ObservedAt: observedAt, ExpiresAt: timestamptz(m.ExpiresAt)}
	}
	var batchErr error
	q.UpsertCoinGeckoMapping(ctx, params).Exec(func(_ int, err error) {
		if batchErr == nil && err != nil {
			batchErr = fmt.Errorf("upsert CoinGecko mapping: %w", err)
		}
	})
	return batchErr
}

func (store *Store) ListCaps(ctx context.Context, ids []string) (map[string]marketcap.Cap, error) {
	rows, err := store.queries.ListCoinGeckoMarketCaps(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list CoinGecko market caps: %w", err)
	}
	caps := make(map[string]marketcap.Cap, len(rows))
	for _, row := range rows {
		usd, err := numeric.ParseFinite(row.MarketCapUsd)
		if err != nil {
			return nil, fmt.Errorf("invalid persisted market cap for %s", row.CoinID)
		}
		caps[row.CoinID] = marketcap.Cap{CoinID: row.CoinID, USD: usd, Available: true, FetchedAt: row.FetchedAt.Time, ObservedAt: row.ObservedAt.Time}
	}
	return caps, nil
}

// SaveCaps writes all caps in one round trip.
func (store *Store) SaveCaps(ctx context.Context, caps []marketcap.Cap) error {
	if len(caps) == 0 {
		return nil
	}
	params := make([]generated.UpsertCoinGeckoMarketCapParams, len(caps))
	for index, c := range caps {
		params[index] = generated.UpsertCoinGeckoMarketCapParams{CoinID: c.CoinID, MarketCapUsd: decimal(c.USD), FetchedAt: pgtype.Timestamptz{Time: c.FetchedAt, Valid: true}, ObservedAt: pgtype.Timestamptz{Time: c.ObservedAt, Valid: true}}
	}
	var batchErr error
	store.queries.UpsertCoinGeckoMarketCap(ctx, params).Exec(func(_ int, err error) {
		if batchErr == nil && err != nil {
			batchErr = fmt.Errorf("upsert CoinGecko market cap: %w", err)
		}
	})
	return batchErr
}
