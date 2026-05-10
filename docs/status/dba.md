# DBA Status

**Task**: Fix staging DB migration dirty version 74 + missing permissions on game_plays  
**Started**: 2026-05-10

## Step 1 — Gathered connection info ✓
- Endpoint: `mtga-companion-postgres.cujuc62msfbv.us-east-1.rds.amazonaws.com`
- Secret ARN: `arn:aws:secretsmanager:us-east-1:901347789205:secret:rds!db-12c647a0-0880-45e5-bf35-d9c1ccb05433-qzBP5e`
- DB user: `mtga_admin`

## Step 2 — Inspected migration 74 ✓
- File: `000074_add_partial_to_game_plays.up.sql`
- Operation: `ALTER TABLE game_plays ADD COLUMN IF NOT EXISTS partial BOOLEAN NOT NULL DEFAULT FALSE`
- Assessment: Safe to clear dirty flag — the `IF NOT EXISTS` guard means re-running is idempotent. No partial state harm.

## Step 3 — DB fixes applied ✓

### Fix 1: dirty migration 74
- `UPDATE schema_migrations SET dirty=false WHERE version=74` — cleared dirty flag
- Migration 74 SQL (`ADD COLUMN IF NOT EXISTS partial BOOLEAN NOT NULL DEFAULT FALSE`) manually applied since the BFF would not re-run a non-dirty migration
- Schema confirmed: `partial` column now exists on `game_plays`

### Fix 2: ownership / ALTER TABLE permission
- Root cause: `game_plays` was owned by `mtga_admin`; BFF runs migrations as `vaultmtg_staging_app`; `ALTER TABLE` requires ownership in PostgreSQL — `GRANT ALL` is insufficient
- Fix: `ALTER TABLE game_plays OWNER TO vaultmtg_staging_app`
- Confirmed: `game_plays` owner is now `vaultmtg_staging_app`

### Final DB state
- `schema_migrations`: version=75, dirty=false ✓
- `game_plays.partial` column: EXISTS, BOOLEAN NOT NULL DEFAULT FALSE ✓
- `game_plays` owner: `vaultmtg_staging_app` ✓

## Step 4 — BFF service status ✓ (DB issues resolved)
- Logs now show: `Running database migrations... Migrations complete.`
- No more dirty version error; no more `must be owner of table game_plays`
- BFF is crashing on `listen tcp :8080: bind: address already in use` — **separate port conflict issue, out of DBA scope**
- Infrastructure agent should investigate the port 8080 conflict on i-065351fbb99da2d22
