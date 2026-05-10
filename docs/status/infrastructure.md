# Infrastructure Agent -- Current Task Status
**Updated**: 2026-05-10
**Task**: Trigger staging deploy post-PR-#1732 merge and verify end-to-end
**Status**: Blocked -- awaiting LE review of PR #1733

## Progress
- [x] Read changelog and broadcast
- [x] Triggered staging deploy run 25632039679
- [x] Monitored run -- FAILED at "Provision staging env on EC2" (exit code 254)
- [x] Root cause identified (bash -e + aws CLI exit code 254 interaction)
- [x] Fix applied -- PR #1733 opened
- [ ] LE review and merge PR #1733
- [ ] Re-trigger staging deploy
- [ ] Verify BFF /healthz returns 200

## Root Cause

All four SSM polling loops in staging-deploy.yml use this pattern:

    RAW=$(aws ssm get-command-invocation ... 2>&1); RC=$?

GHA runs steps with `bash -e`. On the first poll iteration, SSM returns
`InvocationDoesNotExist` (exit code 254 from AWS CLI) because the
command was just dispatched. With `bash -e`, the command substitution
`RAW=$(...)` exits the step immediately with exit 254 before `RC=$?`
can be checked -- the error-handling `if [ "$RC" -ne 0 ]` block never
runs.

Proof: SSM commands ARE completing successfully on EC2. Run
25632039679: Command ID 8eb7cc48-368b-446a-9c83-5365d5a02a4b completed
with Status: Success, ResponseCode: 0. Only the GHA polling exits
prematurely.

## Fix

PR #1733: add `set +e` before and `set -e` after each
get-command-invocation call in all four polling loops.
Branch: fix/staging-deploy-errexit-ssm-poll

## Blockers
Awaiting lead-engineer review of PR #1733 (Protected File Policy:
.github/workflows/ requires LE review before merge).
