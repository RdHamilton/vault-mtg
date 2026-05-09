# VaultMTG Company Vision

**Owner: Najah (Product Manager)**

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

## Current Status

| Milestone | Status | Board |
|---|---|---|
| v0.2.0 "Foundation" | CLOSED — 2026-05-07 | #28 |
| v0.3.0 "Telemetry Parity" | CLOSED — 2026-05-09 | #29 |
| v0.3.1 "Packaging" | **ACTIVE** | #33 (`PVT_kwHOABsZ684BXMn-`) |
| v0.4.0 "Beta Launch" | BLOCKED — starts after v0.3.1 closes | #30 (`PVT_kwHOABsZ684BW67K`) |

Engineering does not begin v0.4.0 Wave 0 until v0.3.1 closes and PM issues a GO.

## Key Milestones

| Date | Milestone |
|------|-----------|
| 2026-05-05 | Cloud infrastructure live; AWS Activate credits activated |
| 2026-05-07 | v0.2.0 "Foundation" closed |
| 2026-05-09 | v0.3.0 "Telemetry Parity" closed; v0.3.1 "Packaging" started |
| **2026-08-01** | **Waitlist opens** (updated from June 2; changed 2026-05-09 per Ray's decision) |
| 2026-06-26 | Internal stretch target — all v0.4.0 exit gates green, first waitlist batch invited |
| **2026-08-18** | **Closed beta launch — official public commitment** |

August 18 is a firm public commitment. June 26 is an internal stretch goal only. These dates were set 2026-05-08 after input from all four business agents; waitlist date updated 2026-05-09.

## Technical Architecture

| Decision | Detail | ADR |
|---|---|---|
| Auth provider | Clerk (JWT on BFF via `clerk-sdk-go v2`; Clerk React SDK on frontend) | ADR-009 |
| Production hosting | S3 + CloudFront via Route 53 | ADR-008 |
| Preview hosting | Vercel (PR previews only — never production) | ADR-008 |
| Daemon distribution | DMG (macOS) + MSI/NSIS (Windows); PKCE browser auth flow | ADR-011, ADR-020 |

## Notes

- The business unit is intentionally flat at this stage — all four agents feed directly into product-manager. Revisit when product-manager becomes a bottleneck, or at v1.0.0.
- lead-engineer is a required gate before any PR merges.
- Versioning follows Semantic Versioning (semver.org) from v0.1.0 onward.
- Vercel is PR preview only — production SPA is served from S3 + CloudFront (ADR-008).
- Beta is free and invite-only. Stripe integration deferred to post-beta GA.
- Active project board: v0.3.1 Packaging (GitHub project #33).
