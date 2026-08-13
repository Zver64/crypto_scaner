# 06 — Verify and Document the Frontend MVP

**What to build:** Turn the completed slices into a reproducible, verified Frontend MVP that another developer can run locally and confidently hand to the separate deployment workflow.

**Blocked by:** 04 — Make Scan Results Usable and Refreshable; 05 — Drill into Instrument Analysis.

**Status:** resolved

- [x] Vitest covers pure form and route validation at defaults, boundaries, invalid integers, out-of-range values, and supported decimal Minimum Range values.
- [x] Vitest covers the API boundary from committed criteria to emitted relative URL, query parameters, successful response mapping, and normalized canonical errors.
- [x] Vitest covers exact symbol preservation and URL encoding for Instrument Analysis.
- [x] Vitest covers the three-significant-digit percentage formatter across zero, very small, near-one, and larger values.
- [x] Vitest covers local symbol filtering, no-match behavior, case handling, and preservation of backend ordering.
- [x] Vitest covers development authorization-header construction without embedding or snapshotting a real credential.
- [x] Pure tests cover development init-data environment validation and safe local-environment upsert behavior with fake credentials where those seams are extracted.
- [x] Tests assert observable pure input/output behavior and do not inspect React hooks, Mantine DOM internals, CSS classes, or snapshots.
- [x] React Testing Library, jsdom component tests, Playwright, and other browser automation are not added.
- [x] Extend the existing local setup documentation with the test, static-check, and production-build commands added by the completed MVP.
- [x] Static checks, Vitest, and the production build complete successfully from a clean dependency installation.
- [x] Manual browser acceptance verifies the authentication gate, readiness transitions, successful and empty Market Scans, local filtering, same-criteria refresh, Instrument Analysis, recalculation, back navigation, cached return, and global error notifications.
- [x] Manual responsive acceptance confirms that the AppShell, forms, and compact table remain usable at representative mobile and desktop widths.
- [x] Where a Telegram Mini App environment is available, manual acceptance verifies `ready`, `expand`, safe areas, signed authentication, and native BackButton behavior.
- [x] The final implementation remains English-only, fixed-dark-theme, uses only in-memory frontend cache for the current app session, and stays strictly within the existing backend API contract.
- [x] Production Nginx, containers, hosting, Telegram Bot launch UX, backend sessions, and backend changes beyond the agreed 24-hour init-data age remain outside this ticket.

## Answer

Extended the local README with backend startup, clean frontend installation, test, quality, and production-build commands. Extracted deterministic development init-data validation, safe Telegram ID parsing, and private environment upsert logic and covered them with fake-only pure tests. Added explicit proxy authorization-header tests.

After `npm ci`, all 40 tests, Biome, TypeScript, and the production build passed. Manual acceptance covered the production Telegram gate, readiness transition, successful and empty scans, local filtering, same-criteria refresh with retained data, exact-symbol Instrument Analysis, recalculation, reload reconstruction, global error notification, cached Back return, and browser Back fallback. The table and forms remained usable at 390×844 and 1280×900 with no browser console errors. The available Telegram 6.0 shim exercised `ready`, `expand`, and safe browser fallback; a full Telegram client with supported native BackButton was not available in this environment, so the version-gated native integration remains the implementation-level acceptance for that conditional check.
