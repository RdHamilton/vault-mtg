# Agent Queue Prompts

One prompt per agent type. Each agent processes its queue one ticket at a time, in order.
Spawn a single agent per type — do not run the same agent type in parallel.

---

## Infrastructure Agent — Queue

```
You are the infrastructure agent for MTGA Companion. Process your ticket queue one at a time, in the order listed. Do not start the next ticket until the current one has a merged PR.

Working directory: /Users/ramonehamilton/Documents/Personal Projects/MTGA-Companion

## Queue

### 1. #1169 — Provision DAEMON_JWT_SECRET on prod EC2

**Priority**: P0 — daemon ingest returns 500s in production without this. Blocks smoke test #979.

**What to do**:
- Move `gh issue view 1169` to In Progress: `gh issue edit 1169 --add-label "in-progress"`
- The BFF reads DAEMON_JWT_SECRET from its environment to validate daemon-to-BFF JWT tokens.
  It must be provisioned on EC2 via SSM Parameter Store and injected at startup.
- Steps:
  1. Add DAEMON_JWT_SECRET to SSM Parameter Store (SecureString) in the prod AWS account
  2. Update the EC2 userdata or systemd unit to pull it from SSM at startup
  3. Add a fail-fast healthcheck in the BFF that asserts DAEMON_JWT_SECRET is non-empty on startup
  4. Update CloudFormation/CDK if applicable
  5. Update `.github/workflows/` deploy workflow to ensure the param is available
- Write integration test verifying BFF startup fails gracefully when secret is missing
- Open a PR, get it merged
- Close #1169 and update its status to Done

**Acceptance criteria**:
- [ ] DAEMON_JWT_SECRET in SSM Parameter Store (SecureString)
- [ ] EC2 BFF process reads it from SSM at startup
- [ ] BFF fails fast with a clear error if secret is absent
- [ ] Existing daemon auth flow works end-to-end in staging

After this ticket is done, your queue is empty. Exit.
```

---

## Backend Agent — Queue

```
You are the backend engineer for MTGA Companion. Process your ticket queue one at a time, in the order listed. Do not start the next ticket until the current one has a merged PR. Each PR must pass: gofumpt, go vet ./..., go test -race ./... before opening.

Working directory: /Users/ramonehamilton/Documents/Personal Projects/MTGA-Companion

## Queue (process in order)

### 1. #1099 — Delta sync: foundation
### 2. #1100 — Delta sync: diff computation  
### 3. #1101 — Delta sync: apply + persist

These three are strictly sequential — each depends on the previous.
Run `gh issue view 1099` (then 1100, 1101) for full descriptions before starting each.
Move each to In Progress when you start, Done when PR is merged.
The goal of this chain is to close #1127 (ADR-005 delta sync implementation).
After merging #1101, close parent #1127 if all sub-tickets are done.

### 4. #1160 — Daemon parser: inventory
### 5. #1161 — Daemon parser: quest log
### 6. #1162 — Daemon parser: collection
### 7. #1163 — Daemon parser: deck
### 8. #1164 — Daemon parser: match result

These five are the v2.0 Player.log parser suite. Each is independently implementable but
do them one at a time. Run `gh issue view <number>` for the full spec before starting each.
Move each to In Progress when you start, Done when PR is merged.

**For every ticket**:
- Read the ticket fully before writing any code
- Write unit tests alongside the implementation
- Add integration tests for any new repository or handler changes
- Run before opening PR: gofumpt ./... && go vet ./... && go test -race ./...
- PR body must reference the ticket number (#NNNN)

After all 8 tickets are done, your queue is empty. Exit.
```

---

## Project Manager Agent — Hygiene Queue

```
You are the project manager agent for MTGA Companion. Perform a hygiene pass on stale open tickets. Process each item below, verify current state, and take the appropriate action.

Working directory: /Users/ramonehamilton/Documents/Personal Projects/MTGA-Companion

## Queue

### 1. #1126 — Parent ticket for daemon_events persistence chain
All sub-tickets have shipped. Verify by checking if all child issues are closed.
If all children are closed: close #1126 with a comment listing the merged PRs.

### 2. #1123 — Wave 3 sync ticket (reported as shipped)
Run `gh issue view 1123`. If the implementation is in main (check git log), close it.
Comment with the commit/PR that delivered it before closing.

### 3. #1071 — Possible duplicate of #1094
Run `gh issue view 1071` and `gh issue view 1094`. 
If duplicate: close #1071 as duplicate, comment "Duplicate of #1094", add duplicate label.
If distinct: add a comment clarifying the difference and leave open.

### 4. #1165 — Likely already in repo
Run `gh issue view 1165`. Check git log / grep the codebase for the described feature.
If already implemented: close with a comment referencing the file/commit.
If not implemented: leave open and add a "confirmed-open" comment with verification steps.

**For each action taken**, add a comment to the issue explaining what you did and why.
Do not close tickets without evidence — always cite the PR, commit, or code path.

After all 4 are processed, your queue is empty. Exit.
```

---

## Usage

Spawn each agent with:
```
Agent({
  subagent_type: "infrastructure",   // or "backend-engineer", "project-manager"
  prompt: <paste the queue prompt above>
})
```

Only one agent of each type should be running at a time.
If a ticket needs clarification, the agent should post a question as a GitHub issue comment
and move on to the next ticket rather than blocking.
```
