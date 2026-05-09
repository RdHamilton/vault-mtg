---
name: BROADCAST
description: Shared operational directives broadcast to all agents. Read at the start of every task. PM and Ray are the only ones who may update this file.
---

# Agent Broadcast — VaultMTG

**Last updated**: 2026-05-09
**Updated by**: PM (v0.4.0 kickoff cleanup)

---

## Current Wave

**v0.4.0 — "Beta Launch"**
- Board: #30 (Project ID: `PVT_kwHOABsZ684BW67K`, status field: `PVTSSF_lAHOABsZ684BW67KzhSLc30`, Todo option: `cf9b61a3`)
- v0.3.0 tag: CUT ✓ (2026-05-09)
- Current focus: Wave 0 — architecture brittleness fixes (T1–T5 from arch assessment) before any new feature work
- Ray is doing manual testing of v0.3.0; bugs go to Board #30 as they are filed

---

## Active Directives

Read these before picking up any ticket. They override default agent behavior.

1. **Wave 0 gate — do not start engineering** — All pre-Wave-0 GATE items (A1–A10 from `docs/product/milestones/v0.4.0/kickoff.md` Section 3) must be confirmed complete before any Wave 0 ticket moves to In Progress on the v0.4.0 project (#30). Outstanding gates: A2 (agent permission audit in `docs/engineering/release-checklist.md` §0), A3 (PC-4 in BROADCAST Standing Orders), A4 (PC-9 in BROADCAST Standing Orders). Do not pick up T1–T5 until PM confirms all gates are green.
2. **PC-2: CI is a hard gate** — No wave closes with CI red. No release tag is cut with CI red. "Unrelated" failures still block — a broken pipeline means the codebase is not shippable. PM must verify CI status at time of writing any wave-close report.
3. **PC-6: v0.4.0 project is #30** — Project ID `PVT_kwHOABsZ684BW67K`, Milestone #68. Do NOT reference Project #29 (v0.3.0) or any prior version project. All ticket moves, board queries, and project-item-add calls use #30.
4. **All wave-close reports require LE co-sign** — PM writes the report; LE verifies ACs are met in the merged PR diffs. Both names must appear. No wave closes without both sign-offs.
5. **Dependency-coupled tickets must ship together** — Tickets with a same-PR-or-immediately-after dependency may not be split across waves. LE must reject PRs that violate this. If a foundation ticket cannot ship in its intended wave, that wave does not close.
6. **Staging proven healthy before any release tag** — Before cutting a release tag: (a) run staging deploy pipeline from scratch, (b) verify BFF `/healthz` returns 200, (c) run Playwright staging smoke suite, (d) confirm all smoke tests pass.

---

## Freeze Flags

- [ ] **Code freeze** (inactive)
- [ ] **Release freeze** (inactive — v0.3.0 tag cut 2026-05-09 ✓)
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
- **PC-4 (branch cleanliness)**: Every agent runs `git status && git log --oneline -5` before opening any PR. If the working tree is dirty or contains unexpected commits, STOP and report back — do not open the PR.
- **PC-9 (agent invocation mode)**: Use synchronous invocation for output-producing tasks (research, status checks, reports). Use `run_in_background: true` only for state-update tasks (ticket moves, board updates, GitHub notifications).
- **PC-10 (LE review after every PR — orchestrator rule)**: The PostToolUse hook only fires in the main session. When a subagent reports back that it created a PR, the main orchestrator (Claude Code) MUST immediately spawn a background general-purpose agent to run the full LE review — `run_in_background: true`. Prompt: "You are the lead engineer for MTGA-Companion. Review PR #<NUMBER> and run the full Post-PR Review Protocol from `.claude/agents/lead-engineer.md`. Repo: /Users/ramonehamilton/Documents/Personal Projects/MTGA-Companion." Never let a PR sit unreviewed because it was opened from a subagent session.

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
