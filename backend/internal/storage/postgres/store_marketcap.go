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
	for _, m := range mappings {
		if err = upsertMapping(ctx, q, m); err != nil {
			return err
		}
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

func (store *Store) GetMapping(ctx context.Context, base string) (marketcap.Mapping, error) {
	row, err := store.queries.GetCoinGeckoMapping(ctx, base)
	if err != nil {
		return marketcap.Mapping{}, err
	}
	return marketcap.Mapping{BaseAsset: row.BaseAsset, CoinID: row.CoinID.String, QuoteAsset: row.QuoteAsset, SourceSymbol: row.SourceSymbol, Status: row.Status, Reason: row.Reason.String, ExpiresAt: timePointer(row.ExpiresAt)}, nil
}

func (store *Store) SaveMapping(ctx context.Context, m marketcap.Mapping) error {
	return upsertMapping(ctx, store.queries, m)
}

func upsertMapping(ctx context.Context, q *generated.Queries, m marketcap.Mapping) error {
	return q.UpsertCoinGeckoMapping(ctx, generated.UpsertCoinGeckoMappingParams{BaseAsset: m.BaseAsset, CoinID: pgtype.Text{String: m.CoinID, Valid: m.CoinID != ""}, QuoteAsset: m.QuoteAsset, SourceSymbol: m.SourceSymbol, Status: m.Status, Reason: pgtype.Text{String: m.Reason, Valid: m.Reason != ""}, ObservedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}, ExpiresAt: timestamptz(m.ExpiresAt)})
}

func (store *Store) GetCap(ctx context.Context, id string) (marketcap.Cap, error) {
	row, err := store.queries.GetCoinGeckoMarketCap(ctx, id)
	if err != nil {
		return marketcap.Cap{}, err
	}
	usd, err := numeric.ParseFinite(row.MarketCapUsd)
	if err != nil {
		return marketcap.Cap{}, err
	}
	return marketcap.Cap{CoinID: row.CoinID, USD: usd, Available: true, FetchedAt: row.FetchedAt.Time, ObservedAt: row.ObservedAt.Time}, nil
}

func (store *Store) SaveCap(ctx context.Context, c marketcap.Cap) error {
	return store.queries.UpsertCoinGeckoMarketCap(ctx, generated.UpsertCoinGeckoMarketCapParams{CoinID: c.CoinID, MarketCapUsd: decimal(c.USD), FetchedAt: pgtype.Timestamptz{Time: c.FetchedAt, Valid: true}, ObservedAt: pgtype.Timestamptz{Time: c.ObservedAt, Valid: true}})
}
