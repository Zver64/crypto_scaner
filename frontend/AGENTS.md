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
- Dedicated screen-reader support is not required. Do not add screen-reader-only markup, hidden labels (including `VisuallyHidden`), or related abstractions unless explicitly requested.

## Testing

- Write tests only where they are really needed: important logic whose breakage would be costly, such as calculations, transformations, validation, branching, edge cases, or regression-prone contracts. Keep them to a minimum. Never write tests for their own sake, to reach a count, or merely because a source file was added or changed, and never write tests that restate configuration, constants, or declarations: a test that repeats what the config says is an anti-pattern.
- Update existing tests as needed to reflect functionality changes. Running existing tests is allowed.
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
- Reference Mantine CSS variables through `themeToVars` from `@mantine/vanilla-extract` (for example `themeToVars(theme).colors.dark[5]`), not handwritten strings. Verified: shade indexes (`dark[15]`) fail typecheck. Component `vars` resolver keys are not checked by Mantine's resolver type, so check them with `satisfies` against Mantine's CssVariables type (for example `satisfies Partial<Record<TooltipCssVariables["tooltip"], string>>`); then a misspelled key (`--tooltip-bgx`) fails typecheck. Not verified by TypeScript, because Mantine types color names as any string: color names (`colors.drak`) and semantic names (`colors.defaultHovr`) in `themeToVars`, and names and shades in `theme.colors`.
- CSS custom properties belong only in CSS files, and only once a linter in `npm run quality` validates their names. No such linter exists yet (oxlint and oxfmt exclude `src/styles.css`, and nothing checks custom property names), so ask before adding styling that depends on CSS variables.

## Number formatting

- Format displayed numbers only with `formatNumber` or `formatCompactNumber` (`src/utils/number-format.ts`); no ad-hoc `Intl.NumberFormat` or rounding helpers. Pass `Decimal.toFixed()` output as a string to keep precision. Lower `maximumFractionDigits` (default 18, the backend `NUMERIC(38,18)` scale) only where smaller values are noise, such as chart axis ticks.

## React architecture

