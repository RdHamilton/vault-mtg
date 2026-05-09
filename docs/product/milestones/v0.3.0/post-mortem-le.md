# v0.3.0 Post-Mortem — Lead Engineer Findings

**Author**: Lead Engineer
**Date**: 2026-05-09
**For**: PM synthesis into final v0.3.0 post-mortem

---

## 1. Why Did #1514 Ship Without Repo Integration Tests?

### What happened

PR #1609 (backend-engineer, ticket #1514) contained handler-level tests and a clean CLAUDE.md compliance pass, but the three new `StatsRepository` methods (`ListDraftAnalytics`, `ListRankProgression`, `GetResultBreakdown`) had zero repository integration tests at the time the PR was submitted. The LE review caught this as an AC gap and marked the PR APPROVED-with-gap rather than BLOCKED — because the production code was correct and all other checks passed. The tests were split into a follow-up PR (#1610), which then required a separate LE review cycle and merge.

### Root cause

The CLAUDE.md test coverage rule ("Add integration tests for backend changes — repository, handlers, services") was present but not enforced at the moment of implementation. The implementing agent appears to have treated handler-level tests as sufficient evidence of coverage without checking whether repo-layer tests existed. The AC for #1514 did explicitly require "query-level tests (repository tests against real DB)" — but no pre-PR self-check caught the gap before `gh pr create` was called.

### Rule to add — Test Coverage Checkpoint (Backend)

**Before opening any PR that adds a new `*Repository` method, the implementing agent MUST:**

1. Run: `grep -r "TestStatsRepository\|TestMatchRepository\|Test.*Repository" services/bff/internal/storage/repository/ --include="*_test.go" -l`
2. Verify that a `*_test.go` file exists in the same package as every new exported repository method.
3. Verify that each new method has at least one `Test<Type>_<Method>_*` test case using the `openTestDB(t)` / `t.Skip("TEST_DATABASE_URL not set")` pattern.
4. If any method is missing a test: **do not open the PR**. Write the tests first.

This check happens before `gh pr create`, not after. LE should also enforce it as a hard BLOCK (not a soft gap) when repo integration tests are absent for new exported methods.

---

## 2. Why Did the Infra Agent Contaminate PR #1607 with #1513 Code?

### What happened

PR #1607 was scoped to CI fixes (#1524): shellcheck violations and the SSE 503 E2E failure. The infra agent committed `StatsHandler` wiring into `services/bff/cmd/main.go` — referencing `handlers.StatsHandler` and `repository.NewStatsRepository` — which belonged to #1513 (a separate ticket worked by backend-engineer). Because those types did not exist on `main` or on the #1524 branch, CI failed with `undefined: handlers.StatsHandler` across three jobs (Build BFF binary, BFF Unit Tests, Go Lint). The LE review caught this and issued a BLOCKED verdict requiring removal of the out-of-scope wiring before the PR could merge.

### Root cause

The infra agent was working on CI fixes at the same time backend-engineer was implementing #1513 stats endpoints in a local working tree. The infra agent appears to have committed from a working directory that already contained uncommitted #1513 changes, or cherry-picked/rebased across branch boundaries. The pre-commit check (`git diff --cached --name-only`) was not run to verify that only CI-related files were staged. The Task Scope Enforcement rule ("before any commit, ask: was I explicitly instructed to do this?") was violated.

### Rule to add — Branch Hygiene Pre-Commit Check

**Before every `git commit` on a ticket branch, the implementing agent MUST run:**

```bash
git diff --cached --name-only
```

And verify that every staged file belongs to the current ticket's scope. Specific gates:

- If the commit is for a CI/infra ticket (`fix/1524-*`): no files under `services/` may appear in the staged list unless the ticket explicitly says to touch application code.
- If any staged file is outside the ticket's declared scope: unstage it (`git restore --staged <file>`) and stop. Do not commit. Report the out-of-scope change to PM.
- After unstaging, re-run `go vet ./...` to confirm the remaining commit compiles cleanly before committing.

This rule is an extension of the existing Task Scope Enforcement Standing Order — it adds a concrete, machine-checkable step rather than relying on the agent's self-assessment.

---

## 3. What Pre-PR Checks Would Have Caught Both Issues Earlier?

### Current state

