---
name: BROADCAST
description: Shared operational directives broadcast to all agents. Read at the start of every task. PM and Ray are the only ones who may update this file.
---

# Agent Broadcast — VaultMTG

**Last updated**: 2026-05-08
**Updated by**: LE (rogue-agent safeguard)

---

## Current Wave

**Wave 4 — v0.4.0 "Closed Beta"**
- Board: #30
- Target: August 18, 2026 (internal stretch: June 26, 2026)
- Critical path: #1524 (CI fix) → #1516 (pagination standard) → #1513/#1514 (analytics endpoints) → #1488 (security audit) → release

---

## Active Directives

Read these before picking up any ticket. They override default agent behavior.

1. **P0: CI is red on main** — Infrastructure owns #1524. No new feature work starts until CI is green and main is clean.
2. **Architect review COMPLETE — PROCEED** — `docs/arch/wave4-implications.md` is merged. Key gates before tickets start: (a) ADR-015 (pagination standard) must be written before #1513/#1514; (b) #1517 (CSP) must schedule after #1573 (Crisp); (c) #1519/#1520/#1513 partial-flag field name must be agreed on before any of the three start coding; (d) #1488 (security audit) is last.
3. **v0.3.0 release tag is blocked** — Do not cut the v0.3.0 release tag until CI is green. Infrastructure will notify PM when it's ready.
4. **ENFORCEMENT — task scope violation logged** — A rogue infrastructure agent made out-of-scope commits on 2026-05-08 (reverted a LE-approved CLERK secret, added logparse CI job, moved a ticket). The incident is resolved. All agents must read the Task Scope Enforcement rule in Standing Orders before any action.

---

## Freeze Flags

- [ ] **Code freeze** (inactive)
- [x] **Release freeze** — v0.3.0 tag blocked until CI green (#1524)
- [ ] **Merge freeze** (inactive)

---

## Standing Orders

These apply to every agent on every task and do not expire:

- **ENFORCEMENT — strict task scope**: Every agent MUST perform ONLY the work explicitly described in its assigned instruction. This is not a suggestion — it is a hard rule.
  - If your instruction says "relay a message", send the message and stop. Do NOT resolve merge conflicts, modify CI files, move tickets, or touch any code.
  - If your instruction says "check a status", read the status and report it. Do NOT write code, open PRs, or make commits.
  - Narrow instructions (relay, check, report, notify) are TERMINAL — they end with output, not with code changes.
  - Before any commit, git operation, or ticket move: ask "Was I explicitly instructed to do this?" If no: stop and report back instead.
  - Agents that violate task scope on this project will have their session terminated and their commits reverted. There is no grace period.
- **Security checklist on every PR**: LE runs `npm audit`, `govulncheck`, and secrets scan before any approval
- **Factual claims require a merged PR**: growth-marketing must cite a merged PR number for every feature claim in any copy
- **PM sign-off before public content posts**: all Reddit and X posts require PM approval before scheduling
- **Status checkpoints for long-running tasks**: infrastructure, backend-engineer, and dba must write `docs/status/{agent}.md` at each major step during tasks expected to take >5 minutes. If the same status is written 3+ times without advancing, add `## STUCK — NEEDS RESTART` as the first line of the file so PM's standup flags it to Ray immediately
- **CI test failures route to the right owner**: frontend test failures → front-engineer; Go test failures → backend-engineer; pipeline/env failures → infrastructure
- **No new issues created directly by PM**: all GitHub issue creation goes through project-manager

---

## Recent Architectural Decisions

| ADR | Decision | Impact |
|---|---|---|
| ADR-008 | CloudFront serves SPA (not Vercel) | FE deploys to S3+CloudFront; Vercel is preview-only |
| ADR-009 | Clerk is the auth provider | No custom JWT; Clerk SDK on both FE and BFF |
| ADR-014 | logparse extracted to `pkg/logparse` | CI must reference new module path (#1524) |

---

## How to Update This File

Only Ray and PM may update this file. To broadcast a new directive:
1. Edit `.claude/agents/BROADCAST.md`
2. Commit and push: `git add .claude/agents/BROADCAST.md && git commit -m "chore(broadcast): [directive description]" && git push`
3. All agents will pick it up on their next task

To clear a directive, remove it from Active Directives. To clear a freeze flag, uncheck the box.
