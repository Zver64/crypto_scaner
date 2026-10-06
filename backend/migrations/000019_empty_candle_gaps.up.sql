-- Gaps between stored candles for which the exchange returned no candles,
-- such as trading halts. Synchronization does not request a gap again before
-- retry_after; a gap is identified by its first missing open time and the
-- open time of the next stored candle.
CREATE TABLE binance_spot.empty_candle_gaps (
    instrument_id BIGINT NOT NULL REFERENCES binance_spot.instruments(id) ON DELETE CASCADE,
    interval      TEXT NOT NULL,
    gap_from      TIMESTAMPTZ NOT NULL,
    gap_to        TIMESTAMPTZ NOT NULL,
    retry_after   TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (instrument_id, interval, gap_from),
    CONSTRAINT empty_candle_gaps_supported_interval
        CHECK (interval IN ('1h', '1d', '1w', '1M')),
    CONSTRAINT empty_candle_gaps_ordered CHECK (gap_from < gap_to)
);
