-- name: ListCoinMPerpetuals :many
SELECT base_asset
FROM binance_spot.coinm_perpetuals
ORDER BY base_asset;

-- name: ClearCoinMPerpetuals :exec
DELETE FROM binance_spot.coinm_perpetuals;

-- name: InsertCoinMPerpetuals :exec
INSERT INTO binance_spot.coinm_perpetuals (base_asset)
SELECT DISTINCT unnest(sqlc.arg(base_assets)::text[]);
