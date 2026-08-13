# 06 — Verify and Document the Frontend MVP

**What to build:** Turn the completed slices into a reproducible, verified Frontend MVP that another developer can run locally and confidently hand to the separate deployment workflow.

**Blocked by:** 04 — Make Scan Results Usable and Refreshable; 05 — Drill into Instrument Analysis.

**Status:** ready-for-agent

- [ ] Vitest covers pure form and route validation at defaults, boundaries, invalid integers, out-of-range values, and supported decimal Minimum Range values.
- [ ] Vitest covers the API boundary from committed criteria to emitted relative URL, query parameters, successful response mapping, and normalized canonical errors.
- [ ] Vitest covers exact symbol preservation and URL encoding for Instrument Analysis.
- [ ] Vitest covers the three-significant-digit percentage formatter across zero, very small, near-one, and larger values.
- [ ] Vitest covers local symbol filtering, no-match behavior, case handling, and preservation of backend ordering.
- [ ] Vitest covers development authorization-header construction without embedding or snapshotting a real credential.
- [ ] Pure tests cover development init-data environment validation and safe local-environment upsert behavior with fake credentials where those seams are extracted.
- [ ] Tests assert observable pure input/output behavior and do not inspect React hooks, Mantine DOM internals, CSS classes, or snapshots.
- [ ] React Testing Library, jsdom component tests, Playwright, and other browser automation are not added.
- [ ] Extend the existing local setup documentation with the test, static-check, and production-build commands added by the completed MVP.
- [ ] Static checks, Vitest, and the production build complete successfully from a clean dependency installation.
- [ ] Manual browser acceptance verifies the authentication gate, readiness transitions, successful and empty Market Scans, local filtering, same-criteria refresh, Instrument Analysis, recalculation, back navigation, cached return, and global error notifications.
- [ ] Manual responsive acceptance confirms that the AppShell, forms, and compact table remain usable at representative mobile and desktop widths.
- [ ] Where a Telegram Mini App environment is available, manual acceptance verifies `ready`, `expand`, safe areas, signed authentication, and native BackButton behavior.
- [ ] The final implementation remains English-only, fixed-dark-theme, uses only in-memory frontend cache for the current app session, and stays strictly within the existing backend API contract.
- [ ] Production Nginx, containers, hosting, Telegram Bot launch UX, backend sessions, and backend changes beyond the agreed 24-hour init-data age remain outside this ticket.
