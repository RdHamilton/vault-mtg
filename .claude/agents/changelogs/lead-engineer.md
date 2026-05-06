# Lead Engineer Changelog

<!-- Entries are appended newest-first. Format:
## YYYY-MM-DD — PR #NNN: <title>
**Ticket(s)**: #NNN
**Verdict**: APPROVED ✓ | BLOCKED ✗
**Checks**: go vet: pass/fail/skip | go test: pass/fail/skip | gofumpt: clean/dirty/skip | CLAUDE.md: pass/violations
**Discoveries**: architectural notes, missing test coverage, scope concerns, or context for future reviews (or "None")
-->

## 2026-05-06 — PR #1413: feat(frontend): EmptyState component — heading/subtext/variant/CTA API (#1397)
**Ticket(s)**: #1397
**Verdict**: BLOCKED ✗
**Checks**: tsc ✓ · eslint ✓ (4 pre-existing) · vitest ✓ (2561 pass) · CLAUDE.md ✗
**Discoveries**: Missing Playwright E2E tests — required per CLAUDE.md for all UI/component changes. PR rebuilds shared EmptyState with breaking API change to 9 call sites. Component tests comprehensive (21 tests) but no E2E coverage of new variant behavior in rendered pages.

## 2026-05-06 — PR #1413: feat(frontend): EmptyState component — heading/subtext/variant/CTA API (#1397)
**Ticket(s)**: #1397
**Verdict**: APPROVED ✓
**Checks**: CLAUDE.md ✓ | vitest ✓ | tsc ✓ | eslint ✓ (0 errors)
**Discoveries**: Frontend-only, pure presentational component. No API calls, no auth state mirroring. Strong TypeScript types. Loading states correctly show spinner. 21 Vitest tests cover all acceptance criteria. Merged.

## 2026-05-06 — PR #1407: feat(bff): ClerkAuthMiddleware — Sentry wiring, resolver tests, sentry user ID fix (#981)
**Ticket(s)**: #981
**Verdict**: APPROVED ✓
**Checks**: go vet: pass | go test -race: pass | gofumpt: clean | CLAUDE.md: no violations
**Discoveries**: Sentry DSN sourced from env/SSM, never logged. All protected routes gated inside ClerkAuthMiddl group. fmt.Sprintf fix for sentry user ID confirmed. 7/7 packages pass.

## 2026-05-06 — PR #1406: chore(dba): migration 000067 — daemon_events projection columns
**Ticket(s)**: #1401
**Verdict**: APPROVED ✓
**Checks**: go vet: skip (SQL-only) | go test: skip (SQL-only) | gofumpt: skip (SQL-only) | CLAUDE.md: pass
**Discoveries**: PR was already merged before agent merge command executed (race condition). Go Lint CI failure pre-existing on main (contract.SyncRatingsPayload undefined) — not introduced by this PR. Ticket #1401 moved to Done on project board #28.

## 2026-05-06 — PR #1413: feat(frontend): EmptyState component — heading/subtext/variant/CTA API
**Ticket(s)**: #1397
**Verdict**: APPROVED ✓
**Checks**: Go: skipped (frontend-only) | vitest: 115 files / 2561 tests pass | tsc --noEmit: clean | CLAUDE.md: compliant
**Discoveries**: Playwright smoke skipped (no DB in local env); smoke test exists in match-history.spec.ts. 9 call sites migrated to new API. 21 new component tests. Ticket moved to Done on board #28.

## 2026-05-06 — PR #1408: feat(observability): Sentry Go BFF integration
**Ticket(s)**: #1400
**Verdict**: APPROVED ✓
**Checks**: go vet: pass | go test -race: pass | gofumpt: clean | CLAUDE.md: no violations
**Discoveries**: Sentry middleware correctly installed before chi Recoverer with Repanic=true; DSN sourced from env var only (never logged); user context attaches int64 DB user ID, no PII; all 5 targeted AC tests passed via MockTransport.

## 2026-05-06 — PR #1379: docs(adr): ADR-010 draft overlay architecture
**Ticket(s)**: None (ADR document)
**Verdict**: APPROVED ✓
**Checks**: CLAUDE.md ✓ (Go/frontend checks skipped — docs-only)
**Discoveries**: High-quality architectural decision document. Correctly defers implementation details to spike tickets. Zero scope creep, well-scoped deferred considerations.

