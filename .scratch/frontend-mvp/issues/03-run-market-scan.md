# 03 — Run a Market Scan End to End

**What to build:** Allow an authorized user to enter supported Market Scan criteria, explicitly submit them, and receive the backend's ordered Scan Result as a compact table.

**Blocked by:** 02 — Bootstrap the Mini App Shell and Access State.

**Status:** ready-for-agent

- [ ] The primary route presents an English-language Market Scan form without automatically requesting analysis on page load or input change.
- [ ] Analysis Period is an integer input with default 30 and accepted bounds 1–3650.
- [ ] Range Percentile is an integer input with default 80 and accepted bounds 0–100.
- [ ] Minimum Range is a non-negative numeric input with default 3 and step 0.1.
- [ ] Mantine Form owns draft values and field validation; invalid input prevents submission and produces clear field-level feedback.
- [ ] Valid submission commits the three criteria to validated TanStack Router search parameters before requesting the Market Scan.
- [ ] Editing draft values without submitting does not change the committed URL or the displayed Scan Result.
- [ ] TanStack Query calls the existing market percentile endpoint with the committed `period_days`, `percentile`, and `minimum_range_percent` values.
- [ ] The first request with no prior data shows a Mantine loader, and the submit button exposes its built-in loading state.
- [ ] Submission is disabled while values are invalid, authentication is absent, readiness is unavailable, or an equivalent request is already pending.
- [ ] A successful response renders one compact Mantine table row per returned instrument at mobile and desktop widths.
- [ ] Each row contains exactly the backend's unmodified `symbol`, `range_percent`, and `candle_count` values as the three data columns.
- [ ] The frontend preserves backend row ordering and does not add interactive sorting.
- [ ] An empty successful response renders a useful empty state rather than an error.
- [ ] Canonical backend, network, and unexpected failures are normalized at the API boundary and displayed through the globally mounted Mantine notification system.
- [ ] No current price, volume, unsupported instrument metadata, or full-market catalog is displayed or implied.
