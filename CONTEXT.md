# Crypto Scanner

Crypto Scanner evaluates cryptocurrency instruments through an ordered set of configurable criteria.

## Language

**Criterion**:
A reusable rule that evaluates an instrument and determines whether it remains eligible in an analysis pipeline.
_Avoid_: Filter

**Criterion Instance**:
A configured use of a criterion within an analysis pipeline. It has a composition-level key and display label, allowing the same criterion to be used more than once.
_Avoid_: Mode, tab

**Volatility Criterion**:
A criterion that evaluates a percentile of candle high-low ranges over a configured period and candle unit.
_Avoid_: Percentile criterion, daily criterion, hourly criterion

**Analysis Pipeline**:
An ordered sequence of criterion instances in which each criterion evaluates only the instruments retained by preceding criteria.
_Avoid_: Daily scan, hourly scan
