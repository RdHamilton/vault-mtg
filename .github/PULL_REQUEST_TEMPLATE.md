## Agent
**Agent**: `<bob | frank | ray | tim | sarah | pam | najah | greg | faye | lee>`

## Summary

- 
- 

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Refactor (no functional change)
- [ ] Infra / config
- [ ] Docs

## Linked Issue
Closes #

## Changes Made

| File | Change |
|---|---|
| `path/to/file` | Description |

## Testing
- [ ] Unit tests added/updated
- [ ] Integration tests added/updated
- [ ] Playwright E2E test added/updated
- [ ] Manually tested in local dev environment

## Auth / Security Changes
- [ ] No auth or security changes
- [ ] Yes — Sarah (Security Engineer) review required

## Rollback Plan
N/A — code-only change

## CLAUDE.md Compliance Checklist
- [ ] `gofumpt` run on all changed Go files
- [ ] `npm run lint` passes (frontend changes)
- [ ] `npm run tsc` passes (frontend changes)
- [ ] Tests exist for all new logic
- [ ] No hardcoded secrets, tokens, or credentials
- [ ] Scope limited to the linked issue — no unrelated changes

## Pre-Review Checklist
- [ ] Staged files verified — only files belonging to this ticket are committed
- [ ] `go vet ./...` passes (or `npx tsc --noEmit` for frontend PRs)
- [ ] `go test -race ./...` passes (or `npm run test:run` for frontend PRs)
- [ ] `gofumpt` run on all changed `.go` files (Go PRs only)
- [ ] For new repo methods: integration test exists using `openTestDB(t)` pattern
- [ ] For new routes: route is inside `ClerkAuthMiddleware`-protected group OR explicitly documented as public
- [ ] For frontend UI changes: Playwright E2E spec added or updated
- [ ] AC items from the ticket listed and each marked PASS/FAIL

## Local Verification
<!-- Paste real command output here. Never prose claims. -->

```
$ <command>
<output>
```

## Notes
<!-- INVARIANTS.md is a workspace-level file (not tracked in git) — P-04 added
     enforcement tags to it. Agents read it from the filesystem at task start. -->