- React Compiler memoizes automatically (`reactCompilerPreset` in `vite.config.ts`, client only, so Vitest runs uncompiled code). Code must follow the Rules of React; oxlint enforces `rules-of-hooks`, `exhaustive-deps`, and the React Compiler rules of its `correctness` category (`purity`, `refs`, `immutability`, `static-components`, and the rest).
- Never memoize manually: no `memo`, `useMemo`, or `useCallback`. Solve a needed stable identity structurally (module-level definitions, query `select`, state, keys). Use `"use no memo"` only to work around a compiler problem, with a comment, and remove it once fixed.
- No effects that synchronize or reset state: derive during render, reset per-entity state by keying components, put cache housekeeping in cache handlers or subscriptions.
- Transform data at its boundary (sources and stores, module-level TanStack Query `select`/`combine` functions, one-time creation such as `createBacktestChart`); components receive ready, reference-stable data. Stateful instances (live and snapshot sources) live in `useState`, never in `useMemo`.
- Query errors are reported only through typed `meta` (`errorNotification`, `userScoped`) handled centrally in `src/app/query-client.ts`; never effects on `query.error`. Validate responses in `queryFn`, not `select`.
- Wrappers over imperative libraries (`src/components/lightweight-chart/`) compare their own inputs (`fast-deep-equal` for options, `dataChange` for data); callers never memoize for them. Functions inside chart options are module-level, created with their data, or owned by the series that applies them; never module-level caches.
- Forms use TanStack Form, only through `@/components/form` (`useAppForm`, `withForm`, `withFieldGroup`); its other files are internals. Render inputs as `field.X` inside `form.AppField` and the submit button as `form.SubmitButton` inside `form.AppForm`. A Mantine input needed by several forms becomes a field component there, built with `useFieldContext`; a feature-specific input takes `field.state` and handlers through `form.Field`. A form created by a feature hook is passed to its parts as `ReturnType<typeof useXForm>`.
- Follow the TanStack Form docs: validate with field `validators`, or form-level `validators.onChange` for checks across fields; show errors from `field.state.meta.errors` (a feature's own input inside `form.Field` uses `fieldError` from `@/components/form`); submit through `form.handleSubmit`; put side effects of a field change in its `listeners` (they do not run on `reset`); reuse groups of fields across forms with `withFieldGroup`. Values are stored in the shape inputs emit. Never-submitted state such as option filters stays local.
- Validate values with Zod schemas, kept in the feature's `schemas.ts` (shared building blocks such as `numberInputSchema` in `src/utils/`), passed to TanStack directly as validators: field schemas on fields, a form-level schema only where the whole form is checked or parsed at once (market scan and volatility settings), whose issues TanStack maps to fields by path. `onSubmit` parses the values with the same schema to get its output, and URL search parsing reuses the schema. Derive value types from schemas with `z.input` and `z.output`. Use validator functions only where a schema cannot express the check: it needs component state (the grid estimator), or it spans fields of a form a schema cannot type (the strategy market cap bounds, and its coins against the coin source).
- Read form state in render only with `useSelector(form.store, selector)`, field render props, or a lazy `useState` initializer, never `form.state` (the form object is stable, so React Compiler would keep a stale read); read `form.state` in event handlers. Keep `defaultValues` stable or deep-equal across renders, or change them only together with `form.reset(values)`, which also updates them: TanStack resets an untouched form when they change.
- Known TanStack Form limitation: it cannot compute field paths for recursive value types (TS2589, https://github.com/TanStack/form/issues/1553). The strategy form types its rule and price trees as `unknown` (`StrategyFormData`), reads them with `useSelector` through `strategyFormValues`, and writes them with `form.setFieldValue`; drop both once https://github.com/TanStack/form/pull/2422 is released. `Checkbox` keeps its last state on an undefined `checked`, so checkboxes of optional values pass `checked` explicitly.
- The strategy form is one form owned by `StrategySettings` (`form.reset(values)` when a draft opens) and not an HTML form, so Enter in a rule input does not save it; fields are separate components, and components that show values subscribe with `useSelector`. Unsaved changes are checked at event time with `strategyFormDirty`, which compares rules by their CEL. Rule builders are uncontrolled `react-querybuilder` instances (`defaultQuery` + `onQueryChange`), remounted with a new key when a query is replaced; `controlElements` are module-level and fields come from the `strategyVariableSets` select.

## Backend-driven UI

- Market Scan, Top Market Cap, and Favorites tables come from the backend `table` (columns, kinds, sortability, default sort, cells), rendered generically by `MarketScanResultsTable`; never add columns, row fields, or cell calculations in the frontend. Only a new `TableColumnKind` needs a renderer in `cellRenderers`. Favorites get their table only from `POST /api/v1/favorites/analysis`.
- Coin charts arrive only over the live WebSocket; backtest charts only as immutable snapshots in the backtest response (`chart: true`).
- Chart indicators, scanner indicator types and parameters, strategy variables, and `of` coins come from backend catalogs and descriptors; never hardcode indicator types, names, parameters, validation, colors, or per-indicator code. A catalog may be empty.
- Strategy expressions are CEL strings: `strategyExpression` and `strategyQuery` convert the builder query, and the functions in `expressions.ts` must match what the backend `strategy.Compile` accepts and `docs/strategy-language.md` describes.
- After changing scanner indicators, invalidate the scanner indicator list, chart catalogs, strategies and strategy variables, and market and favorites analyses; after changing strategies, the strategies and scanner indicator lists.
- Backtests run only through the Run button (`useBacktestRun`, outside query caches); selection changes, visits, focus, reconnects, and cache invalidation never start one, and shown results are never cleared automatically. Backtest parameters live in the URL search.
- The plaintext API token exists only in the create mutation's result (`gcTime: 0`); never put it in query caches, the URL, storage, or logs.
- Candle history loads start only from the Commands page.
