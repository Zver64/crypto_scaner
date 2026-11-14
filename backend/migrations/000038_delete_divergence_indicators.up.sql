-- The divergence indicator is removed: a divergence is a rule over the
-- pivots of an oscillator. The periods of the deleted indicators go with
-- them (ON DELETE CASCADE).
DELETE FROM app.scanner_indicators WHERE indicator_type = 'divergence';
