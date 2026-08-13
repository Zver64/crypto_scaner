# 02 — Bootstrap the Mini App Shell and Access State

**What to build:** Deliver the application frame an authorized user sees when opening the Mini App: a fixed dark Mantine shell, Telegram initialization, access gating, backend readiness, and globally available request infrastructure.

**Blocked by:** 01 — Prepare Frontend Dependencies.

**Status:** resolved

- [x] Every route renders inside one Mantine `AppShell` with a compact header containing `CS` on the left and backend readiness on the right.
- [x] The fixed dark appearance is configured through `MantineProvider`, theme tokens, Mantine CSS variables, component props, and the official Styles API.
- [x] Mantine component internals are not restyled through ad hoc global selectors or hardcoded replacement colors.
- [x] TanStack Query is provided once at the application root with an in-memory cache and no browser persistence.
- [x] Mantine Notifications is mounted once at the application root and can be invoked globally without page-level alert components.
- [x] The application calls the supported Telegram `ready()` and `expand()` methods once and accounts for Telegram/device safe areas without requesting full-screen mode.
- [x] A production visitor without signed Telegram init data sees a blocking English-language "Open in Telegram" state and cannot invoke business API requests.
- [x] Readiness is requested immediately from the existing health endpoint and refreshed every 30 seconds.
- [x] The header clearly distinguishes checking, ready, and unavailable states without exposing internal backend details.
- [x] Analysis actions have a reusable source of truth indicating whether authentication and readiness permit a request.
- [x] The application remains usable in an ordinary development browser even though Telegram-only navigation controls are unavailable.

## Answer

Added the fixed dark Mini App shell, shared Query and Notifications providers,
Telegram initialization and safe-area handling, production access gating,
30-second backend readiness, and a reusable business-request permission source.
