# Infrastructure Agent -- Current Task Status
**Updated**: 2026-05-10
**Task**: Diagnose and fix staging deploy failure
**Status**: In Progress

## Progress
- [x] Confirmed EC2 instance running (system/instance status: ok)
- [x] Confirmed staging deploy CI run 25631623971 failed at "Provision staging env" step (exit 254)
- [x] Identified root causes:
  - Primary: `mtga-bff.service` (production unit) running on staging instance, reading /etc/mtga-companion/env which has mtga_admin credentials -- crashing in restart loop, caused port 8080 conflict
  - Secondary: `mtga-companion-staging.service` is healthy (PID 674405, port 8081) -- staging deploy IS working
  - CI failure was a race: provision script succeeded but the CI exit 254 came from the restart step hitting the port conflict
- [x] Confirmed DATABASE_URL in SSM (/mtga-companion/staging/database-url) is correct and vaultmtg_staging_app connects successfully
- [ ] Stop and disable rogue mtga-bff.service on staging instance
- [ ] Re-trigger staging deploy to confirm clean run

## Blockers
None -- fix in progress

## ETA
~10 minutes
