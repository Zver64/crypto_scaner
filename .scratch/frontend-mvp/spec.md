# Frontend MVP for Crypto Scanner

Status: ready-for-agent

## Problem Statement

The backend already supports authenticated market scanning and single-instrument analysis, but users do not have a production-oriented frontend through which to use those capabilities. The current frontend is only a technical template and does not provide Telegram Mini App authentication, usable analysis forms, results navigation, request-state handling, or a coherent mobile interface.

Users need a compact Telegram Mini App that lets them define market-scanning criteria, inspect the matching instruments, and open an individual instrument for further analysis without losing their prior result. The frontend must remain strictly within the current backend contract and must not imply unsupported data such as current price, volume, or a complete instrument catalog.

## Solution

Build an English-language, dark-themed React MVP around the two existing percentile-analysis endpoints.

The primary page provides market-scanning inputs for analysis period, range percentile, and minimum range. A submitted scan displays the backend response as a dense, single-line-per-instrument table on every viewport. Each row shows the exact symbol returned by the backend, its calculated range percent, and candle count. Selecting a row opens a dedicated instrument-analysis page using the same period and percentile. On that page, the user can change those two values and explicitly recalculate the instrument.

TanStack Router owns navigation and committed URL state. TanStack Query owns request state and in-memory response caching so navigation back from an instrument restores the previous scan during the current app session. Mantine supplies the shared application shell, theme, forms, loading states, table, and globally mounted notifications. Telegram Mini App APIs provide signed authentication data, application readiness, expansion, safe-area integration, and native back navigation.

## User Stories

