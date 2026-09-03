# Instrument Detail Statistics, Bot Grid Steps, and Seven-day Chart

## Problem Statement

The instrument detail page currently shows little beyond the symbol and market capitalization. A user who opens an instrument from Market Scan loses sight of its daily and hourly range statistics, the amount of history supporting them, and its recent price movement. The user also has to calculate the trading bot's hourly and daily Grid Steps manually.

The page should bring those existing analysis results together and make the recommended bot settings easy to identify, without introducing a different chart or a second set of analysis settings.

## Solution

Present compact statistics at the top: symbol, existing Market Cap, Daily Range, Hourly Range, and available daily and hourly history relative to the requested sample sizes.

Show a prominent separate block titled “Рекомендованные настройки торгового бота”, with “Шаг сетки — часовой” and “Шаг сетки — дневной”. Each Grid Step is half the corresponding range percentage. Format these percentages adaptively and identify incomplete samples or unavailable calculations.

Below the information blocks, display a larger version of the existing Seven-day Price History chart. Retain exactly the table chart's behavior and hourly closing-price data semantics, without interval selection, scrolling into older history, or new interaction.

## User Stories

1. As a scanner user, I want the instrument symbol to remain clearly visible, so that I know which instrument I am examining.
2. As a scanner user, I want to retain the existing Market Cap information when available, so that I can assess the instrument's size alongside its volatility.
3. As a scanner user, I want to see Daily Range on the instrument page, so that I do not have to return to the results table for its daily volatility value.
4. As a scanner user, I want to see Hourly Range on the instrument page, so that I can assess its hourly volatility in the same place.
5. As a scanner user, I want the detail statistics to use the scan settings carried into the page, so that the daily and hourly values retain their intended meaning.
6. As a scanner user, I want the daily history count labeled “Дней доступно”, so that I understand how much daily history supports the analysis.
7. As a scanner user, I want the hourly history count labeled “Часов доступно”, so that I understand how much hourly history supports the analysis.
8. As a scanner user, I want each count shown relative to the requested sample size, such as “20 из 30”, so that I can distinguish a complete sample from a partial one.
9. As a scanner user, I want the summary to be compact and near the top, so that I can read the essential statistics quickly on a phone.
10. As a bot operator, I want a clearly separated block named “Рекомендованные настройки торгового бота”, so that I can immediately find the proposed settings.
11. As a bot operator, I want the hourly Grid Step to equal Hourly Range divided by two, so that I do not have to calculate it manually.
12. As a bot operator, I want the daily Grid Step to equal Daily Range divided by two, so that I can compare the two settings.
13. As a bot operator, I want both recommended Grid Steps visible together, so that I do not have to switch modes to read them.
14. As a bot operator, I want each recommendation labeled as hourly or daily and expressed as a percentage, so that I know which setting I am reading.
15. As a bot operator, I want Grid Step to mean spacing between adjacent grid orders, so that I do not confuse it with the total grid span.
16. As a bot operator, I want the calculation to use the original range value before display rounding, so that the recommendation does not compound rounding errors.
17. As a bot operator, I want large percentages displayed with fewer decimal places and small percentages with more, so that values remain readable without losing their scale.
18. As a bot operator, I want a recommendation from a partial sample to remain visible with an incomplete-sample indication, so that I can see both the calculated value and its supporting history.
19. As a bot operator, I want “Недостаточно данных” instead of an unavailable recommendation, so that missing information is not presented as a zero setting.
20. As a scanner user, I want a larger Seven-day Price History chart on the instrument page, so that I can inspect the same recent trend more easily.
21. As a scanner user, I want that chart to use the same hourly closing-price semantics as the results table, so that opening an instrument does not introduce a different chart interpretation.
22. As a scanner user, I want chart data read from the application's database, so that viewing the page does not require an on-demand exchange history download.
23. As a scanner user, I want rising, falling, and unchanged histories to retain the table's existing colors, so that the visual meaning stays consistent.
24. As a scanner user, I want missing history to retain its correct position and gaps, so that a short or interrupted history is not stretched or presented as continuous.
25. As a scanner user, I want a single observation and an empty series handled just as in the table, so that limited history remains understandable.
26. As a scanner user, I want valid statistics and recommendations to remain available when the seven-day series contains no prices, so that missing chart history does not remove useful analysis.
27. As a mobile scanner user, I want the enlarged chart and information blocks to fit the page, so that I can read them without horizontal page overflow.
28. As a scanner user, I want existing back navigation, loading, authentication, and error behavior preserved, so that the expanded page remains part of the same workflow.

