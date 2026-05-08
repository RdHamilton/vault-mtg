# Agent Ecosystem Analysis — VaultMTG
**Last updated**: 2026-05-08

---

## Current Org Map

| Agent | Model | Role | Key Tools |
|---|---|---|---|
| architect | opus | System design, ADRs, cross-cutting decisions | Read, Bash, Grep |
| product-manager | sonnet | Roadmap, PRDs, ceremonies, wave coordination | gh CLI, docs |
| project-manager | haiku | GitHub issues, boards, labels, ticket transitions | gh CLI |
| lead-engineer | sonnet | Code review, compliance, PR gating, CI gate | Bash, Read, Grep |
| backend-engineer | sonnet | Go BFF, daemon, repositories, migrations | Bash, all tools |
| front-engineer | sonnet | React SPA, Vite, Playwright E2E | Bash, all tools |
| dba | sonnet | PostgreSQL schema, migrations, indexes, query optimization | Bash, all tools |
| infrastructure | sonnet | CloudFormation, CI/CD, GitHub Actions, EC2/RDS | Bash, all tools |
| ui-tester | sonnet | Playwright E2E, Vitest component tests | Bash, all tools |
| ux-designer | sonnet | Design tokens, Tailwind specs, component layouts | WebSearch, Read |
| business-analyst | haiku | PostHog analytics, KPIs, cohort analysis, reports | WebSearch, Bash |
| customer-success | haiku | Discord, Crisp, Typeform, Notion, GitHub issues | Bash, WebFetch |
| growth-marketing | haiku | SEO, Buffer, Discord announcements, Mailchimp | WebSearch, Bash |
| finance-controller | haiku | AWS costs, P&L, burn rate, unit economics | Bash, WebFetch |
| general-purpose | sonnet | Ad-hoc research, one-off tasks | All tools |

---

## Interaction Gaps

### 1. Growth agent invents unshipped features (FIXED 2026-05-08)
**Problem**: Growth drafts copy based on what sounds good, not what's merged. PM rejected 6 posts this wave for fabricated claims (letter grades, archetype analysis, beta user counts).
**Fix applied**: Added mandatory `gh pr list` verification step before any copy is written. Every factual claim must cite a merged PR. PM sign-off required before scheduling.

### 2. No formal CS → BA trigger
**Problem**: CS collects qualitative signals ("5 users can't connect the daemon") but never formally triggers BA to quantify the impact in PostHog. Qualitative and quantitative streams are siloed.
**Fix needed**: When CS identifies a theme appearing 3+ times in a week, CS should explicitly hand off to BA: "Can you check PostHog for drop-off at the daemon connection step?" Add this to CS's weekly feedback workflow.

### 3. Finance → PM budget signal is missing
**Problem**: Finance produces monthly P&L but PM makes roadmap decisions without budget context. If AWS costs spike, PM doesn't know to deprioritize infrastructure-heavy features.
**Fix needed**: Finance should flag PM immediately when: (a) a cost center exceeds budget by >20%, (b) runway drops below 6 months, (c) a new service is proposed that would materially increase burn.

### 4. BA → Growth handoff is one-directional
**Problem**: BA identifies acquisition sources in weekly metrics but doesn't tell Growth what to do with that data. "Reddit is sending 3x more engaged users than X" should trigger a Growth response — but it doesn't.
**Fix needed**: BA weekly report should include a "Growth actions" section: explicit recommendations for growth agent based on acquisition data.

### 5. Architect is reactive, not proactive
**Problem**: Architect writes ADRs when asked but doesn't review Wave kickoffs for architectural implications. New features get built without architectural guidance unless someone remembers to ask.
**Fix needed**: PM should explicitly loop architect into every Wave kickoff for a 1-pass architectural review. Architect should produce a brief "architectural implications" note for each wave.

### 6. No incident response protocol
**Problem**: When production breaks, there's no defined owner or runbook. Infrastructure fixes CI/CD but prod outages are handled ad-hoc. As beta users arrive, this becomes a real gap.
**Fix needed**: Infrastructure should own on-call and incident response. Define: P0 (service down), P1 (degraded), P2 (minor). Infrastructure agent gets first page, loops in BE or DBA as needed.

### 7. DBA is siloed
**Problem**: DBA is only invoked for schema changes. Doesn't proactively monitor query performance, index health, or slow queries.
**Fix needed**: DBA should run a proactive health check at the start of each wave: top 5 slowest queries, index usage, bloat. Produce a 1-page report.

### 8. No security ownership
**Problem**: Nobody owns CVE scanning, dependency audits, or security reviews. LE checks compliance but not security vulnerabilities. No one runs `npm audit` or `go mod` CVE checks proactively.
**Fix needed (now)**: Add security checklist to LE: `npm audit` on frontend, `govulncheck` on Go modules, check for exposed secrets. Fix needed (v0.5.0): dedicated security agent.

