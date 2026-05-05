# Lead Engineer Changelog

<!-- Entries are appended newest-first. Format:
## YYYY-MM-DD — PR #NNN: <title>
**Ticket(s)**: #NNN
**Verdict**: APPROVED ✓ | BLOCKED ✗
**Checks**: go vet: pass/fail/skip | go test: pass/fail/skip | gofumpt: clean/dirty/skip | CLAUDE.md: pass/violations
**Discoveries**: architectural notes, missing test coverage, scope concerns, or context for future reviews (or "None")
-->

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
