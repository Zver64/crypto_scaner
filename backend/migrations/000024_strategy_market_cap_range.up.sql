-- A strategy may buy only coins whose current market cap, in USD, lies within
-- its range; either bound may be absent. The range limits only buys of the
-- running strategy: open trades exit as usual, and backtests ignore it.
ALTER TABLE app.strategies
    ADD COLUMN min_market_cap_usd DOUBLE PRECISION CHECK (min_market_cap_usd > 0),
    ADD COLUMN max_market_cap_usd DOUBLE PRECISION CHECK (max_market_cap_usd > 0),
    ADD CONSTRAINT strategies_market_cap_range CHECK (min_market_cap_usd <= max_market_cap_usd);
