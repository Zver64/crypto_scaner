# 05 — Drill into Instrument Analysis

**What to build:** Allow a user to select a matching instrument, inspect its individual analysis, recalculate it with supported criteria, and return to the preserved Market Scan.

**Blocked by:** 03 — Run a Market Scan End to End.

**Status:** ready-for-agent

- [ ] Selecting any Scan Result row navigates to a dedicated Instrument Analysis route for that exact backend symbol.
- [ ] The route path preserves the symbol without frontend display transformation, and validated search parameters contain Analysis Period and Range Percentile.
- [ ] The initial Instrument Analysis inherits the period and percentile used by the originating Market Scan.
- [ ] Refreshing the Instrument Analysis URL reconstructs and reruns the same request without requiring the originating table to remain in memory.
- [ ] The page calls the existing single-instrument percentile endpoint and displays symbol, Daily Range percentage, candle count, and coverage boundaries.
- [ ] Coverage boundaries are rendered as short calendar dates with a visible UTC label and are not converted to local time.
- [ ] The page provides integer Period and Percentile inputs with the same supported bounds as Market Scan.
- [ ] Draft edits do not trigger a request; explicit valid submission commits new route search parameters and recalculates.
- [ ] Instrument-specific recalculation does not mutate the originating Market Scan criteria, Query key, or cached result.
- [ ] First load, refresh, disabled controls, and error notifications follow the same Mantine and TanStack Query conventions as Market Scan.
- [ ] Inside Telegram, the native BackButton is shown only on this route, delegates navigation to TanStack Router, and is unsubscribed and hidden during cleanup.
- [ ] Outside Telegram, a Mantine back control provides equivalent browser-development navigation without duplicating the Telegram control.
- [ ] Returning through normal navigation restores the prior Scan Result and scroll context from the in-memory Query cache without automatically requesting it again.
- [ ] The interface provides no manual symbol input, global symbol search, autocomplete, or navigation to instruments absent from the Scan Result.
- [ ] Shared application components are extracted only where Market Scan and Instrument Analysis provide two confirmed consumers or where Mantine lacks the required application behavior.
