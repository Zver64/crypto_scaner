ALTER TABLE app.coingecko_asset_mappings
    ADD COLUMN is_stablecoin BOOLEAN;

UPDATE app.coingecko_asset_mappings AS mapping
SET is_stablecoin = classification.is_stablecoin
FROM app.asset_classifications AS classification
WHERE classification.base_asset = mapping.base_asset;

DROP TABLE app.asset_classifications;