1. As an authorized Telegram user, I want to open the scanner inside Telegram, so that I can use the backend without a separate login flow.
2. As an unauthorized visitor, I want to see a clear blocking state, so that I understand that the application must be opened through Telegram.
3. As a user, I want the Mini App to signal readiness to Telegram and expand to the available height, so that the interface uses the mobile viewport effectively.
4. As a user on a device with safe areas, I want controls and content to avoid obstructed screen regions, so that the application remains usable.
5. As a user, I want to see whether the backend and synchronized market data are ready, so that I know whether analysis can run.
6. As a user, I want analysis actions disabled while the backend is not ready, so that I do not submit requests that cannot succeed.
7. As a user, I want backend readiness to be checked when the application opens, so that the displayed state is immediately useful.
8. As a user, I want backend readiness to be refreshed every 30 seconds, so that the interface recovers automatically when synchronization completes.
9. As a user, I want the market-scanning page to be the primary application page, so that the main workflow is immediately available.
10. As a user, I want the analysis period to default to 30 days, so that I can run a useful scan without configuring every field.
11. As a user, I want to enter a custom integer analysis period, so that I can choose any backend-supported period from 1 through 3650 days.
12. As a user, I want the range percentile to default to 80, so that the initial scan focuses on relatively sustained daily ranges.
13. As a user, I want to enter an integer range percentile from 0 through 100, so that I can tune the analysis without unsupported fractional input.
14. As a user, I want the minimum range to default to 3%, so that the initial result filters out smaller daily ranges.
15. As a user, I want to enter a non-negative minimum range in increments of 0.1, so that I can tune the market threshold precisely enough for the MVP.
16. As a user, I want validation feedback before a request is sent, so that malformed criteria do not produce avoidable API errors.
17. As a user, I want a scan to run only after I press the scan button, so that opening the application or editing a field does not launch an expensive market-wide request.
18. As a user, I want the scan button disabled while form values are invalid, so that only supported requests can be submitted.
19. As a user, I want the scan button to display a loading state and reject repeated activation during a request, so that duplicate concurrent scans are not created.
20. As a user, I want submitted criteria reflected in the URL only after a valid submission, so that the URL always describes the displayed result rather than unfinished form input.
21. As a user, I want to see the count of matched instruments, analyzed instruments, and instruments with insufficient data, so that I understand the scope and limitations of the result.
22. As a user, I want each matching instrument displayed as one compact table row on mobile and desktop, so that large results remain easy to scan.
23. As a user, I want each row to show the exact backend `symbol`, so that frontend formatting cannot corrupt or reinterpret an instrument identifier.
24. As a user, I want each row to show `range_percent`, so that I can compare how strongly instruments met the selected criterion.
25. As a user, I want each row to show `candle_count`, so that I can see how many closed daily candles contributed to the result.
26. As a user, I want range percentages formatted compactly with three significant digits, so that small and large values remain readable without arbitrary fixed decimal places.
27. As a user, I want result ordering to remain exactly as supplied by the backend, so that the highest calculated ranges remain at the top.
28. As a user, I want to filter the current result locally by exact or partial symbol text, so that I can find an instrument in a long table quickly.
29. As a user, I want local symbol filtering to affect only the current scan result, so that it does not imply a backend-wide instrument catalog or search capability.
30. As a user, I want to select a table row to open that instrument's analysis page, so that I can inspect a matching instrument further.
31. As a user, I want the instrument page to inherit the scan's period and percentile, so that its initial analysis corresponds to the criteria that produced the table row.
32. As a user, I want the instrument page to show the exact backend symbol, calculated range percent, candle count, and analysis coverage dates, so that I can inspect all supported single-instrument output.
33. As a user, I want coverage dates displayed as short dates with an explicit UTC label, so that the dates reflect Binance daily candle boundaries without local-time ambiguity.
34. As a user, I want to change the period and percentile on the instrument page, so that I can recalculate the same instrument under different supported assumptions.
35. As a user, I want instrument recalculation to occur only after I press a button, so that intermediate numeric input does not trigger requests.
36. As a user, I want editing and recalculating an instrument not to alter the criteria or contents of my saved market scan, so that I can return to the original context.
37. As a user, I want the instrument URL to contain its symbol, period, and percentile, so that refreshing the page restores the analysis.
38. As a Telegram user, I want the native Telegram back button on the instrument page to return me to the scan, so that navigation follows Mini App conventions.
39. As a browser-based developer, I want an ordinary Mantine back control when the Telegram back-button API is unavailable, so that local navigation remains testable.
40. As a user, I want returning from an instrument to restore the previous table and scroll context from memory, so that navigation does not discard my work.
41. As a user, I want an explicit scan with unchanged criteria to fetch fresh data, so that the in-memory cache never overrides my intent to refresh.
42. As a user, I want the previous result to remain visible beneath a loading overlay during a refresh, so that the interface does not jump or lose useful context.
43. As a user, I want a clear loader when no prior result exists, so that initial request progress is visible.
44. As a user, I want errors displayed as temporary global notifications, so that they are noticeable without permanently changing the document layout.
45. As a user, I want authentication, authorization, insufficient-data, unavailable-market-data, validation, and unexpected errors translated into understandable English messages, so that I know what action is possible.
46. As a user, I want the notification system mounted once for the whole application, so that errors behave consistently on every page.
47. As a user, I want a compact global header with a `CS` mark and backend status, so that persistent chrome uses minimal mobile space.
48. As a user, I want a consistent fixed dark theme, so that every page and state feels like one application.
49. As a developer, I want one command to generate and privately save fresh development init data, so that local protected API requests work without manual copying or browser-exposed secrets.
50. As a developer, I want the frontend to call relative API paths, so that Vite can proxy development requests and production can use a same-origin reverse proxy without CORS configuration.
51. As a developer, I want form drafts, committed navigation state, and server state to have separate owners, so that editing, navigation, and caching do not interfere with one another.
52. As a developer, I want all frontend behavior to stay within existing backend capabilities, so that the UI never advertises price, volume, arbitrary symbol discovery, synchronization controls, or trading functions that do not exist.

## Implementation Decisions

