-- name: ListAuditedCoinIDs :many
SELECT DISTINCT mapping.coin_id::text AS coin_id
FROM binance_spot.instruments AS instrument
JOIN app.coingecko_asset_mappings AS mapping
  ON mapping.base_asset = instrument.base_asset
 AND mapping.status = 'resolved'
WHERE instrument.is_active = TRUE
  AND mapping.coin_id IS NOT NULL
ORDER BY coin_id;

-- name: ListTokenAudits :many
SELECT coin_id, chain, contract_address, audited_at
FROM app.token_security_audits;

-- name: UpsertTokenAudit :exec
INSERT INTO app.token_security_audits (coin_id, chain, contract_address, supported, issues, audited_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (coin_id, chain) DO UPDATE SET contract_address=EXCLUDED.contract_address, supported=EXCLUDED.supported, issues=EXCLUDED.issues, audited_at=EXCLUDED.audited_at;

-- name: DeleteTokenAuditsExcept :exec
DELETE FROM app.token_security_audits AS audit
WHERE NOT EXISTS (
  SELECT 1
  FROM (
    SELECT unnest(sqlc.arg(coin_ids)::text[]) AS coin_id, unnest(sqlc.arg(chains)::text[]) AS chain,
           unnest(sqlc.arg(addresses)::text[]) AS contract_address
  ) AS kept
  WHERE kept.coin_id = audit.coin_id AND kept.chain = audit.chain AND kept.contract_address = audit.contract_address
);

-- name: ListTokenSecurityIssues :many
SELECT mapping.base_asset, audit.chain, audit.issues
FROM app.coingecko_asset_mappings AS mapping
JOIN app.token_security_audits AS audit
  ON audit.coin_id = mapping.coin_id
WHERE mapping.status = 'resolved'
  AND mapping.base_asset = ANY(sqlc.arg(base_assets)::text[])
  AND audit.supported = TRUE
  AND jsonb_array_length(audit.issues) > 0
ORDER BY mapping.base_asset, audit.chain;
