# Seven-day Charts in Analysis Results

Status: approved and published as GitHub issues. Implementation is deferred at the user's request.

- Task 1: [Integrate Microcharts into the frontend — #18](https://github.com/Zver64/crypto_scaner/issues/18).
- Task 2: [Add seven-day price charts to analysis results — #19](https://github.com/Zver64/crypto_scaner/issues/19), blocked by #18.

## Confirmed Requirements

- Use `@microcharts/react`: https://microcharts.dev/docs/quickstart.
- Add a seven-day price chart for each instrument in the results table, immediately before the Binance link column.
- Use a compact line chart, visually similar to the seven-day charts in CoinMarketCap's market table.
- Show a rolling seven-day window with hourly observations. All instruments share the same time window, ending at the last completed UTC hour when the analysis starts. Do not include the currently open hour or move the window backward for an instrument with stale data.
- A complete series contains 169 consecutive hourly closing prices, spanning exactly 168 hours between its first and last observations.
- Render whatever history exists within the seven-day window. If history starts partway through the window, leave the preceding part blank and begin the chart at the actual start of the available data. Do not stretch a partial history to fill the chart width or hide it solely because it is incomplete.
- The backend is responsible for complete history wherever exchange data exists. As a defensive fallback, missing observations remain blank: internal gaps break the line and missing recent observations leave empty space on the right. Never replace missing prices with zero or connect the line across gaps.
- Color the entire line red when its ending price is below its starting price, and green when its ending price is above its starting price.
- Use gray when the starting and ending prices are equal; compare prices before display rounding.
- Fit the vertical scale independently for each instrument. Equal visual heights across rows do not imply equal percentage changes.
- Render a static line without point interaction, axes, value labels, or fill. Preserve existing row navigation when the chart is clicked.
- Show the chart on both desktop and mobile, approximately 140 by 40 pixels, using the table's existing horizontal scrolling.
- Refresh charts with a new analysis run. Do not refresh them independently while results remain on screen.
- Ensure sufficient hourly history is loaded from the exchange for the seven-day window.
- Prepare at least two separate implementation tasks: library integration and the chart feature.

## Repository Findings

- The shared results table is `frontend/src/features/market-scan/results-view.tsx`. Rows navigate to instrument analysis; the Binance link stops event propagation.
- The table already supports horizontal scrolling on small screens.
- The frontend uses React 19 and npm. Microcharts documents support for React 18/19 and requires a single stylesheet import at the application entry point.
- Market data comes from Binance Spot USDT instruments. Both daily and hourly candles are synchronized, and only closed candles are persisted.
- Hourly synchronization runs at startup and at each UTC hour plus 30 seconds. Completion can occur later.
- Initial hourly synchronization requests only 60 candles. Subsequent synchronization is incremental; no candle retention policy is implemented. A full seven-day hourly history cannot be assumed for every instrument.
- The analysis results API returns criterion evaluations, without candle price series. Current candle reads are scoped to one instrument and interval.
- These findings describe the code, not verified contents or freshness of a running database.

## Task 1: Integrate Microcharts into the Frontend

### Problem and Outcome

The frontend needs the charting dependency and styles before the results-table feature can use Sparkline. Integrate `@microcharts/react` with the existing React 19 application.

### Scope

- Install the current compatible `@microcharts/react` package using npm, recording the resolved version in `frontend/package-lock.json`.
- Import `@microcharts/react/styles.css` once in `frontend/src/main.tsx`.
- Use the static `@microcharts/react/sparkline` entry for the feature; no interactive entry or motion setup is needed.
- Check rendering in the application's dark theme with a temporary local example. Do not leave a demonstration chart in the product UI.
- This task does not change market synchronization, the analysis API, or the results table.

### Acceptance and Verification

- A clean dependency installation resolves the package from the committed manifest and lockfile.
- A minimal Sparkline renders with the imported stylesheet and existing theme.
- Run `npm -C frontend run quality`, `npm -C frontend run test`, and `npm -C frontend run build`.

## Task 2: Add Seven-day Price Charts to Analysis Results

Dependency: task 1.

### Problem and Outcome

Users need to see recent price movement directly beside each instrument in the results table. Add a static seven-day chart before the Binance link, backed by synchronized hourly closing prices and the confirmed behavior above.

### Backend Scope

- Ensure initial hourly synchronization can load at least the 169 closing prices required for a complete chart window.
- Backfill existing instruments with insufficient history. Merely increasing the initial request limit will not repair existing instruments because their synchronization is incremental.
- Detect and repair missing hourly candles within the required history window where the exchange provides them. Newly listed instruments may legitimately have shorter histories.
- Retain idempotent upserts, exchange request limits, and the exclusion of open candles. The existing database has no 60-row cap; changing history depth does not inherently require a schema migration.
- Read historical prices for the fixed analysis window from PostgreSQL and expose them to the results table. Chart history is presentation data, independent of the selected criteria and their lookback periods.
- Missing chart history must not exclude an instrument that otherwise passes the analysis criteria.

### Frontend Scope

- Insert a `7d` column immediately before Binance in the shared results table.
- Render the static Sparkline with the fixed hourly time positions and an individual vertical scale.
- Use green, red, or gray according to the first and last available prices within the window, before display rounding. All segments of a partial chart use that same direction color.
- Preserve empty positions before, inside, and after available history. Keep partial histories aligned with the complete seven-day window.
- Use approximately 140 by 40 pixels, no axes, labels, fill, animation, or point interaction. Keep the chart visible on mobile and preserve horizontal scrolling and row navigation.
- Keep charts attached to the displayed analysis results and refresh them only when analysis runs again.

### Implementation Decisions

- Fix the common window once per analysis request. Represent 169 hourly slots in chronological order, with absent observations represented as `null` and enough common time metadata to identify each slot.
- Deliver chart series in the market-analysis response so results and charts update together. Prefer a bounded query over the returned instruments over one additional query or browser request per table row.
- Perform exchange fetching and repair in synchronization, not while rendering or requesting an analysis result.
- Show a neutral point at its actual time position if only one valid price exists; show `—` when none exist. An isolated point must not be recentered within the chart.
- Give each chart an accessible description without adding interactive controls.

### Acceptance and Verification

- A complete chart has 169 prices spanning 168 hours; the open hour is excluded, including around UTC day boundaries.
- All rows use identical time boundaries, including when synchronization is delayed for one instrument.
- A three-day history occupies its actual final three days of the window, leaving the earlier area blank.
- Missing internal hours produce gaps; stale data leaves blank space on the right; zero and one available observation have defined behavior.
- Rising, falling, and unchanged endpoint prices produce green, red, and gray respectively. Intermediate peaks do not change the direction color.
- Initial synchronization loads sufficient history, existing short histories are backfilled, and recoverable internal gaps are repaired without duplicating candles or storing open candles.
- Results and chart availability remain independent: an instrument can pass its criteria with partial or missing chart history.
- Verify the chart in the existing dark theme on desktop and mobile, including row navigation and the neighboring Binance link.
- Add behavioral tests for synchronization/backfill, the API's common time window and missing slots, and chart direction/placement edge cases. Run `make check` for this cross-cutting change.

## Out of Scope

- Live prices or the currently open hour.
- Independent background chart refresh, point tooltips, zoom, and chart-specific navigation.
- Alternative chart periods, futures price feeds, and new analysis criteria.
- A general-purpose charting abstraction or a permanent demonstration page.

## Documentation

- `CONTEXT.md` records Seven-day Price History as a domain term.
- No ADR is required for the current decisions: dependency integration, hourly history depth, and the table presentation are routine and reversible.
- Both task descriptions are published to GitHub Issues. Implementation requires a separate instruction from the user.