## Implementation Decisions

- Use Grid Step as the domain term for percentage spacing between adjacent orders. The hourly and daily recommendations are derived independently from their corresponding Volatility Criterion Instances.
- Use the existing keyed daily and hourly evaluations, not evaluation array positions. Preserve the configured periods and percentiles passed from Market Scan. Do not hard-code the default 30 daily candles, 60 hourly candles, or 80th percentiles into the page's calculations.
- Daily Range and Hourly Range already represent the configured percentile of individual candle high-low ranges relative to their opening prices. Do not introduce another definition of volatility or calculate it from the seven-day chart.
- Display actual candle counts as “Дней доступно: A из R” and “Часов доступно: A из R”, where A is the available sample used by the evaluation and R is its requested sample size. A is capped by R; these labels do not describe the total stored history. Do not invent a count when an evaluation did not run or its count is unavailable.
- Arrange the page as compact statistics, a visually prominent bot-settings block, and an enlarged chart. Preserve the existing symbol, Market Cap availability behavior, responsive layout, and navigation. New labels and the bot-settings heading use the agreed wording; broader localization is outside this feature.
- Compute each Grid Step from the unrounded range percentage divided by two. Reuse the existing percentage formatter with a maximum of three significant digits through Intl.NumberFormat. Omit unnecessary trailing zeros. Calculated steps of 12.345, 1.2345, 0.12345, and 0.012345 display as 12.3%, 1.23%, 0.123%, and 0.0123% respectively.
- Continue showing calculable recommendations when fewer candles are available than requested, accompanied by an incomplete-sample indication near the affected recommendation. Keep the two recommendations independent. Show “Недостаточно данных” for a recommendation that cannot be calculated; missing or unevaluated values must not silently become zero. A valid numerical zero remains distinct from an unavailable value.
- Preserve the ordered Analysis Pipeline and existing short-circuit behavior. A later evaluation can be absent after an earlier criterion rejects the instrument; present this unavailable state defensively rather than treating the omission as a valid metric or changing market-selection rules.
- Reuse the existing instrument-analysis service, HTTP response, frontend response validation, and query flow. Extend a successful instrument-analysis response with price_history and price_history_window using the same representations as Market Scan. Preserve authentication and canonical error semantics, including the insufficient-history response, and map that outcome to the agreed unavailable recommendation state rather than a fabricated result.
- Build Seven-day Price History with the existing database-backed history logic. Freeze the window at the start of the analysis request, ending at the last completed UTC market hour. It contains 169 consecutive hourly closing-price slots spanning 168 hours between observations. Exclude the open hour; do not move the window backward for stale instruments or align it with volatility lookback periods.
- Keep missing observations as null slots, including leading, internal, and trailing gaps. An all-missing series is a valid empty chart history and does not invalidate otherwise available analysis. Do not disguise a database or transport failure as an empty history.
- Reuse the current lightweight chart implementation, allowing display dimensions to differ between the table and detail page. Retain the table's current dimensions and behavior. Make the detail chart responsive and larger; do not add a second chart library or a separate chart implementation.
- Preserve the chart's unrounded endpoint comparison, green/red/gray direction colors, individual vertical scaling, linear segments, gap positioning, isolated-point placement, accessible description, and empty dash. Do not add axes, fill, tooltips, interval controls, zoom, panning, or navigation into older history.
- Preserve the current detail page's criteria propagation and analysis request lifecycle. Its analysis may be newer than a previously displayed scan; matching chart behavior does not require pinning the detail page to an earlier table response. Do not introduce independent chart refresh, live prices, or on-demand exchange fetching.
- Use existing candle storage and history reads; no schema migration, retention change, historical backfill expansion, or synchronization redesign is required.

