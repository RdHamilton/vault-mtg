-- Grant the rds_iam role to mtga_sync so that the sync Lambda can authenticate
-- via AWS IAM token instead of a static password. The rds_iam role is pre-created
-- by RDS and does not exist on non-RDS (local/test) environments; the DO block
-- guards against failure in those environments.
--
-- References: ADR-003, issue #1065, migration 000057 (role creation).
DO $$
BEGIN
  IF EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'rds_iam') THEN
    GRANT rds_iam TO mtga_sync;
  END IF;
END
$$;