### 9. UX → FE handoff is informal
**Problem**: UX designer produces specs but there's no formal handoff to FE or verification that specs were followed. FE may deviate without knowing.
**Fix needed**: UX should file a GitHub issue (via project-manager) for each design spec. FE references the issue. LE checks that FE implementation matches the spec in review.

### 10. Project-manager vs PM boundary is fuzzy
**Problem**: PM sometimes creates GitHub issues directly instead of delegating to project-manager. Project-manager sometimes moves tickets without PM direction.
**Fix**: PM owns strategy and ACs. Project-manager owns ALL GitHub issue creation and ticket transitions. PM should never create issues directly — always via project-manager.

### 11. No scheduled weekly reporting cadence
**Problem**: BA, CS, Finance, and Growth all produce reports but only when explicitly asked. No automated Monday morning pulse.
**Fix needed**: Set up scheduled routines (via `/schedule`) for:
- BA: weekly metrics report every Monday
- CS: weekly feedback summary every Monday  
- Finance: monthly P&L first Monday of month
- Growth: monthly acquisition report first Monday of month

---

## Evolution Roadmap

The goal: run the agent team like a growing startup. As complexity and user count increase, roles specialize and new ones emerge.

### Phase 1 — Beta (Now, v0.4.0, ~0-500 users)
Current org is appropriate. Priority fixes:
- Wire up the CS→BA trigger
- Add architect to wave kickoffs
- Set up weekly scheduled reporting
- Add security checklist to LE
- Define incident response protocol

### Phase 2 — Post-Beta Launch (v1.0.0, ~500-2,000 users)
New roles needed:

| New Agent | Splits From | Trigger |
|---|---|---|
| **security-agent** | lead-engineer | First external security audit, Stripe integration |
| **devrel-agent** | growth-marketing | Discord community exceeds 500 members |
| **sre-agent** | infrastructure | First prod SLA commitment to users |

Evolution:
- `backend-engineer` becomes **BE Lead** — reviews BE PRs before LE sees them, owns BE architecture decisions
- `lead-engineer` adds architectural review duties — starts shadowing architect on major decisions
- `growth-marketing` hands community management to devrel, focuses on acquisition only

### Phase 3 — Growth (v1.x.0, ~2,000-10,000 users)
Role promotions:

| Promotion | From | To |
|---|---|---|
| lead-engineer | Code reviewer | **Engineering Manager** — coordinates all engineering leads, owns technical strategy |
| architect | System design | **CTO-equivalent** — external partnerships, technology bets, engineering culture |
| product-manager | Roadmap owner | **Head of Product** — multiple product areas, owns strategy not just features |
| business-analyst | Reports | **Head of Data** — owns data platform, event schema governance, analytics engineering |

New splits:
- `backend-engineer` splits into: **API engineer** (BFF, auth, user data) + **platform-engineer** (daemon, log parsing, data pipeline)
- `customer-success` splits into: **support-engineer** (technical triage) + **community-manager** (Discord, relationships)
- `infrastructure` splits into: **infra-engineer** (AWS, networking) + **sre-agent** (uptime, SLOs, incidents)

### Phase 4 — Scale (v2.0.0, ~10,000+ users)
Fully specialized org:
- **billing-engineer** — Stripe integration, subscription lifecycle, dunning
- **data-engineer** — ETL pipelines, event schemas, warehouse
- **analytics-engineer** — dbt models, dashboards, experimentation platform
- **accessibility-engineer** — splits from ui-tester, owns WCAG compliance
- **content-manager** — splits from growth-marketing, owns content calendar and SEO execution

---

## Principles for Agent Growth

1. **An agent's model tier reflects their reasoning load** — haiku for structured/templated output, sonnet for implementation, opus for strategy and creative judgment
2. **Roles specialize when a single agent owns two genuinely different skill sets** — don't split just because workload increases
3. **Every new role needs a defined handoff** — who does it receive from, who does it send to
4. **Promotions happen when the promoted agent's old work can be delegated** — LE becomes EM when BE Lead can handle code review independently
5. **New roles are created for gaps, not comfort** — security agent exists because nobody owns it, not because it sounds nice
6. **Agent definitions are living documents** — update after every wave where gaps were discovered

---

## Immediate Actions (v0.4.0)

| Action | Owner | Priority |
|---|---|---|
| Add CS→BA feedback trigger | CS + BA agent definitions | High |
| Add Finance→PM budget signal | Finance agent definition | High |
| Add architect to wave kickoff protocol | PM agent definition | High |
| Add security checklist to LE | LE agent definition | High |
| Set up weekly scheduled reporting | `/schedule` routines | Medium |
| Clarify PM vs project-manager boundary | Both agent definitions | Medium |
| Wire BA→Growth acquisition handoff | BA agent definition | Medium |
| Define incident response protocol | Infrastructure agent definition | Medium |
