# 01 — Prepare Frontend Dependencies

**What to build:** Establish a verified dependency foundation for the Frontend MVP so subsequent tickets can use the agreed routing, server-state, form, notification, theme, and test capabilities without stopping to redesign or install infrastructure.

**Blocked by:** None — can start immediately.

**Status:** resolved

- [x] Audit the existing React, Vite, TanStack Router, Mantine, and TypeScript versions for compatibility with the additions required by the Frontend MVP.
- [x] Add TanStack Query as the single server-state and in-memory caching solution.
- [x] Add Mantine Form as the form-state and validation solution.
- [x] Add Mantine Notifications as the global transient-notification solution.
- [x] Add Vitest as the test runner for pure frontend logic.
- [x] Do not add Zustand, React Testing Library, snapshot tooling, Playwright, an internationalization framework, or a second notification system.
- [x] Do not add a client Telegram SDK unless the official `window.Telegram.WebApp` API is demonstrably insufficient for the agreed runtime behavior.
- [x] Provide package scripts that let an agent run unit tests and the existing frontend quality checks consistently.
- [x] A clean dependency installation completes without unresolved or incompatible peer dependencies.
- [x] The unchanged frontend passes its configured static checks and production build after the dependency update.

## Answer

Added `@tanstack/react-query` 5.101.4, `@mantine/form` 9.5.1,
`@mantine/notifications` 9.5.1, and Vitest 4.1.10. The installed peer ranges
cover React 19.2.8, Mantine 9.5.1, and Vite 8.2.0; `npm ls --all` reports no
invalid or unresolved required peers. Added consistent `test`, `test:watch`,
`typecheck`, and `quality` scripts. No excluded state, browser-test,
internationalization, notification, or Telegram SDK dependency was added.

Verification completed from `frontend/`: `npm ci`, `npm ls --all`,
`npm run quality`, `npm test`, and `npm run build` all exited successfully.
The only source changes are Biome formatting and removal of one pre-existing
unused import required for the configured static checks to pass; runtime
behavior is unchanged.

## Comments

- Dependency/tooling ticket: TDD was not applicable because there is no new
  product behavior. Clean installation, package scripts, static analysis, the
  empty Vitest seam, and the production build were used as the public
  verification boundary.
