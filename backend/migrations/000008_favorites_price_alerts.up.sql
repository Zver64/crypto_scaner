CREATE TABLE app.favorites (
    user_id       BIGINT NOT NULL REFERENCES app.users(id) ON DELETE CASCADE,
    instrument_id BIGINT NOT NULL REFERENCES binance_spot.instruments(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, instrument_id)
);

CREATE TABLE app.price_alerts (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id       BIGINT NOT NULL,
    instrument_id BIGINT NOT NULL,
    target        NUMERIC(38, 18) NOT NULL,
    version       BIGINT NOT NULL DEFAULT 1,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT price_alerts_target_positive CHECK (target > 0),
    CONSTRAINT price_alerts_favorite_fk FOREIGN KEY (user_id, instrument_id)
        REFERENCES app.favorites(user_id, instrument_id) ON DELETE CASCADE,
    CONSTRAINT price_alerts_unique_target UNIQUE (user_id, instrument_id, target)
);

CREATE INDEX favorites_instrument_idx ON app.favorites (instrument_id);
CREATE INDEX price_alerts_instrument_idx ON app.price_alerts (instrument_id);
