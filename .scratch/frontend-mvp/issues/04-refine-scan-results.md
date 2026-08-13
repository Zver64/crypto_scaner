# 04 — Make Scan Results Usable and Refreshable

**What to build:** Make a completed Scan Result efficient to interpret, filter, and refresh while retaining useful prior data and preserving the backend's semantics.

**Blocked by:** 03 — Run a Market Scan End to End.

**Status:** resolved

- [x] A compact summary above the table displays the backend's matched, analyzed, and insufficient-data counts.
- [x] A local symbol filter narrows only the rows in the current Scan Result and never sends a request or exposes instruments outside that result.
- [x] The symbol filter supports empty, partial, case-insensitive, and no-match states while preserving the relative backend order.
- [x] Symbols remain byte-for-byte identical to the backend value; no slash, quote-asset removal, or display-name transformation is introduced.
- [x] Daily Range percentages are formatted in English with `Intl.NumberFormat` and three significant digits.
- [x] Formatting changes only presentation and never mutates cached numeric values or server ordering.
- [x] Re-submitting the same committed criteria explicitly refetches instead of silently treating cached data as fresh.
- [x] A refresh retains the previous table beneath a Mantine `LoadingOverlay` until the new response succeeds.
- [x] A failed refresh leaves the previous successful result available and reports the failure through a global notification.
- [x] The local symbol filter is session-only page state and is not written to the URL or persistent browser storage.
- [x] The dense table remains a single-row-per-instrument table at supported mobile widths and does not transform into cards.
- [x] Pure formatter and filter behavior is structured so it can be verified with Vitest without rendering React components.

## Answer

Added the Scan Result summary, a session-only case-insensitive symbol filter, three-significant-digit range formatting, and explicit same-criteria refresh behavior that retains successful data under a loading overlay. Pure formatter and filter tests cover presentation, matching, exact symbol preservation, and backend ordering. Verified the live result and refresh flow against the ready local backend and confirmed the three-column table at a 390 px viewport.

Validation: `npm test`, `npm run quality`, and `npm run build` all pass; browser console reported no errors.
