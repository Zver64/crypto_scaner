-- Strategies are administrator-defined CEL expressions over scanner
-- indicators; matching favorites trigger Telegram alerts.
CREATE TABLE app.strategies (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,
    expression TEXT NOT NULL,
    enabled    BOOLEAN NOT NULL,
    -- The current matches must be announced; set by every change that needs
    -- a summary and cleared when the matches are replaced.
    baseline_pending BOOLEAN NOT NULL DEFAULT FALSE,
    -- Grows with every expression or enabled change; match writes of an
    -- evaluation apply only to the revision it evaluated.
    revision   BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The indicators an expression reads. RESTRICT keeps an indicator while any
-- strategy uses it.
CREATE TABLE app.strategy_indicators (
    strategy_id  BIGINT NOT NULL REFERENCES app.strategies(id) ON DELETE CASCADE,
    indicator_id BIGINT NOT NULL REFERENCES app.scanner_indicators(id) ON DELETE RESTRICT,
    PRIMARY KEY (strategy_id, indicator_id)
);

CREATE INDEX strategy_indicators_indicator_idx ON app.strategy_indicators (indicator_id);

-- Instruments that currently match a strategy, so a restart never repeats an
-- alert.
CREATE TABLE app.strategy_matches (
    strategy_id   BIGINT NOT NULL REFERENCES app.strategies(id) ON DELETE CASCADE,
    instrument_id BIGINT NOT NULL REFERENCES binance_spot.instruments(id) ON DELETE CASCADE,
    matched_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (strategy_id, instrument_id)
);

ALTER TABLE app.users ADD COLUMN strategy_alerts BOOLEAN NOT NULL DEFAULT FALSE;
