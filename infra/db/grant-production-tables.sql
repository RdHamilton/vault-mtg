-- grant-production-tables.sql
-- Grants table and sequence privileges to vaultmtg_app (the production app user).
-- Run AFTER golang-migrate has applied all migrations to the production database.
--
-- Executed by infra/scripts/run-migrations.sh.

GRANT SELECT, INSERT, UPDATE, DELETE
    ON ALL TABLES IN SCHEMA public
    TO vaultmtg_app;

ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE
    ON TABLES TO vaultmtg_app;

GRANT USAGE, SELECT
    ON ALL SEQUENCES IN SCHEMA public
    TO vaultmtg_app;

ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT USAGE, SELECT
    ON SEQUENCES TO vaultmtg_app;
