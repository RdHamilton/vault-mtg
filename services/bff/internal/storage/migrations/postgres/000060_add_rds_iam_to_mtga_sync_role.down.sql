-- Revoke the rds_iam role from mtga_sync, reverting migration 000060.
-- The DO block guards against failure on non-RDS environments where rds_iam
-- does not exist.
DO $$
BEGIN
  IF EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'rds_iam')
     AND EXISTS (
       SELECT FROM pg_auth_members m
         JOIN pg_roles r ON r.oid = m.roleid
         JOIN pg_roles mr ON mr.oid = m.member
        WHERE r.rolname = 'rds_iam' AND mr.rolname = 'mtga_sync'
     )
  THEN
    REVOKE rds_iam FROM mtga_sync;
  END IF;
END
$$;
