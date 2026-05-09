# VaultMTG Agent Org Chart

**Owner: Najah (Product Manager)**
**Last updated**: 2026-05-09

---

## Reporting Structure

```
┌─────────────────────────────────────────────────────┐
│                    Ray Hamilton                     │
│              (CTO / Technical Authority)            │
└──────────┬──────────────────────────────────────────┘
           │
     ┌─────┴──────┐
     ▼            ▼
┌──────────┐  ┌─────────────────────────────────────┐
│  Najah   │  │              architect               │
│  (PM)    │  │     (system design, ADRs)            │
└────┬─────┘  └──────────────┬──────────────────────┘
     │                       │
     │                       ▼
┌────┴─────────┐  ┌─────────────────────────────────┐
│   project    │  │           lead-engineer          │
│   manager    │  │     (code review, PR gating)     │
└──────────────┘  └───┬──────────┬──────┬───────────┘
                      │          │      │
           ┌──────────┤    ┌─────┤   ┌──┴──────────┐
           ▼          ▼    ▼     ▼   ▼             ▼
       ┌──────┐  ┌──────┐ ┌────┐ ┌──────────┐ ┌──────────┐
       │  be  │  │  fe  │ │dba │ │  infra   │ │ui-tester │
       │  eng │  │  eng │ │    │ │          │ │          │
       └──────┘  └──┬───┘ └────┘ └──────────┘ └──────────┘
                    │
                    ▼
              ┌────────────┐
              │ux-designer │
              └────────────┘

  ┌─────────────────────────────────────────┐
  │           Business Unit (flat)          │
  │  business-analyst  │  finance-controller│
  │  customer-success  │  growth-marketing  │
  └─────────────────────────────────────────┘
        (all report to Najah / product-manager)
```

---

## Agent Roster

| Agent | Named | Model | Primary Responsibility |
|---|---|---|---|
| **product-manager** | Najah | opus | Roadmap, PRDs, wave ceremonies, backlog prioritization. Routes all business unit input into engineering. |
| **architect** | Ray | opus | System design, ADRs, cross-cutting decisions, service boundaries. |
| **project-manager** | — | haiku | GitHub issues, project boards, labels, ticket status transitions. |
| **lead-engineer** | — | sonnet | Code review, CLAUDE.md compliance, PR gating (APPROVED/BLOCKED), CI gate. |
| **backend-engineer** | — | sonnet | Go BFF, daemon binary, repositories, migrations, middleware. |
| **front-engineer** | — | sonnet | React SPA, Vite config, Storybook, Playwright E2E. |
| **dba** | — | sonnet | PostgreSQL schema, migrations, index strategy, query optimization, RDS config. |
| **infrastructure** | — | sonnet | CloudFormation, CI/CD, GitHub Actions, EC2/RDS, S3/CloudFront. |
| **ui-tester** | — | sonnet | Playwright E2E and Vitest component tests. |
| **ux-designer** | — | sonnet | Design tokens, Tailwind specs, component layouts, Chromatic review. |
| **business-analyst** | — | haiku | PostHog analytics, KPIs, cohort analysis, acquisition reports. |
| **customer-success** | — | haiku | Discord, Crisp, Typeform, Notion, GitHub issue triage. |
| **growth-marketing** | — | haiku | SEO, Buffer, Discord announcements, Mailchimp campaigns. |
| **finance-controller** | — | haiku | AWS costs, P&L, burn rate, unit economics, budget signals to PM. |
| **general-purpose** | — | sonnet | Ad-hoc research, one-off tasks not owned by a named agent. |

---

## Key Handoff Relationships

| From | To | Trigger |
|---|---|---|
| Ray (CTO) | architect | New system-level decision needed; ADR required |
| Ray (CTO) | Najah (PM) | Product direction, priority calls, scope decisions |
| Najah (PM) | project-manager | All GitHub issue creation and ticket transitions |
| Najah (PM) | lead-engineer | Wave kickoff GO; PR merge authorization |
| Najah (PM) | architect | Wave kickoff architectural review (required per-wave) |
| architect | lead-engineer | ADR signed off; implementation constraints communicated |
| lead-engineer | backend-engineer / front-engineer / dba / infrastructure | PR feedback, implementation direction |
| front-engineer | ux-designer | Design spec verification before PR submission |
| ux-designer | front-engineer | Approved design spec; Chromatic story review |
| business-analyst | Najah (PM) | Weekly metrics; Growth action recommendations |
| customer-success | business-analyst | CS theme trigger (3+ same issue in one week → quantify in PostHog) |
| finance-controller | Najah (PM) | Budget alerts (>20% over, runway <6 months, new high-cost service) |
| growth-marketing | Najah (PM) | Copy sign-off required before any external post is scheduled |

---

## Phase 2+ Planned Roles

These roles do not exist yet. They are created when the listed trigger condition is met.

| Agent | Splits From | Creation Trigger | Phase |
|---|---|---|---|
| **security-agent** | lead-engineer | First external security audit or Stripe integration begins | Phase 2 (v1.0.0, ~500–2,000 users) |
| **devrel-agent** | growth-marketing | Discord community exceeds 500 members | Phase 2 (v1.0.0, ~500–2,000 users) |
| **sre-agent** | infrastructure | First production SLA commitment to users | Phase 2 (v1.0.0, ~500–2,000 users) |

For the full evolution roadmap (Phase 3 growth, Phase 4 scale) see `docs/org/agent-ecosystem-analysis.md`.
