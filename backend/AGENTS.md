# Backend

## Architecture

Composition happens only in `cmd/crypto-scanner/app.go`: it builds every dependency, the HTTP handler, and the background services that `lifecycle.go` runs. Add a new background service to that list instead of starting goroutines elsewhere.

Packages and dependency direction (domain never imports adapters):

- Domain: `market` (instruments, candles, intervals), `market/kline`, `market/trade`, `market/live`, `market/gridlimits`, `market/marketsync` (history sync), `market/perpetuals` (daily stored Binance COIN-M perpetual list), `indicator` (registry of self-describing modules), `chart`, `closedindicator` (background indicators on closed candles), `analysis`, `markettable`, `scannerindicator`, `strategy`, `users`, `favorites`, `alerts`, `marketcap`, `auth`.
- Adapters: `indicator/talib`, `indicator/pivot`, `exchange/binance`, `coingecko`, `storage/postgres` (one `Store`, split into `store_*.go` by aggregate), `httpapi` (transport only), `telegrambot`, `auth/telegram`.
- Shared helpers: `platform/backoff`, `platform/numeric`, `platform/logging`, `platform/config`, `platform/delivery` (send workers that drain their queue on shutdown, used by the alert and strategy monitors), and `telegramhtml` (parts of the Telegram HTML notifications: escaping, coin links, cutting).

Rules:

- Authentication: only `POST /api/v1/auth/session` accepts Telegram init data; everything else uses the opaque session token or an API token (`cst_`). Store only token hashes. API tokens cannot manage API tokens or start history loads (403 `session_required`).
- CLI (`cmd/scanner`): talks to the API only through the generated `internal/apiclient`; stays free of cgo and server packages (`deps_test.go`); commands, flags, help, and completion come from `spf13/cobra`, never hand-written; never print the token.
- Every WARN and ERROR log record is sent to the administrator through Telegram (`opsnotify`); log at WARN or above only what the administrator should read. A recurring failure warns once when it starts, logs each repeat at INFO, and is re-armed by its recovery: log it through `logging.Failures` (keyed) or `logging.Failure`, never a hand-rolled flag or set.
- Indicators, chart catalogs, and indicator table columns are configured by the administrator (`scannerindicator`), never in code. Market table static columns live in `cmd/crypto-scanner/tables.go`.
- The administrator is `ADMIN_TELEGRAM_ID`; administrator routes are listed in `administratorRoutes` (`router.go`). Users are added only through the administrator's Telegram bot chat.
- Strategies are CEL expressions compiled by the one compiler in `strategy`; keep new language features there, update `docs/strategy-language.md` with every language or naming change, and keep `frontend/src/features/strategy-settings/expressions.ts` matching it. Trading is one state machine (`Entry.step`, `position.go`) shared by the live monitor and backtests, so they never diverge. Each strategy evaluates the coins chosen for it, a signal possibly the administrator's favorites, whatever they are at the time, or every active coin; otherwise favorites only put each recipient's own favorite coins first, starred, in their Telegram strategy messages. Backtests read only stored candles, never the exchange.
- `closedindicator` calculates in the background only the pairs its subscribers keep: `strategy.Service` pushes the values enabled strategies and signals read whenever they or the active instruments change, and the tracker never asks for them; it calculates them over in-memory candle windows that committed candle changes keep current, reading the store only to fill a window, so every write of the latest `market.SyncDepth` candles must notify `marketsync.ObservableStore.Changed`. Each tracker step loads the missing windows first and then calculates everything due at once, so its listener gets one call per step; there is no retry timer: a failed window load is retried by the next step, such as one a round, a subscription change, or a history load starts; a failed calculation by the next round of its interval. Table columns calculate on demand. Changes travel from their source to subscribers as events (the catalog's active instruments from `marketsync.ObservableStore`, which reports the set of active instruments each synchronization run reads once, when it changes); do not poll. Do not add per-indicator branches.
- Synchronization runs, and their late-candle retries, process instruments by descending market cap, those without one last (`ListActiveInstrumentsByMarketCap`).
- Charts are streamed only over WebSocket; chart logic belongs in `chart`, `httpapi` only serializes. `chart.Window` is the one warm-up policy of charts, the tracker, and backtests.
- Candle depths: `market.SyncDepth` (2,000) bounds synchronization and everything live (the OpenAPI chart `limit` and frontend limits mirror it; change them together); `market.RetentionDepth` (20,000) bounds what is kept and what backtests read. Deeper history comes only from administrator-started history loads. Nothing else deletes candles.
- TA-Lib (`github.com/TA-Lib/ta-lib-cgo`, cgo) serves every function through one generic adapter generated from `ta_func_api.xml`; never write per-indicator adapters. To upgrade, bump the wrapper, replace `ta_func_api.xml` from the matching release, and run `make generate-backend`. Add `indicator/pivot` modules only for calculations TA-Lib cannot do. Client indicator selections go through `Registry.Normalize` before they are compared or stored.

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
- Requests the OpenAPI validator rejects get `invalid_argument` naming the parameter or body field and the reason (`requestErrorMessage` in `router.go`), never kin-openapi's schema and value dumps.
- Register protected operations with method-specific patterns in `protectedRoutes` (`router.go`), so unsupported methods and unknown paths are rejected before authentication.
- `httpapi.New` takes `Dependencies`, all required. Tests replace authentication through `NewWithAuthentication` in `export_test.go`.

## Storage

- Write SQL in `internal/storage/postgres/queries/` and regenerate. Prefer batch queries (`:batchone`, `unnest` joins) over per-row round trips, and `sqlc.embed` with shared row mappers such as `instrumentFromRow` over repeated field-by-field conversions. Cast computed columns so sqlc generates typed fields.
- Database integration tests run only when `CRYPTO_SCANNER_TEST_DATABASE_URL` points to a disposable empty database and `CRYPTO_SCANNER_TEST_DATABASE_RESET_OK=1` is set. They reset the schema, so never point them at the Compose database.


## Testing

- Write tests only where they are really needed: important logic whose breakage would be costly, such as calculations, transformations, validation, branching, edge cases, or regression-prone contracts. Keep them to a minimum. Never write tests for their own sake, to reach a count, or merely because a source file was added or changed, and never write tests that restate configuration, constants, or declarations: a test that repeats what the config says is an anti-pattern.
- Update existing tests as needed to reflect functionality changes. Running existing tests is allowed.
- Do not test static configuration or constants by duplicating their values in assertions. Exercise configuration indirectly through behavioral tests when doing so protects real behavior.

## OpenAPI generation

- `internal/httpapi/openapi/openapi.yaml` is the authoritative HTTP API contract shared by the backend and frontend.
- `internal/httpapi/openapi.gen.go` is generated by `oapi-codegen`. Never edit it manually.
- Generator configuration lives in `internal/httpapi/openapi/oapi-codegen.yaml` (server) and `oapi-codegen-client.yaml` (the CLI client in `internal/apiclient`); the pinned tool declaration lives in `tools.go`.
- After changing the contract or generator configuration, run `make generate-backend` from the repository root.
- When a contract change affects frontend consumers, run `make generate` so both backend bindings and the frontend client are regenerated from the same contract.
- After changing sqlc queries or configuration, run `make generate-sqlc` from the repository root. SQL bindings are committed and must not be edited manually.
- Commit the contract and all resulting generated files together. CI regenerates them and rejects drift.
