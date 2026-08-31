# USD(S)-M Isolated Liquidation Price

## Problem Statement

As an operator of trading bots, I need a reliable USD(S)-M futures liquidation-price calculation before the future coin-page form exists. I need to estimate the liquidation price of a long or short isolated Position from the same market and risk-bracket inputs used by Binance, so I can configure bots without manually repeating calculations in Binance Futures Calculator.

## Solution

Add one client-side, pure USD(S)-M isolated Position liquidation-price calculator. It accepts the Position direction, entry price, base-asset quantity, leverage, and Binance maintenance-margin brackets; returns an unrounded estimated liquidation price; and rejects invalid or internally inconsistent inputs.

The calculator must select the maintenance-margin bracket that applies at its computed liquidation price rather than assuming the entry-price bracket. It must be usable later by a coin-page form and bot-configuration workflow, but this work introduces neither UI nor backend/API changes.

## User Stories

1. As a trading-bot operator, I want to calculate the estimated liquidation price of an isolated USD(S)-M long Position, so that I can assess downside risk before configuring a bot.
2. As a trading-bot operator, I want to calculate the estimated liquidation price of an isolated USD(S)-M short Position, so that I can assess upside risk before configuring a bot.
3. As a trading-bot operator, I want to provide a Position's entry price, quantity, and leverage, so that the calculation reflects my planned Position.
4. As a trading-bot operator, I want the calculation to use the maintenance-margin bracket for the resulting liquidation notional, so that large Positions are not incorrectly calculated with a lower bracket.
5. As a trading-bot operator, I want the result to remain unrounded, so that a later caller can present it at the instrument's tick size without losing calculation precision.
6. As a trading-bot operator, I want invalid values to be rejected, so that a bot configuration cannot silently use a nonsensical liquidation price.
7. As a trading-bot operator, I want a zero or negative quantity, price, or leverage rejected, so that an empty or impossible Position is not treated as liquidatable.
8. As a trading-bot operator, I want non-finite numeric values rejected, so that browser number overflow and malformed form values cannot produce a misleading result.
9. As a trading-bot operator, I want a missing or non-applicable maintenance-margin bracket rejected, so that a risk estimate is never produced from arbitrary fallback rates.
10. As a trading-bot operator, I want the utility to be independent of a rendered form, so that bot calculations and future UI use one definition of liquidation price.
11. As a maintainer, I want the calculation covered by behavioral unit tests for both Position directions, so that formula regressions are caught before UI work begins.
12. As a maintainer, I want tests for bracket boundaries and bracket transitions, so that changes in Position size do not silently select the wrong maintenance requirement.
13. As a maintainer, I want Binance-calculator reference cases added when browser-accessible results are available, so that the estimate remains demonstrably close to Binance's displayed result.

## Implementation Decisions

- The first deliverable is only USD(S)-M futures liquidation for one isolated Position. The margin asset is the USD(S)-M contract's quote collateral, typically USDT.
- The Position is either `long` or `short`. Hedge-mode aggregation, mixed long and short Positions, and cross-margin accounts are not represented.
- Quantity is base-asset quantity. Entry price and returned liquidation price are quote-collateral per base asset.
- The Position has no manually added or removed isolated margin. Initial isolated margin is derived as entry notional divided by leverage.
- Binance maintenance-margin brackets are caller-provided data. Each bracket provides a notional range, maintenance-margin ratio, and cumulative maintenance-margin amount. Retrieving or caching bracket data is not part of this task.
- The calculator solves the linear USD(S)-M isolated-margin equation for each direction, selects the bracket consistent with the resulting liquidation notional, and rejects an input set for which no supplied bracket is consistent.
- The calculator uses JavaScript `number`. It does not introduce a decimal or big-number dependency, and it does not round intermediates or results.
- Funding, trading fees, liquidation fees, realized PnL, open orders, manual margin transfers, and account balances are excluded. The returned value is an estimate, not an exchange liquidation-engine guarantee.
- The public calculation function is the sole new seam. It is a pure module with no React, API, storage, or formatting dependency; future UI and bot configuration consume it rather than reimplementing its formula.
- No backend endpoint, database schema, route, generated file, or visible UI is changed.
- The domain glossary defines Position, Entry Price, and Liquidation Price. Entry Price remains distinct from average-entry and break-even calculations that will be separate future tasks.

## Testing Decisions

- Test the public calculation function's observable contract: a valid input returns the expected estimate, while invalid or bracket-inconsistent inputs are rejected. Do not test internal formula steps or bracket-iteration implementation details.
- Use Vitest unit tests, following the existing frontend pure-helper tests.
- Cover long and short Positions in the first maintenance bracket, a Position whose liquidation notional selects a later bracket, exact bracket boundaries, no matching bracket, malformed bracket definitions, and invalid numeric inputs.
- Use tolerance-based comparisons appropriate to JavaScript floating-point arithmetic. A future Binance Futures Calculator fixture is considered correct when the result is within one symbol price tick; the fixture must record its symbol, direction, entry price, quantity, leverage, isolated mode, calculated result, and collection date.
- Initial tests may use formula-derived cases with explicitly recorded bracket data. Direct Binance Calculator outputs could not be collected in this environment because the public calculator is protected by AWS WAF; add those outputs once they are obtained through a real browser.
- Run frontend quality checks, unit tests, and production build for the implementation task.

## Out of Scope

- COIN-M liquidation calculations.
- Spot, USD(S)-M, or COIN-M average-entry calculations.
- The three-tab calculation form, coin-page UI, input controls, formatting, and result presentation.
- Cross margin, auto-add margin, manual isolated-margin changes, multiple Positions, hedge-mode aggregation, and open orders.
- Binance authentication, account access, exchange metadata retrieval, risk-bracket synchronization, and persistence.
- Exact parity with Binance's private liquidation engine, including unspecified calculator rounding and fee behavior.

## Further Notes

- The planned delivery order is: USD(S)-M isolated liquidation, COIN-M isolated liquidation, Spot average entry, USD(S)-M average entry, and COIN-M average entry. Each is a separate, complete task.
- Binance documents the Position and leverage-bracket fields but does not publish the exact internal Futures Calculator rounding and fee behavior. The estimate should therefore be validated against browser-captured Binance Calculator cases, not asserted to be exactly identical.
- The public Binance calculator page is reachable without account credentials in a real browser, but this environment received an AWS WAF challenge and could not extract calculator outputs automatically.
