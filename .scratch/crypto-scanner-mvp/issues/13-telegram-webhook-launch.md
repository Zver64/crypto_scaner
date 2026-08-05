# 13 — Launch the Mini App through the Telegram bot

**What to build:** Complete the Telegram launch path so the bot can receive secure webhooks, answer `/start`, and direct users to the configured Mini App.

**Blocked by:** 08 — Configure local development

**Status:** resolved

- [x] The webhook rejects a missing or incorrect Telegram secret header before decoding the body.
- [x] Request body size is bounded and exactly one Telegram update is decoded.
- [x] `/start` returns a message containing a button for the configured Mini App URL.
- [x] Unknown updates are acknowledged with HTTP 200 and produce no business side effects.
- [x] The webhook does not expose market analysis operations.
- [x] A standalone command registers the public HTTPS webhook URL and configured secret with Telegram.
- [x] The registration command fails clearly when Telegram does not confirm the operation.
- [x] Bot tokens, webhook secrets, and full Telegram updates are not logged.
- [x] The webhook and registration command use the shared `.env` in both development modes; a public HTTPS address remains operator-provided and is not created by Compose.

## Answer

Implemented the secure Telegram webhook transport, `/start` Mini App launch
button, no-op handling for unknown updates, and the Telegram Bot API outbound
adapter. Added the explicit `crypto-scanner telegram set-webhook` command with
focused configuration loading, positive Telegram confirmation, shared `.env`
development commands, and operator-owned public HTTPS URL documentation.
