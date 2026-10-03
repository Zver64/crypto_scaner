<!-- intent-skills:start -->
# TanStack Intent - before editing files, run the matching guidance command.
tanstackIntent:
  - id: "@tanstack/devtools#devtools-app-setup"
    run: "npx @tanstack/intent@latest load @tanstack/devtools#devtools-app-setup"
    for: "Install TanStack Devtools, pick framework adapter (React/Vue/Solid/Preact), register plugins via plugins prop, configure shell (position, hotkeys, theme, hideUntilHover, requireUrlFlag, eventBusConfig). TanStackDevtools component, defaultOpen, localStorage persistence."
  - id: "@tanstack/devtools#devtools-marketplace"
    run: "npx @tanstack/intent@latest load @tanstack/devtools#devtools-marketplace"
    for: "Publish plugin to npm and submit to TanStack Devtools Marketplace. PluginMetadata registry format, plugin-registry.ts, pluginImport (importName, type), requires (packageName, minVersion), framework tagging, multi-framework submissions, featured plugins."
  - id: "@tanstack/devtools#devtools-plugin-panel"
    run: "npx @tanstack/intent@latest load @tanstack/devtools#devtools-plugin-panel"
    for: "Build devtools panel components that display emitted event data. Listen via EventClient.on(), handle theme (light/dark), use @tanstack/devtools-ui components. Plugin registration (name, render, id, defaultOpen), lifecycle (mount, activate, destroy), max 3 active plugins. Two paths: Solid.js core with devtools-ui for multi-framework support, or framework-specific panels."
  - id: "@tanstack/devtools#devtools-production"
    run: "npx @tanstack/intent@latest load @tanstack/devtools#devtools-production"
    for: "Handle devtools in production vs development. removeDevtoolsOnBuild, devDependency vs regular dependency, conditional imports, NoOp plugin variants for tree-shaking, non-Vite production exclusion patterns."
  - id: "@tanstack/devtools-event-client#devtools-bidirectional"
    run: "npx @tanstack/intent@latest load @tanstack/devtools-event-client#devtools-bidirectional"
    for: "Two-way event patterns between devtools panel and application. App-to-devtools observation, devtools-to-app commands, time-travel debugging with snapshots and revert. structuredClone for snapshot safety, distinct event suffixes for observation vs commands, serializable payloads only."
  - id: "@tanstack/devtools-event-client#devtools-event-client"
    run: "npx @tanstack/intent@latest load @tanstack/devtools-event-client#devtools-event-client"
    for: "Create typed EventClient for a library. Define event maps with typed payloads, pluginId auto-prepend namespacing, emit()/on()/onAll()/onAllPluginEvents() API. Connection lifecycle (5 retries, 300ms), event queuing, enabled/disabled state, SSR fallbacks, singleton pattern. Unique pluginId requirement to avoid event collisions."
  - id: "@tanstack/devtools-event-client#devtools-instrumentation"
    run: "npx @tanstack/intent@latest load @tanstack/devtools-event-client#devtools-instrumentation"
    for: "Analyze library codebase for critical architecture and debugging points, add strategic event emissions. Identify middleware boundaries, state transitions, lifecycle hooks. Consolidate events (1 not 15), debounce high-frequency updates, DRY shared payload fields, guard emit() for production. Transparent server/client event bridging."
  - id: "@tanstack/devtools-vite#devtools-vite-plugin"
    run: "npx @tanstack/intent@latest load @tanstack/devtools-vite#devtools-vite-plugin"
    for: "Configure @tanstack/devtools-vite for source inspection (data-tsd-source, inspectHotkey, ignore patterns), console piping (client-to-server, server-to-client, levels), enhanced logging, server event bus (port, host, HTTPS), production stripping (removeDevtoolsOnBuild), editor integration (launch-editor, custom editor.open). Must be FIRST plugin in Vite config. Vite ^6 || ^7 only."
  - id: "@tanstack/router-core#router-core"
    run: "npx @tanstack/intent@latest load @tanstack/router-core#router-core"
    for: "Framework-agnostic core concepts for TanStack Router: route trees, createRouter, createRoute, createRootRoute, createRootRouteWithContext, addChildren, Register type declaration, route matching, route sorting, file naming conventions. Entry point for all router skills."
  - id: "@tanstack/router-core#router-core/auth-and-guards"
    run: "npx @tanstack/intent@latest load @tanstack/router-core#router-core/auth-and-guards"
    for: "Route protection with beforeLoad, redirect()/throw redirect(), isRedirect helper, authenticated layout routes (_authenticated), non-redirect auth (inline login), RBAC with roles and permissions, auth provider integration (Auth0, Clerk, Supabase), router context for auth state."
  - id: "@tanstack/router-core#router-core/code-splitting"
    run: "npx @tanstack/intent@latest load @tanstack/router-core#router-core/code-splitting"
    for: "Automatic code splitting (autoCodeSplitting), .lazy.tsx convention, createLazyFileRoute, createLazyRoute, lazyRouteComponent, getRouteApi for typed hooks in split files, codeSplitGroupings per-route override, splitBehavior programmatic config, critical vs non-critical properties."
  - id: "@tanstack/router-core#router-core/data-loading"
    run: "npx @tanstack/intent@latest load @tanstack/router-core#router-core/data-loading"
    for: "Route loader option, loaderDeps for cache keys, staleTime/gcTime/ defaultPreloadStaleTime SWR caching, pendingComponent/pendingMs/ pendingMinMs, errorComponent/onError/onCatch, beforeLoad, router context and createRootRouteWithContext DI pattern, router.invalidate, Await component, deferred data loading with unawaited promises."
  - id: "@tanstack/router-core#router-core/navigation"
    run: "npx @tanstack/intent@latest load @tanstack/router-core#router-core/navigation"
    for: "Link component, useNavigate, Navigate component, router.navigate, ToOptions/NavigateOptions/LinkOptions, from/to relative navigation, activeOptions/activeProps, preloading (intent/viewport/render), preloadDelay, navigation blocking (useBlocker, Block), createLink, linkOptions helper, scroll restoration, MatchRoute."
  - id: "@tanstack/router-core#router-core/not-found-and-errors"
    run: "npx @tanstack/intent@latest load @tanstack/router-core#router-core/not-found-and-errors"
    for: "notFound() function, notFoundComponent, defaultNotFoundComponent, notFoundMode (fuzzy/root), errorComponent, CatchBoundary, CatchNotFound, isNotFound, NotFoundRoute (deprecated), route masking (mask option, createRouteMask, unmaskOnReload)."
  - id: "@tanstack/router-core#router-core/path-params"
    run: "npx @tanstack/intent@latest load @tanstack/router-core#router-core/path-params"
    for: "Dynamic path segments ($paramName), splat routes ($ / _splat), optional params ({-$paramName}), prefix/suffix patterns ({$param}.ext), useParams, params.parse/stringify, pathParamsAllowedCharacters, i18n locale patterns."
  - id: "@tanstack/router-core#router-core/search-params"
    run: "npx @tanstack/intent@latest load @tanstack/router-core#router-core/search-params"
    for: "validateSearch, search param validation with Zod/Valibot/ArkType adapters, fallback(), search middlewares (retainSearchParams, stripSearchParams), custom serialization (parseSearch, stringifySearch), search param inheritance, loaderDeps for cache keys, reading and writing search params."
  - id: "@tanstack/router-core#router-core/ssr"
    run: "npx @tanstack/intent@latest load @tanstack/router-core#router-core/ssr"
    for: "Non-streaming and streaming SSR, RouterClient/RouterServer, renderRouterToString/renderRouterToStream, createRequestHandler, defaultRenderHandler/defaultStreamHandler, HeadContent/Scripts components, head route option (meta/links/styles/scripts), ScriptOnce, automatic loader dehydration/hydration, memory history on server, data serialization, document head management."
  - id: "@tanstack/router-core#router-core/type-safety"
    run: "npx @tanstack/intent@latest load @tanstack/router-core#router-core/type-safety"
    for: "Full type inference philosophy (never cast, never annotate inferred values), Register module declaration, from narrowing on hooks and Link, strict:false for shared components, getRouteApi for code-split typed access, addChildren with object syntax for TS perf, LinkProps and ValidateLinkOptions type utilities, as const satisfies pattern."
  - id: "@tanstack/router-plugin#router-plugin"
    run: "npx @tanstack/intent@latest load @tanstack/router-plugin#router-plugin"
    for: "TanStack Router bundler plugin for route generation and automatic code splitting. Supports Vite, Webpack, Rspack, and esbuild. Configures autoCodeSplitting, routesDirectory, target framework, and code split groupings."
  - id: "@tanstack/virtual-file-routes#virtual-file-routes"
    run: "npx @tanstack/intent@latest load @tanstack/virtual-file-routes#virtual-file-routes"
    for: "Programmatic route tree building as an alternative to filesystem conventions: rootRoute, index, route, layout, physical, defineVirtualSubtreeConfig. Use with TanStack Router plugin's virtualRouteConfig option."
