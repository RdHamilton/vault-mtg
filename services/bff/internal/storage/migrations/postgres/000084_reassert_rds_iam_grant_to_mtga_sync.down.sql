-- Revoke rds_iam from mtga_sync. Guarded against environments where the
-- rds_iam role does not exist (local/test). Reverses 000084.
DO $$
BEGIN
  IF EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'rds_iam') THEN
    REVOKE rds_iam FROM mtga_sync;
  END IF;
END
$$;
