DROP TABLE IF EXISTS app.strategy_states;

CREATE TABLE app.strategy_matches (
    strategy_id   BIGINT NOT NULL REFERENCES app.strategies(id) ON DELETE CASCADE,
    instrument_id BIGINT NOT NULL REFERENCES binance_spot.instruments(id) ON DELETE CASCADE,
    matched_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (strategy_id, instrument_id)
);

CREATE INDEX strategy_matches_instrument_idx ON app.strategy_matches (instrument_id);

ALTER TABLE app.strategies
    DROP CONSTRAINT IF EXISTS strategies_accumulate_needs_exit,
    DROP CONSTRAINT IF EXISTS strategies_max_buys_range,
    DROP COLUMN IF EXISTS max_buys,
    DROP COLUMN IF EXISTS accumulate,
    DROP COLUMN IF EXISTS exit_expression;