- The implementation uses the existing React, Vite, TanStack Router, and Mantine frontend as its foundation.
- Add TanStack Query for server-state requests, deduplication, request status, and in-memory session caching. Do not add Zustand or another global state manager.
- Add Mantine Form for form state and validation.
- Add Mantine Notifications. Mount its provider-level notifications renderer once in the shared application root and invoke the library's global notification API from any route or request boundary.
- Add `@tma.js/init-data-node` as development-only tooling for generating correctly signed local Telegram init data. Do not implement Telegram signing logic by hand and do not ship this package as client runtime authentication.
- Use one Mantine `AppShell` around every route. Its compact header contains `CS` on the left and backend readiness on the right. The instrument route's browser-only back control belongs in page content rather than expanding the global header.
- Configure a fixed dark color scheme through `MantineProvider` and `createTheme`. Colors, radii, typography, spacing, variants, and component customization must use Mantine theme tokens, Mantine CSS variables, component props, and the official Styles API. Do not globally override Mantine component internals with ad hoc CSS or hardcoded colors.
- Custom CSS may be used only where layout or application-specific structure is not adequately represented by Mantine props or the Styles API. Such CSS must consume theme variables instead of duplicating visual constants.
- The interface is English-only for the MVP. Do not add an internationalization framework. Number formatting uses an English locale.
- The application initializes the Telegram Mini App once at the root by calling the supported `ready()` and `expand()` methods. It incorporates Telegram/device safe-area values into the `AppShell` layout. It does not request Telegram full-screen mode.
- In production, business pages require non-empty signed `Telegram.WebApp.initData`. When it is absent, show a blocking "Open in Telegram" state rather than allowing requests that will inevitably receive `401`.
- API requests authenticate with `Authorization: tma <raw initData>`. The frontend never uses a standalone Telegram user ID as proof of identity.
- For local development only, provide a package command that reads `TELEGRAM_BOT_TOKEN` and `ADMIN_TELEGRAM_ID` from the repository environment, uses `@tma.js/init-data-node` to generate fresh signed init data with the current authentication date, and upserts it as `TELEGRAM_DEV_INIT_DATA` in the gitignored root `.env.local`.
- The generator must not print the bot token or generated credential. It preserves unrelated local environment entries, creates or updates the private file with owner-only permissions, and reports only successful generation, the configured validity period, and the need to restart Vite.
- Vite reads `TELEGRAM_DEV_INIT_DATA` only in its server process and injects `Authorization: tma <value>` into proxied business requests that do not already provide authorization. The value must not use a `VITE_` prefix, enter `import.meta.env`, appear in browser code, or enter the production bundle. Real Telegram authorization always takes precedence.
- The backend remains stateless and validates raw init data on every business request. The required `TELEGRAM_INIT_DATA_MAX_AGE` environment variable is the single source of truth for credential validity and is configured as `24h` for this application. No backend session, access token, refresh token, or automatic credential renewal is introduced.
- All client API calls use relative `/api/v1/...` and `/health/...` paths. Vite proxies them to `127.0.0.1:8080` during development. Production reverse-proxy and Nginx work are outside this spec.
- Readiness uses `GET /health/ready`, runs immediately, and refetches every 30 seconds. Its status is visible in the global header. Analysis submit actions are disabled unless readiness succeeds.
- The primary route is market scanning. It contains three numeric controls: integer analysis period with default 30 and bounds 1–3650; integer percentile with default 80 and bounds 0–100; non-negative minimum range with default 3 and step 0.1.
- Forms use Mantine Form validation. Invalid values prevent submission and expose field-level validation. No form setting or result is persisted to `localStorage`, `sessionStorage`, IndexedDB, cookies, or a custom persistent store.
- Form inputs represent draft values. Confirmed scan criteria are written to TanStack Router search parameters only on valid submit. Search parameters must be parsed and validated at the route boundary.
- The market request calls the existing market percentile endpoint with `period_days`, `percentile`, and `minimum_range_percent`. The client models and displays `matched_count`, `analyzed_count`, `insufficient_data_count`, and every returned item's `symbol`, `range_percent`, and `candle_count`.
- The result uses a Mantine table at every breakpoint. It does not switch to cards on mobile. Each instrument occupies one row with exactly three data columns: symbol, range percent, and candle count.
- Symbols are rendered byte-for-byte as returned by the backend. Do not insert separators, remove quote assets, relabel them as coins, or otherwise transform identifiers.
- Range percent is formatted for display with `Intl.NumberFormat` and three significant digits. The original numeric value remains unchanged in cached data and is not re-rounded before comparison or ordering.
- Preserve backend ordering. Do not add interactive column sorting in the MVP.
- Add a compact client-side symbol filter above the result. It filters only the returned rows and is not stored in URL or persistent storage. It must not call the backend or expose instruments outside the scan result.
- Selecting a row navigates to a dedicated instrument route. Manual symbol entry, global symbol search, autocomplete, and a complete active-instrument catalog are not implemented because the current backend does not expose a catalog endpoint.
- The instrument route path contains the exact symbol. Its validated search parameters contain the committed period and percentile, allowing refresh to reconstruct the request.
- The instrument page initially receives the period and percentile used by the originating scan. It calls the existing single-instrument percentile endpoint and displays symbol, range percent, candle count, and coverage dates.
- Coverage boundaries are formatted as short calendar dates with a visible UTC label. The UI does not convert Binance daily candle boundaries to the user's local time and does not include a timezone switcher.
- The instrument page has Mantine Form inputs for integer period and integer percentile. Submitting them commits new instrument search parameters and recalculates. These changes do not mutate the cached market-scan key or result.
- TanStack Query keys are derived from committed, validated route parameters. Returning through Router navigation restores the market response from Query's in-memory cache without an automatic request.
- An explicit market form submission always represents a refresh intent. When submitted criteria equal the current committed criteria, explicitly refetch instead of silently returning only cached data.
- During a first load with no data, show a Mantine loader. During a refresh with prior data, retain that data and show a Mantine `LoadingOverlay`. Submit buttons use Mantine's built-in loading state and are disabled when invalid, already pending, unauthenticated, or not ready.
- Preserve the scan route and its cached response when opening an instrument. Router-managed history and scroll restoration return the user to the previous scan and position during the same application session.
- Inside Telegram, show and subscribe to the native `BackButton` only while the instrument route is active. Its handler performs a TanStack Router history transition. Remove the handler and hide the button on cleanup. Outside Telegram, show a normal Mantine back control.
- Normalize backend error envelopes in one API-client boundary. Route code triggers globally mounted Mantine notifications with English messages appropriate to `unauthenticated`, `access_denied`, `symbol_not_found`, `insufficient_data`, `market_data_unavailable`, validation failures, network failures, and unexpected errors.
- Notifications float outside document flow, auto-close after a reasonable duration, and use themed Mantine variants. Do not add route-specific alert components for request errors.
- Existing backend routes, response schemas, stateless Telegram authentication, synchronization, database schema, and analysis calculations are unchanged by the frontend feature beyond the separately agreed 24-hour init-data maximum age.
- No ADR is required: the agreed frontend library choices follow the project's existing TanStack/Mantine direction and do not create a surprising system-wide irreversible boundary.

