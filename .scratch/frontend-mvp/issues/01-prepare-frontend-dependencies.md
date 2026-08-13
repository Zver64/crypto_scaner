# 01 — Prepare Frontend Dependencies

**What to build:** Establish a verified dependency foundation for the Frontend MVP so subsequent tickets can use the agreed routing, server-state, form, notification, theme, and test capabilities without stopping to redesign or install infrastructure.

**Blocked by:** None — can start immediately.

**Status:** ready-for-agent

- [ ] Audit the existing React, Vite, TanStack Router, Mantine, and TypeScript versions for compatibility with the additions required by the Frontend MVP.
- [ ] Add TanStack Query as the single server-state and in-memory caching solution.
- [ ] Add Mantine Form as the form-state and validation solution.
- [ ] Add Mantine Notifications as the global transient-notification solution.
- [ ] Add Vitest as the test runner for pure frontend logic.
- [ ] Do not add Zustand, React Testing Library, snapshot tooling, Playwright, an internationalization framework, or a second notification system.
- [ ] Do not add a client Telegram SDK unless the official `window.Telegram.WebApp` API is demonstrably insufficient for the agreed runtime behavior.
- [ ] Provide package scripts that let an agent run unit tests and the existing frontend quality checks consistently.
- [ ] A clean dependency installation completes without unresolved or incompatible peer dependencies.
- [ ] The unchanged frontend passes its configured static checks and production build after the dependency update.
