# 12 — Launch the Mini App through the Telegram bot

**What to build:** Complete the Telegram launch path so the bot can receive secure webhooks, answer `/start`, and direct users to the configured Mini App.

**Blocked by:** 02 — Add configuration, logging, and process lifecycle

**Status:** ready-for-agent

- [ ] The webhook rejects a missing or incorrect Telegram secret header before decoding the body.
- [ ] Request body size is bounded and exactly one Telegram update is decoded.
- [ ] `/start` returns a message containing a button for the configured Mini App URL.
- [ ] Unknown updates are acknowledged with HTTP 200 and produce no business side effects.
- [ ] The webhook does not expose market analysis operations.
- [ ] A standalone command registers the public HTTPS webhook URL and configured secret with Telegram.
- [ ] The registration command fails clearly when Telegram does not confirm the operation.
- [ ] Bot tokens, webhook secrets, and full Telegram updates are not logged.