## Testing Decisions

- A good automated test in this feature verifies observable input/output behavior at a stable boundary. It does not inspect hook internals, component implementation details, Mantine DOM structure, CSS class names, or snapshots.
- Use Vitest as the test runner for pure frontend logic only. Do not add React Testing Library.
- Prefer one high API/client-state seam: given validated route or form values and a mocked HTTP response, verify the emitted URL, query parameters, authorization behavior where applicable, parsed success value, and normalized error outcome.
- Test form and route validation as pure functions: accepted defaults and boundaries, rejection of non-integer period/percentile values, rejection of out-of-range values, and acceptance of non-negative decimal minimum ranges.
- Test search-parameter parsing and serialization so committed URLs reconstruct the same request criteria and invalid URL state falls back or fails according to the route contract.
- Test market API mapping against the existing backend response shape, including counts, ordered items, empty results, and canonical error envelopes.
- Test single-instrument API mapping, exact symbol preservation, URL encoding, UTC coverage strings, and canonical errors.
- Test the percentage display formatter with zero, very small decimals, values around one, larger values, and three-significant-digit rounding.
- Test local symbol filtering as a pure function, including empty filter, case handling, partial matches, no matches, and preservation of backend order.
- Test development proxy header construction as isolated configuration logic: missing or blank development init data produces no authorization header, while a configured value produces the exact `tma` scheme. Never assert or snapshot a real credential.
- Test the development generator through deterministic pure seams where practical: required environment validation, positive safe Telegram ID parsing, preservation/upsert behavior for the private environment file, and use of the current authentication date. Test fixtures use fake tokens and generated values only.
- Existing backend HTTP handler tests are prior art for endpoint success and canonical error envelopes. Frontend tests should consume that published contract rather than duplicate backend percentile calculations.
- UI rendering, responsive table behavior, `AppShell` composition, loaders, overlays, notification portals, Telegram BackButton integration, navigation restoration, and safe-area behavior are verified manually in a real browser and, where available, Telegram's Mini App environment.
- Manual acceptance must cover initial readiness, blocked unauthenticated state, successful scan, empty scan, local filtering, row navigation, instrument recalculation, native/browser back behavior, cached return, explicit same-criteria refresh, API error notification, mobile width, and production build output.
- Do not add snapshot tests, React Testing Library, jsdom-based component behavior tests, or Playwright in this MVP. Browser automation can be introduced later when end-to-end coverage justifies its maintenance cost.

