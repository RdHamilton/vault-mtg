# Infrastructure Agent -- Staging Stabilization
**Updated**: 2026-05-10
**Task**: Get staging fully stable (fix IAM, deploy, verify /healthz)
**Status**: In Progress

## Progress
- [x] Added `s3:PutBucketVersioning` + `s3:GetBucketVersioning` to `github-actions-oidc-deploy` inline policy (direct AWS CLI, role not in CF)
- [ ] Trigger staging-deploy.yml via workflow_dispatch
- [ ] Monitor deploy run to completion
- [ ] Verify BFF /healthz returns 200

## Blockers
None

## ETA
~15 minutes
