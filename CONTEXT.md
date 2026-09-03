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

**Seven-day Price History**:
An instrument's hourly closing prices within a rolling seven-day period ending at the last completed market hour. Instruments in the same analysis share the period, even when some of their price history is unavailable; the history is independent of the criteria used to select them.
_Avoid_: Calendar week history

**Position**:
A directional futures exposure in an instrument, represented by its entry price, quantity, leverage, and margin mode.
_Avoid_: Trade, order

**Entry Price**:
The price at which a Position was opened. It is distinct from a break-even price and from a current market price.
_Avoid_: Average price, cost basis

**Liquidation Price**:
The estimated mark price at which a Position's isolated margin no longer covers its maintenance margin requirement.
_Avoid_: Bankruptcy price
