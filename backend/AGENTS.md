# Backend

## Architecture

Composition happens only in `cmd/crypto-scanner/app.go`: it builds every dependency, the HTTP handler, and the list of background services that `lifecycle.go` runs (HTTP first, then the services; the first to stop stops the rest). Add a new background service to that list instead of starting goroutines elsewhere.

Packages and dependency direction (domain never imports adapters):

- Domain: `market` (instruments, candles, intervals, `NormalizeSymbol`), `market/kline` and `market/trade` (exchange-neutral live feeds), `market/live` (demand-driven live candle subscriptions), `market/marketsync` (REST history sync and scheduler; `ObservableStore` reports committed candle changes), `indicator` (registry of self-describing indicator modules plus `Registry.CalculateCandles`), `chart` (closed chart ranges and `LiveSession`, which merges live candles with history), `closedindicator` (background indicators on closed candles), `analysis` (criteria pipeline), `favorites`, `alerts`, `marketcap`, `auth`.
- Adapters: `indicator/talib` (every TA-Lib function through one generic adapter), `exchange/binance` (REST client and one generic WebSocket `streamPool` for kline and trade streams), `coingecko`, `storage/postgres` (one `Store`, split into `store_*.go` by aggregate), `httpapi` (transport only), `telegrambot`, `auth/telegram` (init-data verification, no `net/http`).
- Shared helpers: `platform/backoff` (retry delays, `Sleep`), `platform/numeric` (`Finite`, `ParseFinite`), `platform/logging`, `platform/config`.

Product constraints:

- Closed-candle indicators are computed in the background by `closedindicator` even with no clients, because Telegram alerts on arbitrary indicators are planned. Table values come from the same tracker. To track another indicator in tables, add a target to `closedTargets` in `app.go`; do not add request-time recalculation or per-indicator branches.
- Charts are streamed over WebSocket only (snapshot plus tail updates with sequential versions). Chart logic belongs in `chart`; `httpapi` only serializes and manages the connection.
- Chart indicators are backend-configured. `chartIndicatorCatalog` in `cmd/crypto-scanner/indicators.go` lists what every chart calculates and how clients draw it (placement, lines with theme color tokens, pane scale); clients read it from `GET /api/v1/chart/indicators`. To add a chart indicator, add a catalog entry; the frontend needs no change. `chart.NewService` validates the catalog against the registry at startup. Modules declare their outputs, and the registry rejects results that differ from them.
- `market.HistoryDepth` is the number of closed candles backfilled and kept per instrument and interval. The weekly `retention` pruner deletes older candles, and removes instruments inactive for over a week that nobody has favorited. The depth also bounds chart ranges and analysis periods (the OpenAPI chart `limit` maximum and the frontend limits mirror it). Change them together.
- Indicators come from TA-Lib through `github.com/TA-Lib/ta-lib-cgo`, which compiles the embedded C library with cgo (builds need `CGO_ENABLED=1` and a C compiler). `indicator/talib/functions_gen.go` is generated from the official `ta_func_api.xml` and checked against the wrapper signatures; one generic adapter serves every function, so never write per-indicator adapters. To upgrade TA-Lib, bump the wrapper, replace `ta_func_api.xml` with the file from the matching TA-Lib release (`ta.TALibVersion`), and run `make generate-backend`. Selection parameter keys are snake_case TA-Lib option names, with `Time Period` as `period`; a single output is named after the type (`rsi`), several outputs after TA-Lib (`macd`, `macdsignal`, `macdhist`). `Registry.Normalize` returns the canonical form with defaults; client selections must go through it before they are compared, deduplicated as closed indicator targets, or stored.
- Chart builds load each selection's `Lookback` of extra closed candles before the requested range, calculate over them, and return only the requested range, so indicator values do not depend on how far the client scrolled.

## Go conventions

