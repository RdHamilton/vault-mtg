-- Re-assert GRANT rds_iam TO mtga_sync.
--
-- Migration 000060 added the same grant, but it was no-op on production because
-- the rds_iam Postgres role did not exist at the time -- the live RDS instance
-- had IAMDatabaseAuthenticationEnabled=false until vault-mtg-tickets#37 enabled
-- it via rds-vaultmtg.yml. golang-migrate records 000060 as applied even when
-- its IF EXISTS guard short-circuited, so 000060 will not re-run on subsequent
-- deploys.
--
-- This migration re-asserts the grant once IAM auth is enabled on the instance
-- (which creates the rds_iam role). It carries the same IF EXISTS guard for
-- safety in local/test environments that have no rds_iam role.
--
-- References: ADR-003, vault-mtg-tickets#37, migrations 000057 (role creation)
-- and 000060 (original grant attempt).
DO $$
BEGIN
  IF EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'rds_iam') THEN
    GRANT rds_iam TO mtga_sync;
  END IF;
END
$$;
