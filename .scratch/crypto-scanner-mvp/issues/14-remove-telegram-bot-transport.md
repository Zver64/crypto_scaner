# 14 — Remove Telegram bot transport

**What to build:** Remove the Telegram webhook and outbound bot launch path while retaining Telegram Mini App `initData` authentication.

**Blocked by:** 11, 12, 13

**Status:** resolved

- [x] `POST /telegram/webhook` is no longer exposed.
- [x] Inbound update handling and `/start` launch behavior are removed.
- [x] The outbound Telegram Bot API client and webhook registration command are removed.
- [x] Webhook and launch-only configuration is removed from code and local tooling.
- [x] The removed command is rejected instead of starting the server.
- [x] `TELEGRAM_BOT_TOKEN` remains the signature secret source for Mini App authentication.
- [x] Authenticated analysis endpoints keep their existing contract.

## Answer

Removed the Telegram webhook transport delivered by issue 13, including
`/start` launch behavior, the outbound Bot API adapter, and the registration
command. The service now uses Telegram only to verify Mini App `initData`;
hosting and launching the Mini App are outside this backend's scope.

## Comments

This ticket supersedes issue 13's webhook-based launch path by product
decision. Issue 13 remains unchanged as the historical implementation record.
