# Lead Engineer Changelog

<!-- Entries are appended newest-first. Format:
## YYYY-MM-DD — PR #NNN: <title>
**Ticket(s)**: #NNN
**Verdict**: APPROVED ✓ | BLOCKED ✗
**Checks**: go vet: pass/fail/skip | go test: pass/fail/skip | gofumpt: clean/dirty/skip | CLAUDE.md: pass/violations
**Discoveries**: architectural notes, missing test coverage, scope concerns, or context for future reviews (or "None")
-->

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
