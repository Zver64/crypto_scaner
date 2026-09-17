-- Market-wide scanner defaults are data, not an exchange activation state. The
-- list is keyed by the Binance base asset because one asset can have several quotes.
CREATE TABLE app.asset_classifications (
    base_asset TEXT PRIMARY KEY,
    is_stablecoin BOOLEAN NOT NULL DEFAULT FALSE,
    policy_source TEXT NOT NULL
);

INSERT INTO app.asset_classifications (base_asset, is_stablecoin, policy_source) VALUES
    ('AEUR', TRUE, 'reviewed_static_list'), ('BUSD', TRUE, 'reviewed_static_list'),
    ('DAI', TRUE, 'reviewed_static_list'), ('EURI', TRUE, 'reviewed_static_list'),
    ('FDUSD', TRUE, 'reviewed_static_list'), ('FRAX', TRUE, 'reviewed_static_list'),
    ('GUSD', TRUE, 'reviewed_static_list'), ('LUSD', TRUE, 'reviewed_static_list'),
    ('PYUSD', TRUE, 'reviewed_static_list'), ('RLUSD', TRUE, 'reviewed_static_list'),
    ('TUSD', TRUE, 'reviewed_static_list'), ('USDC', TRUE, 'reviewed_static_list'),
    ('USDD', TRUE, 'reviewed_static_list'), ('USDE', TRUE, 'reviewed_static_list'),
    ('USDP', TRUE, 'reviewed_static_list'), ('USD1', TRUE, 'reviewed_static_list'),
    ('USTC', TRUE, 'reviewed_static_list')
ON CONFLICT (base_asset) DO UPDATE SET is_stablecoin = EXCLUDED.is_stablecoin, policy_source = EXCLUDED.policy_source;
