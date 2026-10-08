ALTER TABLE app.strategies
    DROP CONSTRAINT IF EXISTS strategies_market_cap_range,
    DROP COLUMN IF EXISTS min_market_cap_usd,
    DROP COLUMN IF EXISTS max_market_cap_usd;
