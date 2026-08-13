# 05 — Drill into Instrument Analysis

**What to build:** Allow a user to select a matching instrument, inspect its individual analysis, recalculate it with supported criteria, and return to the preserved Market Scan.

**Blocked by:** 03 — Run a Market Scan End to End.

**Status:** resolved

- [x] Selecting any Scan Result row navigates to a dedicated Instrument Analysis route for that exact backend symbol.
- [x] The route path preserves the symbol without frontend display transformation, and validated search parameters contain Analysis Period and Range Percentile.
- [x] The initial Instrument Analysis inherits the period and percentile used by the originating Market Scan.
- [x] Refreshing the Instrument Analysis URL reconstructs and reruns the same request without requiring the originating table to remain in memory.
- [x] The page calls the existing single-instrument percentile endpoint and displays symbol, Daily Range percentage, candle count, and coverage boundaries.
- [x] Coverage boundaries are rendered as short calendar dates with a visible UTC label and are not converted to local time.
- [x] The page provides integer Period and Percentile inputs with the same supported bounds as Market Scan.
- [x] Draft edits do not trigger a request; explicit valid submission commits new route search parameters and recalculates.
- [x] Instrument-specific recalculation does not mutate the originating Market Scan criteria, Query key, or cached result.
- [x] First load, refresh, disabled controls, and error notifications follow the same Mantine and TanStack Query conventions as Market Scan.
- [x] Inside Telegram, the native BackButton is shown only on this route, delegates navigation to TanStack Router, and is unsubscribed and hidden during cleanup.
- [x] Outside Telegram, a Mantine back control provides equivalent browser-development navigation without duplicating the Telegram control.
- [x] Returning through normal navigation restores the prior Scan Result and scroll context from the in-memory Query cache without automatically requesting it again.
- [x] The interface provides no manual symbol input, global symbol search, autocomplete, or navigation to instruments absent from the Scan Result.
- [x] Shared application components are extracted only where Market Scan and Instrument Analysis provide two confirmed consumers or where Mantine lacks the required application behavior.

## Answer

Added the validated `/instruments/$symbol` route, exact-symbol row navigation, the authorized single-instrument API boundary, explicit recalculation, UTC coverage presentation, cached request state, and Telegram/browser back behavior. Recalculation replaces the current instrument history entry so Back returns to the originating Market Scan.

Pure Vitest coverage verifies instrument criteria and search parameters, exact symbol preservation and URL encoding, API response/error mapping, and UTC date formatting. Live browser acceptance verified navigation from a 424-row Scan Result, 30-to-20-day recalculation, URL reload reconstruction, and cached return without a Market Scan refetch.
