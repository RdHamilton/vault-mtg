-- create-production-db.sql
-- Provisions the vaultmtg_app application role on the shared RDS instance.
--
-- Run this script ONCE as the RDS master user (mtga_admin / postgres) via:
--   infra/scripts/create-production-db.sh
--
-- NOTE: This script does NOT create the production database — it already
-- exists. It only creates/converges the vaultmtg_app application role and
-- applies schema-level grants so the BFF can connect with least privilege.
--
-- The production database (mtga_companion) and its schema were created when
-- the RDS instance was provisioned. Do NOT add a CREATE DATABASE statement
-- here.
--
-- Idempotency: the DO-block guard converges an existing role (including the
-- manually-created NOLOGIN role from the v0.3.1 incident) to LOGIN + managed
-- password without error on a second run.

-- ---------------------------------------------------------------------------
-- 1. Create or converge the production application role
-- ---------------------------------------------------------------------------
-- Password placeholder: replaced at execution time by create-production-db.sh,
-- which generates the actual value and stores it in Secrets Manager.
-- REPLACE_WITH_PROD_PASSWORD is never committed with a real value.
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'vaultmtg_app') THEN
        CREATE ROLE vaultmtg_app
            WITH LOGIN
                 PASSWORD 'REPLACE_WITH_PROD_PASSWORD'
                 CONNECTION LIMIT 10;
    ELSE
        -- Converges the manually-created NOLOGIN role (v0.3.1 incident) to
        -- LOGIN with the newly-generated managed password.
        ALTER ROLE vaultmtg_app WITH LOGIN PASSWORD 'REPLACE_WITH_PROD_PASSWORD' CONNECTION LIMIT 10;
    END IF;
END
$$;

-- ---------------------------------------------------------------------------
-- 2. Grant database-level privileges to the production role
-- ---------------------------------------------------------------------------
GRANT ALL PRIVILEGES ON DATABASE mtga_companion TO vaultmtg_app;

-- ---------------------------------------------------------------------------
-- 3. Schema-level grants
-- ---------------------------------------------------------------------------
-- Must be run connected to mtga_companion (the production database).
-- create-production-db.sh executes this block with \c mtga_companion.
\c mtga_companion

REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT  CREATE ON SCHEMA public TO vaultmtg_app;
GRANT  USAGE  ON SCHEMA public TO vaultmtg_app;

-- After golang-migrate runs all migrations, table-level privileges are
-- applied by infra/scripts/run-migrations.sh:
--   GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO vaultmtg_app;
--   ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO vaultmtg_app;
--   GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO vaultmtg_app;
--   ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO vaultmtg_app;
-- These are the same grants already in infra/db/grant-production-tables.sql.
