CREATE TEMP TABLE migration_v1_schema_ownership ON COMMIT DROP AS
SELECT app_schema_created, binance_spot_schema_created
FROM app.schema_migrations
WHERE version = 1;

DROP TABLE IF EXISTS binance_spot.candles;
DROP TABLE IF EXISTS binance_spot.sync_state;
DROP TABLE IF EXISTS binance_spot.instruments;
DROP TABLE IF EXISTS app.users;
DROP TABLE IF EXISTS app.schema_migrations;

DO $migration$
BEGIN
    IF (SELECT binance_spot_schema_created FROM migration_v1_schema_ownership) THEN
        DROP SCHEMA IF EXISTS binance_spot;
    END IF;
EXCEPTION
    WHEN dependent_objects_still_exist THEN NULL;
END
$migration$;

DO $migration$
BEGIN
    IF (SELECT app_schema_created FROM migration_v1_schema_ownership) THEN
        DROP SCHEMA IF EXISTS app;
    END IF;
EXCEPTION
    WHEN dependent_objects_still_exist THEN NULL;
END
$migration$;
