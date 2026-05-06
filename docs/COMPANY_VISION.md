# VaultMTG Company Vision

## North Star

**50,000 Monthly Active Users (MAU)**

Every feature decision, pricing model, infrastructure choice, and growth strategy should be evaluated against whether it moves the needle toward 50K MAU.

- A free tier that converts to paid at 5% = 2,500 paying users at scale
- Infrastructure that can't handle 50K concurrent users is a blocker
- Features that don't differentiate or retain users don't move the needle

## Mission

Build the best MTG Arena companion experience — helping players track matches, improve through draft guidance, and understand the metagame — at a scale that makes VaultMTG the go-to tool for the MTG Arena community.

---

## Organization Hierarchy

```
┌─────────────────────────────────────────────────────┐
│                    Ray Hamilton                     │
│              (CTO / Technical Authority)            │
└──────────┬──────────────────────────────────────────┘
           │
     ┌─────┴──────┐
     ▼            ▼
┌──────────┐  ┌─────────────────────────────────────┐
│ product  │  │              architect               │
│ manager  │  │     (system design, ADRs)            │
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
              │ ux-designer│
              └────────────┘

  ┌─────────────────────────────────────────┐
  │           Business Unit (flat)          │
  │  business-analyst  │  finance-controller│
  │  customer-success  │  growth-marketing  │
  └─────────────────────────────────────────┘
        (all report to product-manager)
```

## Roles

| Role | Responsibility |
|------|---------------|
| **Ray Hamilton** | CTO / single technical authority. Final call on all technical and product decisions. |
| **architect** | System design, ADRs, service boundaries, cross-cutting decisions. |
| **lead-engineer** | Code review, CLAUDE.md compliance, PR gating (APPROVED/BLOCKED). |
| **product-manager** | Roadmap, PRDs, prioritization. Synthesizes all business unit input. |
| **project-manager** | GitHub issues, project boards, ticket status transitions. |
| **backend-engineer** | Go BFF service, daemon binary, repositories, migrations, middleware. |
| **front-engineer** | React SPA, Vite config, Playwright E2E tests. |
| **dba** | PostgreSQL schema, migrations, index strategy, query optimization, RDS config. |
| **infrastructure** | CloudFormation, EC2, CI/CD workflows, nginx, GitHub Actions. |
| **ui-tester** | Playwright E2E and Vitest component tests. |
| **ux-designer** | Brand design, color palettes, typography, design tokens. |
| **business-analyst** | KPIs, dashboards, cohort analysis, A/B test design. |
| **finance-controller** | P&L, burn rate, cloud costs, MRR/churn tracking. |
| **customer-success** | User feedback, support docs, bug triage, feedback loop. |
| **growth-marketing** | SEO, content, social media, email campaigns. |

## Notes

- The business unit is intentionally flat at this stage — all four agents feed directly into product-manager. Revisit when product-manager becomes a bottleneck, or at v1.0.0.
- lead-engineer is a required gate before any PR merges.
- Versioning follows Semantic Versioning (semver.org) from v0.1.0 onward.
- Active project board: Vault MTG v0.2.0 (GitHub project #28).
