-- name: MappingBootstrapCompleted :one
SELECT (completed_at IS NOT NULL)::boolean AS completed FROM app.coingecko_mapping_bootstrap WHERE id = TRUE;

-- name: ReplaceMappingsAndCompleteBootstrap :exec
UPDATE app.coingecko_mapping_bootstrap SET completed_at = now() WHERE id = TRUE;

-- name: ClearMappings :exec
DELETE FROM app.coingecko_asset_mappings;

-- name: UpsertCoinGeckoMapping :batchexec
INSERT INTO app.coingecko_asset_mappings (base_asset, coin_id, quote_asset, source_symbol, status, reason, observed_at, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (base_asset) DO UPDATE SET coin_id=EXCLUDED.coin_id, quote_asset=EXCLUDED.quote_asset, source_symbol=EXCLUDED.source_symbol, status=EXCLUDED.status, reason=EXCLUDED.reason, observed_at=EXCLUDED.observed_at, expires_at=EXCLUDED.expires_at,
    -- A changed coin is unclassified until the next classification refresh.
    is_stablecoin = CASE
        WHEN coingecko_asset_mappings.coin_id IS DISTINCT FROM EXCLUDED.coin_id OR coingecko_asset_mappings.status <> EXCLUDED.status THEN NULL
        ELSE coingecko_asset_mappings.is_stablecoin
    END;

-- name: ReplaceStablecoinClassifications :exec
UPDATE app.coingecko_asset_mappings
SET is_stablecoin = CASE
    WHEN status = 'resolved' THEN coin_id = ANY(sqlc.arg(stablecoin_ids)::text[])
    ELSE NULL
END
-- Rows already classified this way are not rewritten.
WHERE is_stablecoin IS DISTINCT FROM CASE
    WHEN status = 'resolved' THEN coin_id = ANY(sqlc.arg(stablecoin_ids)::text[])
    ELSE NULL
END;

-- name: ListCoinGeckoMappings :many
SELECT base_asset, coin_id, quote_asset, source_symbol, status, reason, observed_at, expires_at
FROM app.coingecko_asset_mappings WHERE base_asset = ANY(sqlc.arg(base_assets)::text[]);

-- name: ListCoinGeckoMarketCaps :many
-- Rows from before only positive caps were stored are not available caps.
SELECT coin_id, market_cap_usd, fetched_at, observed_at
FROM app.coingecko_market_caps WHERE coin_id = ANY(sqlc.arg(coin_ids)::text[]) AND market_cap_usd > 0;

-- name: UpsertCoinGeckoMarketCap :batchexec
INSERT INTO app.coingecko_market_caps (coin_id, market_cap_usd, fetched_at, observed_at) VALUES ($1, $2, $3, $4)
ON CONFLICT (coin_id) DO UPDATE SET market_cap_usd=EXCLUDED.market_cap_usd, fetched_at=EXCLUDED.fetched_at, observed_at=EXCLUDED.observed_at;

-- name: DeleteCoinGeckoMarketCaps :exec
DELETE FROM app.coingecko_market_caps WHERE coin_id = ANY(sqlc.arg(coin_ids)::text[]);
