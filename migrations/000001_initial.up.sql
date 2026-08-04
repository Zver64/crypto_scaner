CREATE TEMP TABLE migration_v1_schema_ownership ON COMMIT DROP AS
SELECT to_regnamespace('app') IS NULL AS app_schema_created,
       to_regnamespace('binance_spot') IS NULL AS binance_spot_schema_created;

CREATE SCHEMA IF NOT EXISTS app;
CREATE SCHEMA IF NOT EXISTS binance_spot;

CREATE TABLE app.schema_migrations (
    version    BIGINT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    app_schema_created BOOLEAN NOT NULL,
    binance_spot_schema_created BOOLEAN NOT NULL
);

CREATE TABLE app.users (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    telegram_id  BIGINT NOT NULL UNIQUE,
    username     TEXT,
    display_name TEXT,
    is_enabled   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE binance_spot.instruments (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    symbol          TEXT NOT NULL UNIQUE,
    base_asset      TEXT NOT NULL,
    quote_asset     TEXT NOT NULL,
    exchange_status TEXT NOT NULL,
    is_active       BOOLEAN NOT NULL,
    CONSTRAINT instruments_symbol_nonempty CHECK (symbol <> ''),
    CONSTRAINT instruments_base_nonempty CHECK (base_asset <> ''),
    CONSTRAINT instruments_quote_nonempty CHECK (quote_asset <> '')
);

CREATE TABLE binance_spot.candles (
    instrument_id      BIGINT NOT NULL REFERENCES binance_spot.instruments(id),
    interval           TEXT NOT NULL,
    open_time          TIMESTAMPTZ NOT NULL,
    close_time         TIMESTAMPTZ NOT NULL,
    open               NUMERIC NOT NULL,
    high               NUMERIC NOT NULL,
    low                NUMERIC NOT NULL,
    close              NUMERIC NOT NULL,
    volume             NUMERIC NOT NULL,
    quote_asset_volume NUMERIC NOT NULL,
    trade_count        BIGINT NOT NULL,
    PRIMARY KEY (instrument_id, interval, open_time),
    CONSTRAINT candles_supported_interval CHECK (interval = '1d'),
    CONSTRAINT candles_time_order CHECK (close_time > open_time),
    CONSTRAINT candles_open_positive CHECK (open > 0),
    CONSTRAINT candles_high_valid CHECK (high >= open AND high >= close AND high >= low),
    CONSTRAINT candles_low_valid CHECK (low <= open AND low <= close),
    CONSTRAINT candles_volume_nonnegative CHECK (volume >= 0),
    CONSTRAINT candles_quote_volume_nonnegative CHECK (quote_asset_volume >= 0),
    CONSTRAINT candles_trade_count_nonnegative CHECK (trade_count >= 0)
);

CREATE INDEX candles_instrument_interval_time_desc_idx
    ON binance_spot.candles (instrument_id, interval, open_time DESC);

CREATE TABLE binance_spot.sync_state (
    profile_key           TEXT PRIMARY KEY,
    last_started_at       TIMESTAMPTZ,
    last_succeeded_at     TIMESTAMPTZ,
    last_closed_open_time TIMESTAMPTZ,
    status                TEXT NOT NULL,
    error_message         TEXT,
    CONSTRAINT sync_state_status
        CHECK (status IN ('never_run', 'running', 'succeeded', 'failed'))
);

INSERT INTO app.schema_migrations (
    version,
    app_schema_created,
    binance_spot_schema_created
)
SELECT 1, app_schema_created, binance_spot_schema_created
FROM migration_v1_schema_ownership;
