# Infrastructure Agent -- Staging Stabilization
**Updated**: 2026-05-10T15:20
**Task**: Get staging fully stable (fix IAM, deploy, verify /healthz)
**Status**: In Progress -- waiting for LE review of PR #1734

## Progress
- [x] Added `s3:PutBucketVersioning` + `s3:GetBucketVersioning` to `github-actions-oidc-deploy` inline policy (direct AWS CLI)
- [x] Triggered staging deploy -- old runs (a6256a9) failed with exit 254 on SSM poll loop (pre-existing bug, already fixed in 030e860)
- [x] Triggered new run on 030e860 -- provision + stage-binary passed; migrations failed: migrate CLI not installed + script expected repo at /opt/mtga-companion
- [x] PR #1734 opened: fix migrations to download from S3, auto-install migrate CLI
- [ ] LE review and merge PR #1734
- [ ] Trigger staging deploy after merge
- [ ] Verify BFF /healthz returns 200

## Blockers
Waiting for LE review of PR #1734

## ETA
~20 minutes after LE approves
