CREATE TABLE app.coingecko_mapping_bootstrap (
    id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    completed_at TIMESTAMPTZ
);
INSERT INTO app.coingecko_mapping_bootstrap (id) VALUES (TRUE) ON CONFLICT DO NOTHING;

CREATE TABLE app.coingecko_asset_mappings (
    base_asset TEXT PRIMARY KEY,
    coin_id TEXT,
    quote_asset TEXT NOT NULL,
    source_symbol TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('resolved', 'unresolved')),
    reason TEXT,
    observed_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ
);
CREATE TABLE app.coingecko_market_caps (
    coin_id TEXT PRIMARY KEY,
    market_cap_usd NUMERIC NOT NULL CHECK (market_cap_usd >= 0),
    fetched_at TIMESTAMPTZ NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL
);