<!-- intent-skills:end -->

## API generation

- `../backend/internal/httpapi/openapi/openapi.yaml` is the authoritative API contract.
- `orval.config.ts` configures the generated TanStack React Query client and TypeScript models.
- Everything under `src/api/generated/` is generated by Orval. Never edit any generated file manually.
- Regenerate the frontend client with `npm run generate-api` from this directory or `make generate-frontend` from the repository root.
- After changing the OpenAPI contract, run `make generate` from the repository root to regenerate both backend and frontend artifacts.
- Commit generated API changes together with the contract or generator configuration that produced them. CI regenerates the client and rejects drift.
- Consume generated request functions, query keys/options/hooks, and DTOs directly. Do not duplicate generated transport, response parsing, query keys, hooks, or contract types in handwritten modules.

## Component decomposition

- Keep each React component in its own file.
- Place genuinely reusable, business-agnostic UI components in `src/components/`. Their names, props, and implementation must not depend on a specific feature, domain concept, or business meaning.
- Shared components accept generic presentation data or content. Keep business calculations, domain-specific labels, units, formatting, API calls, and feature state in the consuming feature; shared components must not import from feature modules.
- Place components specific to a feature or screen alongside that feature in separate files, not in `src/components/`. Repetition alone does not make a component suitable for the shared directory.
- Follow KISS: extract clear UI responsibilities and reuse existing components rather than duplicating markup. Do not introduce speculative abstractions or configuration unrelated to actual use cases.