- Declare small interfaces at the consumer. Put every method a consumer needs into its interface; do not detect optional capabilities with runtime type assertions.
- One constructor per type. Pass required dependencies positionally and use an `Options` struct only for genuinely optional settings. Loggers are required: never fall back to `slog.Default()` or a discard logger.
- Background services implement `Run(ctx) error` and own their context. Do not store a context in a struct or create one from `context.Background()` inside a service; pass `ctx` down. Every goroutine or timer a service starts must stop before `Run` returns (use `sync.WaitGroup`; avoid `time.AfterFunc` callbacks that can fire after shutdown). Return `nil` when `ctx` is cancelled, including during startup.
- Calls from HTTP handlers into background services must never block: use non-blocking sends with a safe fallback, and make sure the fallback cannot reorder or drop committed changes.
- Use `platform/backoff` for retry delays and `platform/numeric` to validate floats; do not write new backoff loops or NaN/Inf checks. Normalize symbols with `market.NormalizeSymbol`.
- Latest-candle reads return chronological order (oldest first) through the batch `ListLatestCandles`. Keep that contract in stores and in test fakes.
- `encoding/json` matches keys case-insensitively. When decoding external payloads with keys that differ only in case (Binance sends `e`/`E`, `t`/`T`, `v`/`V`, `q`/`Q`), declare a field for every such key, even an unused one, or one value silently overwrites another.
- Logging: attach the module once with `logger.With("module", …)` or pass a consistent `"module"` value, use the `*Context` variants when a context exists, and log errors as `"error", err`.
- Wrap errors with `%w`. Reuse sentinel errors such as `market.ErrInstrumentNotFound` instead of defining parallel ones.
- Use builtin `min`/`max`, the `slices` and `maps` packages, and Go 1.23+ timer semantics; `crypto/rand.Read` does not fail.

## HTTP API

- Build error responses with the helpers in `internal/httpapi/errors.go` so generated responses carry `X-Request-ID`. Report unexpected errors with `api.internalError`, which logs the cause; never return a 500 without logging.
- Register protected operations with method-specific patterns in `protectedRoutes` (`router.go`), so unsupported methods and unknown paths are rejected before authentication.
- `httpapi.New` takes `Dependencies`, all required. Tests replace authentication through `NewWithAuthentication` in `export_test.go`.

## Storage

- Write SQL in `internal/storage/postgres/queries/` and regenerate. Prefer batch queries (`:batchone`, `unnest` joins) over per-row round trips, and `sqlc.embed` with shared row mappers such as `instrumentFromRow` over repeated field-by-field conversions. Cast computed columns so sqlc generates typed fields.
- Database integration tests run only when `CRYPTO_SCANNER_TEST_DATABASE_URL` points to a disposable empty database and `CRYPTO_SCANNER_TEST_DATABASE_RESET_OK=1` is set. They reset the schema, so never point them at the Compose database.


## Testing

- Write new tests only when the user explicitly requests them. Do not add new tests proactively for features, fixes, refactors, or code review findings. Update existing tests as needed to reflect functionality changes; no separate user request is required. Running existing tests is allowed.
- When explicitly requested, add tests only when they verify meaningful behavior, transformations, validation, branching, edge cases, or regression-prone contracts. Do not add a test merely because a source file was added or changed.
- Do not test static configuration or constants by duplicating their values in assertions. Exercise configuration indirectly through behavioral tests when doing so protects real behavior.

## OpenAPI generation

- `internal/httpapi/openapi/openapi.yaml` is the authoritative HTTP API contract shared by the backend and frontend.
- `internal/httpapi/openapi.gen.go` is generated by `oapi-codegen`. Never edit it manually.
- Generator configuration lives in `internal/httpapi/openapi/oapi-codegen.yaml`; the pinned tool declaration lives in `tools.go`.
- After changing the contract or generator configuration, run `make generate-backend` from the repository root.
- When a contract change affects frontend consumers, run `make generate` so both backend bindings and the frontend client are regenerated from the same contract.
- After changing sqlc queries or configuration, run `make generate-sqlc` from the repository root. SQL bindings are committed and must not be edited manually.
- Commit the contract and all resulting generated files together. CI regenerates them and rejects drift.
