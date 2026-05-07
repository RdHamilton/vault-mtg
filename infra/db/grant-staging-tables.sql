-- grant-staging-tables.sql
-- Grants table and sequence privileges to vaultmtg_staging_app.
-- Run AFTER golang-migrate has applied all migrations to vaultmtg_staging.
--
-- This is a separate script from create-staging-db.sql because the tables
-- do not exist until migrations complete.
--
-- Executed by infra/scripts/run-staging-migrations.sh.

GRANT SELECT, INSERT, UPDATE, DELETE
    ON ALL TABLES IN SCHEMA public
    TO vaultmtg_staging_app;

ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE
    ON TABLES TO vaultmtg_staging_app;

GRANT USAGE, SELECT
    ON ALL SEQUENCES IN SCHEMA public
    TO vaultmtg_staging_app;

ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT USAGE, SELECT
    ON SEQUENCES TO vaultmtg_staging_app;
