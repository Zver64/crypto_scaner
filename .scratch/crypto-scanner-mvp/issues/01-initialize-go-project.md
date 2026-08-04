# 01 — Initialize the Go project

**What to build:** Create the agreed project foundation so every later slice starts from a compiling, navigable Go service rather than inventing structure independently.

**Blocked by:** None — can start immediately

**Status:** resolved

- [x] The repository has a valid Go module and the agreed modular-monolith package layout.
- [x] The main command builds and starts without placeholder business behavior.
- [x] Agreed runtime and development dependencies are declared and reproducibly installable.
- [x] Build, test, code-generation, and dependency-tidying commands are documented and executable.
- [x] The baseline test command passes and the module is clean after dependency tidying.
- [x] Local secrets, generated binaries, coverage output, and editor artifacts are excluded from version control.

## Answer

Initialized the `crypto-scanner` Go 1.26 module with the specified modular-monolith
package boundaries and a no-op composition command. Pinned the agreed PostgreSQL,
Binance, Telegram, and `sqlc` dependencies, documented executable Make targets,
and added ignore rules for local secrets and generated development artifacts.

Validated with `go mod tidy`, `make generate`, `make test`, `make vet`,
`make build`, and a successful invocation of `bin/crypto-scanner`.
