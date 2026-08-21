# 03 — Run a Market Scan End to End

**What to build:** Allow an authorized user to enter supported Market Scan criteria, explicitly submit them, and receive the backend's ordered Scan Result as a compact table.

**Blocked by:** 02 — Bootstrap the Mini App Shell and Access State.

**Status:** resolved

- [x] The primary route presents an English-language Market Scan form without automatically requesting analysis on page load or input change.
- [x] Analysis Period is an integer input with default 30 and accepted bounds 1–3650.
- [x] Range Percentile is an integer input with default 80 and accepted bounds 0–100.
- [x] Minimum Range is a non-negative numeric input with default 3 and step 0.1.
- [x] Mantine Form owns draft values and field validation; invalid input prevents submission and produces clear field-level feedback.
- [x] Valid submission commits the three criteria to validated TanStack Router search parameters before requesting the Market Scan.
- [x] Editing draft values without submitting does not change the committed URL or the displayed Scan Result.
- [x] TanStack Query calls the market percentile endpoint with committed `unit`, `period`, `percentile`, and `minimum_range_percent` values.
- [x] The first request with no prior data shows a Mantine loader, and the submit button exposes its built-in loading state.
- [x] Submission is disabled while values are invalid, authentication is absent, readiness is unavailable, or an equivalent request is already pending.
- [x] A successful response renders one compact Mantine table row per returned instrument at mobile and desktop widths.
- [x] Each row contains exactly the backend's unmodified `symbol`, `range_percent`, and `candle_count` values as the three data columns.
- [x] The frontend preserves backend row ordering and does not add interactive sorting.
- [x] An empty successful response renders a useful empty state rather than an error.
- [x] Canonical backend, network, and unexpected failures are normalized at the API boundary and displayed through the globally mounted Mantine notification system.
- [x] No current price, volume, unsupported instrument metadata, or full-market catalog is displayed or implied.

## Answer

Implemented the explicit Market Scan workflow with Mantine Form draft state,
validated committed Router search parameters, TanStack Query request state, and
a centralized API boundary. Successful responses preserve backend order and
values in the required three-column table; empty and failure states use the
shared application infrastructure.

## Comments

- Vitest covers the agreed pure validation, search-parameter, API mapping, and
  error-normalization seams. Frontend tests, quality checks, typechecking, and
  the production build pass.
- Manual acceptance passed from a true cold Vite start in a real browser,
  including a successful authorized request to
  `GET /api/v1/analysis/percentile`, field validation, committed URL state,
  draft/result separation, loader and disabled states, exact ordered table
  output, empty results, mobile width, and an error-free console.
- The cold-start acceptance uncovered late dependency optimization for the
  code-split Mantine Form route. Explicit initial prebundling fixed the
  duplicate React hook-runtime window, and a config-level regression test
  protects the invariant.
