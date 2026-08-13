# 04 — Make Scan Results Usable and Refreshable

**What to build:** Make a completed Scan Result efficient to interpret, filter, and refresh while retaining useful prior data and preserving the backend's semantics.

**Blocked by:** 03 — Run a Market Scan End to End.

**Status:** ready-for-agent

- [ ] A compact summary above the table displays the backend's matched, analyzed, and insufficient-data counts.
- [ ] A local symbol filter narrows only the rows in the current Scan Result and never sends a request or exposes instruments outside that result.
- [ ] The symbol filter supports empty, partial, case-insensitive, and no-match states while preserving the relative backend order.
- [ ] Symbols remain byte-for-byte identical to the backend value; no slash, quote-asset removal, or display-name transformation is introduced.
- [ ] Daily Range percentages are formatted in English with `Intl.NumberFormat` and three significant digits.
- [ ] Formatting changes only presentation and never mutates cached numeric values or server ordering.
- [ ] Re-submitting the same committed criteria explicitly refetches instead of silently treating cached data as fresh.
- [ ] A refresh retains the previous table beneath a Mantine `LoadingOverlay` until the new response succeeds.
- [ ] A failed refresh leaves the previous successful result available and reports the failure through a global notification.
- [ ] The local symbol filter is session-only page state and is not written to the URL or persistent browser storage.
- [ ] The dense table remains a single-row-per-instrument table at supported mobile widths and does not transform into cards.
- [ ] Pure formatter and filter behavior is structured so it can be verified with Vitest without rendering React components.
