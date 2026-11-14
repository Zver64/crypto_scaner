# Crypto Scanner

Telegram Mini App for scanning cryptocurrency markets and analyzing instruments.

## Pi agents
Agent and subagent orchestration is done via herdr.

## Project structure

- `backend/`: Go HTTP API, Telegram authentication, market synchronization, analysis criteria, PostgreSQL storage, migrations, and service commands.
- `frontend/`: React Telegram Mini App built with Vite and TanStack Router.
- `backend/cmd/scanner`: the `scanner` CLI for creating strategies and backtesting saved or unsaved ones against a running backend (see CLI below).
- `docs/agents/`: repository-specific configuration consumed by engineering skills.
- `compose.yaml`: local PostgreSQL, migrations, and backend development stack (frontend runs on the host).
- `compose.production.yaml`: production deployment stack.

## Nested instructions

Agents do not load nested `AGENTS.md` files automatically. Before planning, editing, or reviewing anything under one of these directories, read its `AGENTS.md` in full; its rules apply on top of this file:

- `backend/`: `backend/AGENTS.md` (architecture, package boundaries, Go conventions).
- `frontend/`: `frontend/AGENTS.md` (component decomposition, testing, utilities, styling, number formatting).
- `docs/`: `docs/AGENTS.md` (buy strategy rules).

Pass these references on to subagents that work in those directories.

## Technology

- Backend: Go 1.26, PostgreSQL 18, pgx, sqlc, golang-migrate, Binance connector, and CoinGecko.
- Frontend: React 19 with React Compiler, TypeScript 6, Vite 8, TanStack Router, Query, and Form, Zod, Mantine, oxlint, oxfmt, and Vitest.
- Infrastructure: Docker Compose, GitHub Actions, GHCR, and Lefthook.

## Development

Copy `.env.example` to `.env`, then run:

```sh
make prepare
docker compose up
```

In a separate terminal, run `npm -C frontend run dev` to serve the frontend on the host at `127.0.0.1:3000`.

The Compose development stack starts PostgreSQL, applies migrations, and starts the backend on `127.0.0.1:8080`. The frontend is not included in development Compose. Use `docker compose up --watch` for automatic backend rebuilds on file changes. On normal backend startup, the configured `ADMIN_TELEGRAM_ID` is inserted only when absent; all existing administrator rows remain untouched.

Checks that need the running stack (`make test-db`, backtests through the CLI, checks against real data) are run by one dedicated subagent, which the orchestrating agent starts when it decides so, one at a time. That subagent starts, rebuilds (`docker compose up -d --build`), and stops Compose as it needs, without asking the user. No other agent runs Compose lifecycle commands. Never remove volumes (`docker compose down -v`).

Use `make migrate-up` and `make migrate-down` for manual migration control.

## CLI

`make install-cli` installs the `scanner` CLI and its zsh completion; `make uninstall-cli` removes them. User instructions live in `backend/cmd/scanner/README.md`, and the backtest workflow in the `.agents/skills/backtest/SKILL.md` skill. The CLI creates strategies (always disabled) and backtests saved or unsaved ones over stored history; it cannot start Binance history loads or manage API tokens, which happen only in the Mini App.

After a change to `backend/cmd/scanner` or to the OpenAPI contract, run `make install-cli` so the installed `scanner` matches the code. This build needs no explicit request.

Shared repository skills live in `.agents/skills/`; always create and edit them there, never under `.claude/skills/` (a symlink kept for Claude Code discovery).

## Verification

Run the complete repository checks with:

```sh
make check
```

Backend-only verification:

```sh
go -C backend vet ./...
go -C backend test ./...
```

Database integration tests (backend tests against the `scanner_test` database of the running Compose PostgreSQL, which `make test-db` creates when absent; it never starts Compose and fails when PostgreSQL is not running):

```sh
make test-db
```

The Lefthook `pre-push` hook runs `make test-db` when the pushed commits change `backend/`; when the Compose PostgreSQL is not running, the push fails. CI does not run the database tests.

