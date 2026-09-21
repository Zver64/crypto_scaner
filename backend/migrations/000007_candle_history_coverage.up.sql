CREATE TABLE binance_spot.candle_history_coverage (
    instrument_id            BIGINT NOT NULL REFERENCES binance_spot.instruments(id) ON DELETE CASCADE,
    interval                 TEXT NOT NULL,
    verified_oldest_open_time TIMESTAMPTZ NOT NULL,
    target_depth             INTEGER NOT NULL,
    policy_version           INTEGER NOT NULL,
    retry_after              TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (instrument_id, interval),
    CONSTRAINT candle_history_coverage_supported_interval
        CHECK (interval IN ('1h', '1d', '1w', '1M')),
    CONSTRAINT candle_history_coverage_target_positive CHECK (target_depth > 0),
    CONSTRAINT candle_history_coverage_policy_positive CHECK (policy_version > 0)
);
