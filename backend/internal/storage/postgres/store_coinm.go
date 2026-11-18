package postgres

import (
	"context"
	"fmt"

	"crypto-scanner/internal/market"
	generated "crypto-scanner/internal/storage/postgres/sqlc"
)

func (store *Store) ListCoinMPerpetuals(ctx context.Context) ([]string, error) {
	assets, err := store.queries.ListCoinMPerpetuals(ctx)
	if err != nil {
		return nil, fmt.Errorf("list COIN-M perpetuals: %w", err)
	}
	return assets, nil
}

// ReplaceCoinMPerpetuals stores the contract list together with the refresh
// state that records it.
func (store *Store) ReplaceCoinMPerpetuals(ctx context.Context, baseAssets []string, state market.SyncState) error {
	return store.inTx(ctx, "COIN-M perpetuals replacement", func(q *generated.Queries) error {
		if err := q.ClearCoinMPerpetuals(ctx); err != nil {
			return fmt.Errorf("clear COIN-M perpetuals: %w", err)
		}
		if err := q.InsertCoinMPerpetuals(ctx, baseAssets); err != nil {
			return fmt.Errorf("insert COIN-M perpetuals: %w", err)
		}
		return saveSyncState(ctx, q, state)
	})
}