## Out of Scope

- Further backend changes beyond the already agreed 24-hour init-data maximum age, including sessions, new routes, catalog endpoints, price data, volume data, synchronization controls, authentication redesign, analysis formulas, schemas, or migrations.
- Displaying current instrument price or any value absent from the current API response.
- Browsing, searching, or autocompleting every active backend instrument independently of a scan result.
- Manually entering a symbol to reach the instrument page from the user interface.
- Client-side result sorting or additional filters beyond local symbol filtering.
- Card-based result layouts, charts, candlestick visualization, watchlists, favorites, portfolio features, balances, account access, or order placement.
- Automatic market scanning on app open or on input change.
- Persisting criteria, filters, results, or cache across Mini App sessions.
- Light theme, theme switching, user-configurable appearance, or automatic system-theme selection.
- Internationalization, language selection, and non-English interface copy.
- Local-time conversion or a UTC/local timezone switcher.
- Toast or alert systems other than the globally mounted Mantine Notifications integration.
- React Testing Library, component snapshots, Playwright, or another end-to-end test harness.
- Production Nginx configuration, frontend containers, hosting, deployment, Telegram Bot API launch UX, webhook handling, or Mini App registration.
- Full-screen Telegram mode or additional Telegram primary/secondary action buttons.
- Automatic rotation of development init data while Vite is running; the developer explicitly regenerates it and restarts Vite when the configured 24-hour credential expires.

## Further Notes

- The market endpoint is not a complete instrument-catalog endpoint. A minimum range of zero returns all analyzable active instruments for the selected period, but instruments with insufficient history are still omitted and reported only in `insufficient_data_count`.
- Initial market synchronization backfills up to 30 closed daily candles. The frontend may accept periods above 30 because the backend supports up to 3650 and retained history grows over time, but fresh deployments can legitimately report many instruments as insufficient for longer periods.
- Daily candles use Binance UTC boundaries by domain definition. The UTC label is therefore semantic, not merely a display preference.
- The frontend should call the data items "instruments" or "symbols", not "coins", because identifiers such as `BTCUSDT` represent trading pairs.
- The existing domain glossary is the source of truth for the terms Market Scan, Instrument Analysis, Daily Range, Daily Candle, Analysis Period, Range Percentile, Minimum Range, and Scan Result.
- Local development authentication has been verified end to end: a credential generated by the package command, loaded from the private environment by a restarted Vite proxy, was accepted by the unchanged backend on a protected Instrument Analysis request.
