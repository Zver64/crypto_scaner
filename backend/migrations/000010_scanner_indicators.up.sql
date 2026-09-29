CREATE TABLE app.scanner_indicators (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    interval       TEXT NOT NULL,
    indicator_type TEXT NOT NULL,
    parameters     JSONB NOT NULL,
    show_in_table  BOOLEAN NOT NULL,
    scale_min      DOUBLE PRECISION,
    scale_max      DOUBLE PRECISION,
    scale_levels   DOUBLE PRECISION[] NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT scanner_indicators_supported_interval CHECK (interval IN ('1h', '1d', '1w', '1M')),
    CONSTRAINT scanner_indicators_unique_selection UNIQUE (interval, indicator_type, parameters)
);