## Testing

- Write new tests only when the user explicitly requests them. Do not add new tests proactively for features, fixes, refactors, or code review findings. Update existing tests as needed to reflect functionality changes; no separate user request is required. Running existing tests is allowed.
- When explicitly requested, add tests only when they verify meaningful behavior, transformations, validation, branching, edge cases, or regression-prone contracts. Do not add a test merely because a source file was added or changed.
- Do not test static configuration or constants by duplicating their values in assertions. Exercise configuration indirectly through behavioral tests when doing so protects real behavior.
- Do not test React components, JSX/TSX output, rendered markup, styles, layout, accessibility attributes, or any other visual/UI behavior with Vitest.
- Vitest tests must cover only ordinary non-visual functions and modules that do not depend on JSX/TSX rendering.
- Do not add `.test.tsx` or `.spec.tsx` files, component render tests, snapshots, DOM assertions, or `react-dom`/Testing Library render helpers to Vitest tests.
- Extract non-visual logic from components into plain TypeScript functions when it needs unit-test coverage, and test those functions separately in `.test.ts` files.

## Utilities

- Place UI- and feature-independent utility functions in `src/utils/`, not in components, app-shell modules, routes, or feature modules.
- Co-locate a utility's unit tests with that utility in `src/utils/`.
- Use the `@/` alias for imports; do not add relative imports.
- Keep component-specific utility functions and their tests in the component directory as `utils.ts` and `utils.test.ts`. A `utils.ts` file must contain only functions or methods—never configuration objects, constants, or type declarations.
- Keep `src/config.ts` limited to meaningful application-wide product-tuning values, such as shared presets and defaults. Keep presentation metadata, component behavior, domain constraints, and arbitrary constants with their owning modules. Put component-specific type declarations in the component directory's `types.ts`.
- Do not use `<component>-utils.ts` files at the feature root.

## Styling and static verification

- Do not make changes that static analysis cannot verify. Every name used in code must fail `npm run quality` when misspelled.
- Never write CSS custom properties or `var(--…)` references as strings in TS/TSX: not in props, `style` objects, theme `vars` resolvers, or template strings. Neither TypeScript nor a linter can check them.
- In TS/TSX, pass style values only as typed JS data whose names TypeScript checks. Before relying on an API, confirm that a deliberate typo fails `npm run typecheck` (strip ANSI colors before grepping `tsc` output).
- Reference Mantine CSS variables through `themeToVars` from `@mantine/vanilla-extract` (for example `themeToVars(theme).colors.dark[5]`), not handwritten strings. Verified: shade indexes (`dark[15]`) and component `vars` resolver keys (`--tooltip-bgx`) fail typecheck. Not verified by TypeScript, because Mantine types color names as any string: color names (`colors.drak`) and semantic names (`colors.defaultHovr`) in `themeToVars`, and names and shades in `theme.colors`.
- CSS custom properties belong only in CSS files, and only once a linter in `npm run quality` validates their names. No such linter exists yet (Biome excludes `src/styles.css` and does not check custom property names), so ask before adding styling that depends on CSS variables.

