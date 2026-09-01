# USD(S)-M Isolated Liquidation Price

## Problem Statement

As an operator of trading bots, I need a reliable USD(S)-M futures liquidation-price calculation before the future coin-page form exists. I need to estimate the liquidation price of a long or short isolated Position from its market inputs and maintenance-margin ratio, so I can configure bots without manually repeating calculations in Binance Futures Calculator.

## Solution

Add one client-side, pure USD(S)-M isolated Position liquidation-price calculator. It accepts the Position direction, entry price, base-asset quantity, leverage, and one maintenance-margin ratio; returns an unrounded estimated liquidation price; and rejects invalid inputs.

The calculator intentionally omits Binance risk-bracket adjustments and therefore produces a conservative estimate for Positions outside the first risk bracket. It must be usable later by a coin-page form and bot-configuration workflow, but this work introduces neither UI nor backend/API changes.

## User Stories

1. As a trading-bot operator, I want to calculate the estimated liquidation price of an isolated USD(S)-M long Position, so that I can assess downside risk before configuring a bot.
2. As a trading-bot operator, I want to calculate the estimated liquidation price of an isolated USD(S)-M short Position, so that I can assess upside risk before configuring a bot.
3. As a trading-bot operator, I want to provide a Position's entry price, quantity, and leverage, so that the calculation reflects my planned Position.
4. As a trading-bot operator, I want the calculation to use one maintenance-margin ratio, so that its assumptions are explicit and easy to configure.
5. As a trading-bot operator, I want the result to remain unrounded, so that a later caller can present it at the instrument's tick size without losing calculation precision.
6. As a trading-bot operator, I want invalid values to be rejected, so that a bot configuration cannot silently use a nonsensical liquidation price.
7. As a trading-bot operator, I want a zero or negative quantity, price, or leverage rejected, so that an empty or impossible Position is not treated as liquidatable.
8. As a trading-bot operator, I want non-finite numeric values rejected, so that browser number overflow and malformed form values cannot produce a misleading result.
9. As a trading-bot operator, I want an invalid maintenance-margin ratio rejected, so that a risk estimate is never produced from arbitrary fallback rates.
10. As a trading-bot operator, I want the utility to be independent of a rendered form, so that bot calculations and future UI use one definition of liquidation price.
11. As a maintainer, I want the calculation covered by behavioral unit tests for both Position directions, so that formula regressions are caught before UI work begins.
12. As a maintainer, I want tests for valid maintenance-margin ratios and their boundaries, so that the calculation's assumptions remain explicit.

## Implementation Decisions

- The first deliverable is only USD(S)-M futures liquidation for one isolated Position. The margin asset is the USD(S)-M contract's quote collateral, typically USDT.
- The Position is either `long` or `short`. Hedge-mode aggregation, mixed long and short Positions, and cross-margin accounts are not represented.
- Quantity is base-asset quantity. Entry price and returned liquidation price are quote-collateral per base asset.
- The Position has no manually added or removed isolated margin. Initial isolated margin is derived as entry notional divided by leverage.
- The caller provides one maintenance-margin ratio. The calculator derives maintenance margin from the candidate liquidation notional and this ratio.
- The calculator solves the linear USD(S)-M isolated-margin equation for each direction. It does not select Binance risk brackets or apply their cumulative maintenance-margin adjustments.
- The calculator uses JavaScript `number`. It does not introduce a decimal or big-number dependency, and it does not round intermediates or results.
- Funding, trading fees, liquidation fees, realized PnL, open orders, manual margin transfers, account balances, Binance risk-bracket selection, and cumulative maintenance-margin adjustments are excluded. The returned value is a conservative estimate, not an exchange liquidation-engine guarantee.
- The public calculation function is the sole new seam. It is a pure module with no React, API, storage, or formatting dependency; future UI and bot configuration consume it rather than reimplementing its formula.
- No backend endpoint, database schema, route, generated file, or visible UI is changed.
- The domain glossary defines Position, Entry Price, and Liquidation Price. Entry Price remains distinct from average-entry and break-even calculations that will be separate future tasks.

## Testing Decisions

- Test the public calculation function's observable contract: a valid input returns the expected estimate, while invalid inputs are rejected. Do not test internal formula steps.
- Use Vitest unit tests, following the existing frontend pure-helper tests.
- Cover long and short Positions, a zero maintenance-margin ratio, invalid maintenance-margin ratios, and invalid numeric inputs.
- Use tolerance-based comparisons appropriate to JavaScript floating-point arithmetic. A future Binance Futures Calculator fixture is considered correct when the result is within one symbol price tick; the fixture must record its symbol, direction, entry price, quantity, leverage, isolated mode, calculated result, and collection date.
- Tests use formula-derived cases. They must not claim exact parity with Binance Futures Calculator values for later risk brackets.
- Run frontend quality checks, unit tests, and production build for the implementation task.

## Out of Scope

- COIN-M liquidation calculations.
- Spot, USD(S)-M, or COIN-M average-entry calculations.
- The three-tab calculation form, coin-page UI, input controls, formatting, and result presentation.
- Cross margin, auto-add margin, manual isolated-margin changes, multiple Positions, hedge-mode aggregation, and open orders.
- Binance risk-bracket selection and cumulative maintenance-margin adjustments.
- Binance authentication, account access, exchange metadata retrieval, risk-bracket synchronization, and persistence.
- Exact parity with Binance's private liquidation engine, including risk-bracket adjustments, unspecified calculator rounding, and fee behavior.

## Further Notes

- The planned delivery order is: USD(S)-M isolated liquidation, COIN-M isolated liquidation, Spot average entry, USD(S)-M average entry, and COIN-M average entry. Each is a separate, complete task.
- The estimate is deliberately more conservative than Binance for later risk brackets because it does not subtract their cumulative maintenance-margin adjustment.
