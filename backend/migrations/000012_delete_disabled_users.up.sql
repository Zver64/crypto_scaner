-- Revoking access now deletes the user, so disabled users are deleted with
-- their favorites and price alerts, and every remaining user has access.
DELETE FROM app.users WHERE NOT is_enabled;

ALTER TABLE app.users DROP COLUMN is_enabled;