## Testing Decisions

The following testing boundaries are proposed for the required to-spec confirmation before publication:

- Use two existing high-level boundaries: the rendered instrument-analysis screen and the public instrument-analysis HTTP API. A single existing test harness does not span both frontend and Go backend; do not introduce a new end-to-end framework merely to combine them.
- At the screen boundary, supply representative analysis results and scan criteria through the existing query-provider setup, following the current instrument-page render tests. Assert visible labels, original-value Grid Step calculations, adaptive percentage formatting, actual/requested counts, incomplete-sample indications, unavailable recommendations, and chart presence. Include non-default periods and reordered daily/hourly evaluations to catch hard-coded defaults or positional coupling.
- Exercise rounding cases at different scales, including a case that would differ if the input were rounded before division. Check that a small positive percentage is not rounded to zero and that a genuine zero differs from missing data.
- Cover a short-circuited pipeline result and the existing insufficient-history response at the relevant public boundaries. Verify unavailable recommendations without discarding other valid content returned by the API or misclassifying unrelated failures as insufficient data.
- At the HTTP boundary, follow the existing authenticated analysis tests and seven-day market-history tests, which run the real analysis service with a controlled store. Verify the instrument response's symbol, evaluations, history metadata, 169 chronological slots, exclusion of the open hour, and fixed request-start window, including an hour/day boundary and delayed analysis.
- Use the API boundary to check short, empty, isolated, and gapped price histories. Missing chart observations must preserve their slots and must not remove otherwise valid evaluations. Retain the existing authentication, validation, and canonical error tests.
- Retain the existing shared-chart regression suite as prior art for endpoint colors, unrounded comparisons, leading/internal/trailing gaps, isolated points, and the empty dash. Extend it only where needed to establish that enlarged dimensions preserve those observable semantics and that the table's dimensions remain unchanged. Do not duplicate the entire history algorithm in a new set of helper tests.
- Good tests verify observable screen output and API contracts. Avoid coupling tests to component nesting, internal helper calls, or private calculation steps; use the highest existing boundary that proves each behavior.
- During implementation, run make check for the cross-cutting change. Perform visual checks in the Codex built-in browser at mobile and desktop sizes for compactness, readable recommendations, chart sizing, absence of page overflow, and unchanged table presentation.

## Out of Scope

- Implementing this feature as part of specification preparation or publication.
- Decomposing the specification into implementation tickets in this step; that is the subsequent to-tickets phase.
- Automatic bot configuration, exchange orders, bot execution, strategy optimization, grid boundaries, or additional trading parameters.
- Replacing Grid Step with total grid width or an offset from a reference price.
- Changing the Volatility Criterion calculation, Analysis Pipeline eligibility or ordering, or the existing scan settings workflow.
- Showing the total number of stored daily/hourly candles outside the requested analysis samples.
- Daily/hourly chart switching, older-history scrolling, candlestick charts, live prices, open candles, independent chart refresh, or new chart interaction.
- Changing chart behavior on the Market Scan table, adding another charting library, or introducing a general charting framework.
- New exchange data sources, storage schema changes, retention policies, or synchronization/backfill expansion.
- Broad localization, unrelated page redesigns, and a new end-to-end testing framework.

## Further Notes

- Product requirements were confirmed through grill-with-docs. The glossary records Grid Step. No ADR is needed for these routine, reversible changes.
- Current status: specification drafted; awaiting the to-spec testing-boundary confirmation. Application implementation has not started.
- Publish this specification as one GitHub issue in Zver64/crypto_scaner with the ready-for-agent label after the testing-boundary check. This is the feature specification, not a decomposition into implementation tickets.
- The existing seven-day chart was delivered by [Integrate Microcharts into the frontend — #18](https://github.com/Zver64/crypto_scaner/issues/18) and [Add seven-day price charts to analysis results — #19](https://github.com/Zver64/crypto_scaner/issues/19). Both are closed; this feature reuses their behavior.
- The next phase is to-tickets, followed by implementation of selected tickets only when requested by the user. A ready-for-agent label does not itself authorize starting implementation in this conversation.
