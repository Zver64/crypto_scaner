-- name: MappingBootstrapCompleted :one
SELECT completed_at IS NOT NULL FROM app.coingecko_mapping_bootstrap WHERE id = TRUE;

-- name: ReplaceMappingsAndCompleteBootstrap :exec
UPDATE app.coingecko_mapping_bootstrap SET completed_at = now() WHERE id = TRUE;

-- name: ClearMappings :exec
DELETE FROM app.coingecko_asset_mappings;

-- name: UpsertCoinGeckoMapping :exec
INSERT INTO app.coingecko_asset_mappings (base_asset, coin_id, quote_asset, source_symbol, status, reason, observed_at, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (base_asset) DO UPDATE SET coin_id=EXCLUDED.coin_id, quote_asset=EXCLUDED.quote_asset, source_symbol=EXCLUDED.source_symbol, status=EXCLUDED.status, reason=EXCLUDED.reason, observed_at=EXCLUDED.observed_at, expires_at=EXCLUDED.expires_at;

-- name: GetCoinGeckoMapping :one
SELECT base_asset, coin_id, quote_asset, source_symbol, status, reason, observed_at, expires_at FROM app.coingecko_asset_mappings WHERE base_asset = $1;

-- name: GetCoinGeckoMarketCap :one
SELECT coin_id, market_cap_usd, fetched_at, observed_at FROM app.coingecko_market_caps WHERE coin_id = $1;

-- name: UpsertCoinGeckoMarketCap :exec
INSERT INTO app.coingecko_market_caps (coin_id, market_cap_usd, fetched_at, observed_at) VALUES ($1, $2, $3, $4)
ON CONFLICT (coin_id) DO UPDATE SET market_cap_usd=EXCLUDED.market_cap_usd, fetched_at=EXCLUDED.fetched_at, observed_at=EXCLUDED.observed_at;