Frontend-only verification:

```sh
npm -C frontend run quality
npm -C frontend run test
```

Verify only with tests and static analysis: `make check`, the Makefile targets, and the scripts in `frontend/package.json`. When a script in `frontend/package.json` or a Makefile target runs a tool, run it through that script or target, never directly (`npx tsc`, `npx oxlint`, `npx oxfmt`, `npx vitest`, and so on). Tools with no matching script, such as the TanStack Intent guidance commands in `frontend/AGENTS.md`, may run directly. Never build anything (`npm run build`, `vite build`, `tsc -b`, `go build`, Docker image builds) unless the user explicitly asks for a build; the exceptions are `make install-cli` and the image rebuild by the subagent that runs the stack checks (see Development).

Pre-commit hooks run the checks relevant to staged changes, so do not routinely run those checks separately before committing. Run checks manually only when diagnosing a failure, when no commit will be created, or when explicitly requested. Use `make check` for manual verification of cross-cutting changes.

## Reviews

When asked only to review code or changes, inspect `git status`, diffs, and files. Do not build, run tests, or start services unless asked to.

In addition to the usual review checks, every review also checks decomposition, architecture, and approach: responsibilities and module boundaries, readable well-named units, minimal state with clear invariants, and whether a simpler design would do.

## Commits

Use Conventional Commit messages. Use `feat:` and `fix:` only when a release
should receive the matching SemVer bump; use `!` or `BREAKING CHANGE:` only for
a genuine breaking change.
Do not add AI or tool attribution (such as `Co-Authored-By` trailers) to commit messages or pull requests.
Create commits only when the user asks.

## Agent workflow

- A user question (in chat or a review comment, even a challenge such as "why do we need that?") wants an answer and a recommendation, not a code change. "Can we…" is a question, not approval; change code only after the user agrees.
- In design discussions, answer the point raised and wait for the user's decision; do not rewrite the plan or pick a solution on your own.
- Never deviate from an agreed plan on your own: when the implementation needs a deviation, or a behaviour-changing decision the plan left open, stop and ask the user first. Subagents return such points as questions instead of deciding them, and every subagent brief says so; the orchestrating agent decides purely technical questions itself and brings only behaviour or user-facing ones to the user.
- After implementing an agreed plan or change, report only what differs from it and what was not discussed before (deviations, new findings, open risks, check results); do not restate what was agreed.
- Commit on the current branch (usually `main`); create a branch only when asked.
- Do not drive a browser to check UI changes; finish with the static checks and let the user review visually.
- Do not add UI texts, hints, captions, or warnings the user did not ask for; when asked for a visual cue such as a color, add only that cue.
- Prefer established, community-standard libraries to hand-written code, and keep own code minimal.
- Use subagents at your discretion for large or parallel work; do small tasks such as a commit or a one-file fix directly. Give each new task a new subagent with a self-contained brief; resume a subagent only for fixes to its own just-finished work. Start a review, in a separate subagent, only after the user approves it.

## Generated files

- Do not edit `backend/internal/storage/postgres/sqlc/*.go` directly. Edit migrations or SQL queries and regenerate with sqlc.
- `backend/internal/httpapi/openapi/openapi.yaml` is the authoritative shared API contract. Do not edit `backend/internal/httpapi/openapi.gen.go` manually; run `make generate-backend`.
- Do not edit anything under `frontend/src/api/generated/` manually. It is generated by Orval from the backend OpenAPI contract; run `make generate-frontend`.
- After changing the OpenAPI contract, run `make generate` to refresh both backend and frontend API artifacts.
- Do not edit `frontend/src/routeTree.gen.ts` directly. It is generated from files under `frontend/src/routes/`.

## Configuration and secrets

- Root `.env` and `.env.local` files are local and gitignored.
- Never commit or print Telegram tokens, generated Telegram init data, database passwords, or API keys.
- Keep public defaults and documentation in `.env.example`.
- Frontend API requests use relative `/api` and `/health` paths; Vite proxies them to the backend during development.