## Number formatting

- Format displayed numbers with `formatNumber` or `formatCompactNumber` from `src/utils/number-format.ts`; do not create ad-hoc `Intl.NumberFormat` instances or custom rounding helpers.
- `formatNumber` shows whichever of the whole integer or four significant digits is more precise (158.73 → 158.7, 5785.4 → 5,785) and accepts numbers or numeric strings. Pass `Decimal.toFixed()` output as a string to keep precision beyond JavaScript numbers.
- Its optional `maximumFractionDigits` defaults to 18, the backend `NUMERIC(38,18)` scale. Pass a lower limit only when values below a known resolution are noise, such as chart axis ticks rounded at the chart price resolution.
- Formatters are cached per fraction digit limit because chart axes format every label on each redraw.

## Market scan tables

- Market Scan, Top Market Cap, and Favorites tables are defined by the backend: every analysis response carries `table` with the columns (order, title, rendering `kind`, sortability), the default sort, and one row of cells per instrument. `MarketScanResultsTable` renders it generically; the frontend fixes only that the first column is sticky. Do not add column definitions, row fields, cell calculations, or sortable-column lists to the frontend. A new column is a backend catalog entry; the frontend changes only when the backend adds a `TableColumnKind`, which needs a renderer in `cellRenderers` (`results-table/cells.tsx`).
- Sorting runs on the client over the cell `value` of sortable columns. `resolveTableSort` falls back to the table's `default_sort` when the URL names a column the table does not mark sortable.
- Favorites get their table, including favorites the analysis skipped, only from `POST /api/v1/favorites/analysis`; do not merge it with `GET /api/v1/favorites`.
- Charts arrive over the live WebSocket only (snapshot, then tail updates with sequential versions); there is no HTTP chart endpoint.
- Chart indicators come from the backend catalog of each interval (`GET /api/v1/chart/indicators?interval=`, read for every chart interval with `useChartIndicators`). The administrator configures them, so a catalog may be empty and the chart then draws candles only. The coin chart subscribes each interval with that interval's selections and renders overlays and panes generically from it; do not add per-indicator code, constants or validation to the frontend. Colors arrive as Mantine theme tokens and are resolved against the app theme.
- Indicator table columns use the `number` kind (`formatNumber`); which indicators become columns is configured by the administrator, not the frontend.

## Scanner settings

- The administrator (`GET /api/v1/me`, read with `useAdministrator`) manages the global scanner indicators on `/admin` (`features/scanner-settings/`). The page is entered only by clicking the unmarked "CS" app name in the header, which turns into a close cross on the page (Telegram's back button closes it too); other users see plain text and the backend rejects admin requests.
- The list is one display order, changed by dragging rows (`@hello-pangea/dnd`, touch included) and saved with `PUT /api/v1/admin/scanner-indicator-order`. That order sets the indicator column order in tables (just before Favorite) and the indicator order on charts.
- The add form is built from the backend indicator descriptors (`GET /api/v1/admin/indicator-types`); do not hardcode indicator types, parameters, or validation. The backend decides chart placement, colors, and column titles.
- After changing indicators, invalidate the scanner indicator list, chart catalogs, strategies and strategy variables, and market and favorites analyses so tables and charts pick up the change.
- Indicators that strategies read (`ScannerIndicator.strategies`) are highlighted and cannot be deleted; "Remove all" is disabled while any is used.
- Strategies live on `/admin/strategies` (`features/strategy-settings/`). They are built with `react-querybuilder` and `@react-querybuilder/expr` (no compatibility packages, no expr UI); every control is our own Mantine component: `StrategyRule` for one comparison, `ExpressionEditor` as the single operand editor of either side (indicator, number on the right, function through `FunctionEditor`, or Coin, an operand of another coin through `of`, which is not offered among the functions; plain indicators and numbers are saved as plain rule fields and values), `StrategyGroup` and `StrategyGroupHeader` for AND/OR groups with NOT. Fields come from `GET /api/v1/admin/strategy-variables` and the coins of `of` from `GET /api/v1/admin/strategy-symbols`; do not hardcode indicator names or coins. The API contract is the CEL string: `strategyExpression` exports the query (crosses and expression ranges through a custom rule processor, other expressions through the expr CEL processor) and `strategyQuery` parses it back, restoring ranges and negated groups. The functions, their CEL serializers, and their parsing live in `expressions.ts` and must match what the backend `strategy.Compile` accepts. After changing strategies, invalidate the strategies and scanner indicator lists.
- The Users page toggles `strategy_alerts` per user; the administrator always receives strategy alerts.