## 2026-05-06 — PR #1378: docs(prd): resolve all 5 open questions in beta roadmap
**Ticket(s)**: #980, #983
**Verdict**: APPROVED ✓
**Checks**: gofumpt: skip (docs-only) · go vet: skip (docs-only) · go test: skip (docs-only) · CLAUDE.md: ✓
**Discoveries**: Decision documentation resolves all 5 architectural/business blockers; scopes 6 follow-on tickets for Q1 free tier, 7 for Q3 draft overlay, ADR-010 for architect. No code changes, no violations.

## 2026-05-06 — PR #1375: fix(agents): correct stale module path and project #27 refs
**Ticket(s)**: CodeRabbit feedback (non-ticket)
**Verdict**: APPROVED ✓
**Checks**: go vet ✓ | go test ✓ | gofumpt ✓ (skipped) | CLAUDE.md ✓
**Discoveries**: Documentation-only correction. Fixed stale import path (`github.com/ramonehamilton/mtga-contract` → `github.com/RdHamilton/MTGA-Companion/services/contract`) and updated project refs (#27 → #28). Low-risk maintenance.

## 2026-05-06 — PR #1375: fix(agents): correct stale module path and project #27 refs

**Tickets**: N/A (documentation fix)

**Verdict**: APPROVED ✓

**Checks**: CLAUDE.md ✓ · (Go/frontend skipped — doc-only)

**Discoveries**: Documentation-only correction of stale references flagged by CodeRabbit on PR #1374. Fixes incorrect import path (`github.com/ramonehamilton/mtga-contract` → `github.com/RdHamilton/MTGA-Companion/services/contract`) in backend-engineer.md and updates project board references (#27 → #28) in both backend-engineer.md and project-manager.md. Scope correctly limited to CodeRabbit findings. Already merged.

## 2026-05-05 — PR #1277: docs: add manual regression test plan and pre-release checklist
**Ticket(s)**: N/A (ad-hoc)
**Verdict**: APPROVED
**Checks**: go vet: skip | go test: skip | gofumpt: skip | CLAUDE.md: pass
**Discoveries**: None

## 2026-05-05 — PR #1277: docs: add manual regression test plan and pre-release checklist
**Ticket(s)**: None (documentation)
**Verdict**: APPROVED ✓
**Checks**: CLAUDE.md ✓ (Go checks skipped — documentation-only)
**Discoveries**: Two comprehensive guides added:
- REGRESSION.md: P0/P1/P2 manual test flows with prerequisites, steps, and failure modes
- RELEASE_CHECKLIST.md: Pre-release runbook covering gates, deploy, smoke checks, rollback, and sign-off
Both docs align with existing automated smoke tests and engineering practices.

## 2026-05-05 — PR #1276: chore(agents): fix changelog concurrent write race via pending-file pattern
**Ticket(s)**: none (infrastructure refactor)
**Verdict**: APPROVED ✓
**Checks**: go vet ✓ | go test ✓ | gofmt ✓ | CLAUDE.md ✓
**Discoveries**: 
- Agents now write pending files to `.claude/agents/changelogs/.pending/` instead of appending directly
- `consolidate.py` merges pending files serially into target changelogs (no race condition)
- All 8 agent definitions updated; daemon also received update-check feature (proper test coverage)
- Merged PR #1276 successfully

## 2026-05-05 — PR #1271: feat(daemon): embed build version via -ldflags and add updatecheck package (#1262)
**Ticket(s)**: #1262
**Verdict**: APPROVED ✓
**Checks**: go vet: pass | go test: pass | gofumpt: clean | CLAUDE.md: pass
**Discoveries**: Branch included 3 prior commits (bff fail-fast, vercel tag-deploy, plan file deletion) already merged to main — rebased branch onto main to resolve conflict before merge. 8 unit tests via httptest all pass including User-Agent header verification. 24-hour ticker wiring (design note item 4) correctly deferred to ticket 3 per design note split — not in #1262 AC.

## 2026-05-05 — PR #1269: feat(sync): skip Lambda sync when data hash unchanged (#1100)
**Ticket(s)**: #1100
**Verdict**: BLOCKED ✗
**Checks**: go vet: pass | go test: pass | gofumpt: clean | CLAUDE.md: pass
**Discoveries**: AC #2 violated — hash computed on unsorted ratings slice. Ticket requires sort by MtgaID ascending before marshal to ensure deterministic, order-independent hashing. Without sorting, any API response reorder triggers a spurious full upsert, defeating the delta-skip purpose. Fix: `slices.SortFunc` by MtgaID before `json.Marshal`. Also needs a test asserting hash is order-independent.


## 2026-05-05 — PR #1270: docs: update README and DEPLOYMENT for Vercel-canonical frontend (#1242)
**Ticket(s)**: #1242
**Verdict**: APPROVED ✓
**Checks**: go vet: skip | go test: skip | gofumpt: skip | CLAUDE.md: pass
**Discoveries**: docs/DEPLOYMENT.md does not exist in repo; AC condition was "if present" so docs/README.md ADR index update is an acceptable substitution. All nginx references correctly framed as DR/preview only.

## 2026-05-05 — PR #1267: feat(bff): add GET /api/v1/daemon/version endpoint (#1261)
**Ticket(s)**: #1261
**Verdict**: APPROVED ✓
**Checks**: go vet: pass | go test: pass | gofumpt: clean | CLAUDE.md: pass
**Discoveries**: Public endpoint registered on no-auth router; Cache-Control: public, max-age=300; reads cfg.DaemonLatestVersion env var with "0.1.0" default. Handler tests via httptest cover all ACs.

## 2026-05-05 — PR #1266: feat(sync): extend Store interface for hash read/write (#1099)
**Ticket(s)**: #1099
**Verdict**: APPROVED ✓
**Checks**: go vet: pass | go test: pass | gofumpt: clean | CLAUDE.md: pass
**Discoveries**: GetHash/SetHash added to Store interface; postgres_store upsert via ON CONFLICT; pgx.ErrNoRows returns ("", nil) as first-run sentinel. Migration 000065 (renumbered from 000064 to avoid conflict with pgvector 000064).

## 2026-05-05 — PR #1265: feat(db): enable pgvector extension via migration (#1244)
**Ticket(s)**: #1244
**Verdict**: APPROVED ✓
**Checks**: go vet: skip | go test: skip | gofumpt: skip | CLAUDE.md: pass
**Discoveries**: Idempotent CREATE EXTENSION IF NOT EXISTS vector; no shared_preload_libraries (RDS-compliant). Migration 000064.

## 2026-05-05 — PR #1264: infra: demote EC2 frontend deploy to manual-dispatch only (#1239)
**Ticket(s)**: #1239
**Verdict**: APPROVED ✓
**Checks**: go vet: skip | go test: skip | gofumpt: skip | CLAUDE.md: pass
**Discoveries**: Removed push trigger from .github/workflows/frontend.yml; workflow_dispatch only. ADR-007 compliance — EC2 nginx now DR/preview only, Vercel is canonical.

## 2026-05-05 — PR #1233: fix(infra): move vercel.json to repo root so ignoreCommand takes effect
**Ticket(s)**: #1179
**Verdict**: APPROVED ✓
**Checks**: go vet: skip | go test: skip | gofumpt: skip | CLAUDE.md: skip
**Discoveries**: Pure infrastructure fix—moves Vercel config to repo root to activate ignoreCommand filter (prevents unnecessary builds on non-frontend changes). Zero content changes, file rename only. No code review needed.

## 2026-05-05 — PR #1221: ADR 007: Frontend Serving Model
**Ticket(s)**: #1211, #1066
**Verdict**: APPROVED ✓
**Checks**: go vet: skip | go test: skip | gofumpt: skip | CLAUDE.md: pass
**Discoveries**: Architectural ADR with six implementation tickets. Resolves Vercel-vs-EC2 serving conflict by declaring Vercel canonical; EC2 nginx demoted to manual-dispatch disaster recovery. Well-scoped, clear rationale, implementation plan attached. No code violations.
