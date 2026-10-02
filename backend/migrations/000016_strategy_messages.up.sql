-- An optional Telegram alert text that replaces the generated one; empty
-- keeps the generated text.
ALTER TABLE app.strategies ADD COLUMN message TEXT NOT NULL DEFAULT '';
