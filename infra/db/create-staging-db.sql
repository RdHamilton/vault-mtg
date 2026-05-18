-- create-staging-db.sql
-- Provisions the vaultmtg_staging logical database and its application role
-- on the shared RDS instance alongside the production database.
--
-- Run this script ONCE as the RDS master user (postgres) via:
--   infra/scripts/create-staging-db.sh
--
-- Do NOT run this against production; it only creates objects in the shared
-- instance that are scoped to vaultmtg_staging.

-- ---------------------------------------------------------------------------
-- 1. Create the staging database
-- ---------------------------------------------------------------------------
-- template0 is required when specifying LC_COLLATE / LC_CTYPE explicitly.
CREATE DATABASE vaultmtg_staging
    WITH OWNER     = postgres
         ENCODING  = 'UTF8'
         LC_COLLATE = 'en_US.UTF-8'
         LC_CTYPE   = 'en_US.UTF-8'
         TEMPLATE   = template0;

-- ---------------------------------------------------------------------------
-- 2. Create the staging application role
-- ---------------------------------------------------------------------------
-- Password placeholder: replaced at execution time by create-staging-db.sh,
-- which reads the actual value from SSM /vaultmtg/staging/db-password.
-- REPLACE_WITH_STAGING_PASSWORD is never committed with a real value.
CREATE ROLE vaultmtg_staging_app
    WITH LOGIN
         PASSWORD 'REPLACE_WITH_STAGING_PASSWORD'
         CONNECTION LIMIT 10;

-- ---------------------------------------------------------------------------
-- 3. Grant database-level privileges to the staging role
-- ---------------------------------------------------------------------------
GRANT ALL PRIVILEGES ON DATABASE vaultmtg_staging TO vaultmtg_staging_app;

-- ---------------------------------------------------------------------------
-- 4. Revoke public schema create from PUBLIC (security best practice)
-- ---------------------------------------------------------------------------
-- Must be run connected to vaultmtg_staging, not to the default DB.
-- create-staging-db.sh executes this block with \c vaultmtg_staging.
\c vaultmtg_staging

REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT  CREATE ON SCHEMA public TO vaultmtg_staging_app;
GRANT  USAGE  ON SCHEMA public TO vaultmtg_staging_app;

-- After golang-migrate runs all migrations, grant table-level privileges:
--   GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO vaultmtg_staging_app;
--   ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO vaultmtg_staging_app;
--   GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO vaultmtg_staging_app;
--   ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO vaultmtg_staging_app;
-- These are run by infra/scripts/run-staging-migrations.sh after migrations complete.