Both failures were caught by LE during PR review — after CI had already run (and failed for the #1607 contamination) or after the PR was open (for the #1514 test gap). The implementing agents did not run pre-PR checks equivalent to what LE runs.

### Pre-PR Self-Check Protocol (implementing agent, mandatory before `gh pr create`)

The implementing agent MUST run all of the following and confirm each passes before opening a PR. If any step fails, fix it before opening.

**For any Go change:**

```bash
# 1. Staged files are in scope
git diff --cached --name-only

# 2. Code compiles
cd services/bff && go vet ./...

# 3. All existing tests still pass
cd services/bff && go test -race ./...

# 4. Formatting clean
gofumpt -l services/bff/

# 5. Secrets scan
git diff main...HEAD -- . | grep -iE "(api_key|secret|password|token|sk_live|sk_test|AKIA[A-Z0-9]{16})" | grep "^+" | grep -v "^+++"
```

**For tickets that add new repository methods (additional check):**

```bash
# 6. Repo integration tests exist for every new exported method
grep -r "func Test" services/bff/internal/storage/repository/ --include="*_test.go" | grep -i "<NewMethodName>"
```

If step 6 returns no results: write the tests before opening the PR.

**Benefit**: Both v0.3.0 incidents would have been caught at step 1 (infra contamination) and step 6 (missing repo tests) respectively — before CI ran, before LE was invoked, and before a second PR was needed.

---

## 4. Reducing First-Pass Block Rate: Pre-Review Checklist

### What rework cost us

The v0.3.0 BLOCKED → fix → re-review cycle added at minimum two full review round-trips:
- PR #1607: BLOCKED by LE for CI contamination → infra agent removed out-of-scope wiring → re-review → MERGED
- PR #1609: APPROVED-with-gap by LE → backend-engineer opened PR #1610 for missing tests → second LE review → MERGED

Each round-trip consumed LE agent time, CodeRabbit credits (CodeRabbit was rate-limited during this period), and extended the critical path before the v0.3.0 release tag could be cut.

### Pre-Review Checklist (implementing agent, mandatory, in PR description)

Every PR description MUST include a completed checklist before LE is invoked. LE will reject (not review) any PR missing this section.

```markdown
## Pre-Review Checklist

- [ ] Staged files verified — only files belonging to ticket #XXXX are committed (`git diff --cached --name-only` reviewed)
- [ ] `go vet ./...` passes (or `npx tsc --noEmit` for frontend PRs)
- [ ] `go test -race ./...` passes (or `npm run test:run` for frontend PRs)
- [ ] `gofumpt` run on all changed `.go` files (Go PRs only)
- [ ] Secrets scan clean (no `api_key|secret|token|sk_*|AKIA` in diff)
- [ ] For new repo methods: integration test exists using `openTestDB(t)` pattern
- [ ] For new routes: route is inside `ClerkAuthMiddleware`-protected group OR explicitly documented as public
- [ ] For frontend UI changes: Playwright E2E spec added or updated
- [ ] AC items from the ticket listed and each marked PASS/FAIL
```

If any box is unchecked: the implementing agent must resolve it before requesting LE review. LE will not review PRs with unchecked boxes — it will post a single comment pointing to this rule and close the review.

---

## 5. Making LE Review Faster Without Sacrificing Quality

### Current bottlenecks identified

1. **LE re-reads the entire CLAUDE.md on every review.** For PRs that don't touch auth or frontend, the auth route audit and UX spec audit are N/A but still consume review time.
2. **LE runs security checks (`npm audit`, `govulncheck`, secrets scan) even when a PR is trivially scoped** (e.g., a test-only PR like #1610). These are correct to run but could be ordered to fail fast.
3. **LE posts a full review comment even for trivial approvals**, which adds noise to the PR timeline.
4. **First-pass blocks cause a full second review** rather than a focused re-check of only the flagged items.

### Protocol Changes

**5a. Tiered review scope (determined by diff file list)**

| PR type | Auth audit | UX spec audit | Security checks | Test verification |
|---|---|---|---|---|
| Test-only (no prod code) | Skip | Skip | Secrets scan only | Confirm test pattern correct |
| CI/infra only (`.github/`, scripts) | Skip | Skip | Secrets scan + shellcheck | N/A |
| Backend feature (Go, no auth files) | Skip | Skip | Full (govulncheck + secrets) | AC + repo test check |
| Frontend feature | Skip | Run if spec linked | npm audit + secrets | AC + Playwright check |
| Auth file touched | Mandatory | N/A | Full | Mandatory |

Rule: LE determines tier from `git diff main...HEAD --name-only` before doing anything else. Skipped checks are noted in the review comment as "N/A (tier: X)" — not silently omitted.

**5b. Re-review is scoped, not full**

When a PR is re-submitted after a BLOCKED verdict:
- LE reads only the delta since the previous review commit SHA.
- LE re-runs only the checks that previously failed.
- If all previously-failed checks now pass and no new files were added: APPROVED immediately.
- LE does not re-run the full security suite on a re-review unless new files were added.

**5c. Single consolidated comment, always**

LE posts exactly one comment per PR (existing rule). For approvals of test-only or trivially-scoped PRs, the comment may be as short as three lines:

```
## LE Review — APPROVED

Tier: test-only. Secrets scan clean. Test pattern correct (openTestDB, skip-on-no-TEST_DATABASE_URL).
Merged.
```

**5d. Pre-PR checklist gates LE invocation**

If the implementing agent's PR description does not include a completed Pre-Review Checklist (see Section 4), LE posts a single comment:

> "Pre-Review Checklist missing or incomplete. Complete it and re-request review."

LE does not read the diff until the checklist is present. This eliminates reviews that would end in a BLOCK for a mechanical reason the agent should have caught itself.

**5e. Fail-fast ordering within a review**

LE runs checks in this order, stopping at first failure:
1. Staged-file scope check (catches contamination in seconds)
2. `go vet` / `tsc` (catches compilation errors before reading the diff)
3. Secrets scan (non-negotiable, fast)
4. Test coverage check (catches missing tests before reading logic)
5. CLAUDE.md compliance review of the diff
6. AC verification against ticket

Stopping at first failure means LE does not spend time on compliance review when the code doesn't compile, and does not spend time on AC verification when tests are missing.

---

## Summary of New Rules (actionable, copy to BROADCAST / agent definitions)

| Rule | Where to enforce | Trigger |
|---|---|---|
| Repo integration test required for every new exported repository method | Implementing agent pre-PR | Any new `func (r *<X>Repository) <Method>` |
| `git diff --cached --name-only` before every commit | Implementing agent | Every `git commit` |
| `go vet ./... && go test -race ./...` before `gh pr create` | Implementing agent | Every Go PR |
| Pre-Review Checklist required in PR description | LE gates on it | Every PR |
| Tiered LE review scope | LE | Determined by `--name-only` diff |
| Re-review scoped to delta since last review SHA | LE | Any re-submitted BLOCKED PR |
| Missing checklist → single comment + no review | LE | Missing/incomplete checklist |
| Fail-fast check ordering in LE review | LE | Every review |
