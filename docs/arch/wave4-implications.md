# Wave 4 Architectural Implications — v0.4.0

**Reviewed by**: Architect
**Date**: 2026-05-08

This is a one-pass architectural implications review for Wave 4 (v0.4.0 Closed Beta). It flags cross-cutting risks, ADR gaps, and sequencing constraints that engineering must respect before opening any ticket.

---

## Cross-Cutting Concerns

### 1. Pagination standard (#1516) is a hard prerequisite for #1513 and #1514
The pagination/filtering/sorting standard MUST be merged (ADR or tech spec + first reference implementation) before any analytics endpoint in #1513 / #1514 is coded. If these endpoints ship without the standard, we will rewrite their query layer twice and break frontend infinite-scroll contracts. Critical-path order is correct as stated.

### 2. `account_id` scoping is non-negotiable on every new BFF route
All eight new endpoints in #1513 + #1514 must scope every query by the `account_id` resolved from Clerk session context (per ADR-009 and the multi-tenancy rule). No raw `user_id`-from-claim parsing in handlers — use the established `auth.UserIDFromContext(ctx)` → `account_id` resolver.

### 3. Partial GRE rows must be filtered everywhere they aggregate (#1520 ↔ #1513)
The `partial = TRUE` column from #1520 must be filtered out of every win/loss aggregate in #1513 (DeckPerformance, WinRateTrend, FormatDistribution) and likely in #1514 (RankProgression, ResultBreakdown). Without it, partial sessions inflate loss counts and skew win-rate trends. Backend-engineer must treat this as a single coordinated change, not two independent tickets.

### 4. CSP allowlist (#1517) requires three-way coordination
The CSP policy must whitelist Clerk JS, PostHog, Crisp (#1573), and Sentry origins. If #1517 ships before #1573 lands, Crisp will be CSP-blocked at runtime. Infrastructure must collect the final origin list from front-engineer before publishing the CloudFront response headers policy.

### 5. PostHog event taxonomy is now a security-audit gate
Per #1488 acceptance criteria, every `captureEvent` call is scanned for PII. Front-engineer must NOT ship the activation funnel events or Crisp `setup_idle_90s` event with email addresses, names, raw IPs, or Clerk user IDs in event properties. Use anonymous distinct IDs only. This needs to be communicated up front, not caught in audit.

### 6. CI must be green before any ticket merges
#1524 is P0 because CI is currently red after the `pkg/logparse` extraction. Until #1524 lands, no other Wave 4 PR should be merged — green CI is an exit gate but it is also an entry gate for trusting any subsequent merge.

---

## Ticket-Specific Flags

- **#1513 / #1514** — Both depend on #1512 (GRE projector) and #1503 (account_id migration). If those upstreams are not Done, these tickets are blocked at the data layer regardless of pagination work. Confirm dependency state with PM before scheduling.
- **#1515 (Settings page)** — Must use `useUser()` directly. Do not introduce a Redux/Zustand slice for Clerk user data. CLAUDE.md explicitly forbids mirrored auth state.
- **#1517 (CSP)** — Test the policy in report-only mode first via CloudFront, capture violations for one staging session, then promote to enforce. Going straight to enforce will break Clerk's hosted iframes.
- **#1519 (GRE flush threshold)** — `partial: true` flag on the emitted event MUST match the BFF projector contract from #1512 and the column in #1520. If the daemon emits a different field name, projection silently drops the flag and #1520's filter becomes a no-op. Coordinate field name in the contract module before coding.
- **#1520 (partial column)** — Migration must be reversible AND must not require a table rewrite on production data. Use `ADD COLUMN ... DEFAULT FALSE NOT NULL` — Postgres 11+ handles this without a rewrite. Confirm RDS engine version supports the fast path.
- **#1573 (Crisp)** — Crisp script source domain must be in CSP `script-src` and `connect-src` allowlists for #1517. Front-engineer to publish the exact origin list as part of this ticket's PR description.
- **#1488 (Security audit)** — Must run AFTER all feature tickets are Done. Running it earlier wastes the audit because new code lands and re-introduces findings. PM should sequence this as the final pre-release ticket.
- **#1495 (EnvBadge E2E)** — Low risk, but the production-URL skip path must be deterministic. Do not branch on `process.env.CI` — branch on the target URL the spec is hitting.

---

## ADR Gaps

1. **Pagination standard ADR (gap)** — #1516's acceptance criteria explicitly require an ADR or tech spec before implementation. There is no current ADR covering pagination. The first artifact for #1516 is the ADR itself (proposed: ADR-015). Backend-engineer must not start endpoint code until this is merged.
2. **CSP / security headers policy ADR (optional)** — Not strictly required for #1517, but the CSP allowlist will be referenced repeatedly. Recommend a short ADR-016 capturing the policy, the allowlist, and the deployment mechanism (CloudFront response headers vs Lambda@Edge). Defer if time-constrained, but document the decision in the PR body either way.
3. **PostHog event taxonomy doc (gap)** — Not an ADR but referenced by #1488. There is no canonical list of allowed event properties. Recommend front-engineer publish `docs/analytics/event-taxonomy.md` as part of #1573 (since Crisp adds new events) and reference it in the security audit.

No other ADR gaps detected. ADR-008, ADR-009, ADR-012, ADR-014 cover the rest of the wave.

---

## Recommended Sequencing

Slight refinement of the stated critical path:

1. **#1524** (CI) — must merge first, full stop.
2. **#1516 ADR-015** (pagination standard, doc only) — merge before any code.
3. **#1516 reference implementation** (one endpoint refactored to the standard) — merge before #1513/#1514.
4. **#1520** (partial column migration) — must be in the migration sequence before #1513 ships, since #1513 filters on the column.
5. **#1519** (daemon flush) — can run in parallel with the BFF track; only blocks if daemon contract changes the partial-flag field name.
6. **#1513 / #1514** (analytics endpoints) — backend track, parallel.
7. **#1515 / #1573 / #1495 / #1393** (frontend + docs track) — parallel with backend.
8. **#1517** (CSP) — schedule AFTER #1573 lands so the Crisp origin is in the allowlist on first deploy.
9. **#1488** (security audit) — last, single pass over the merged result.

This is consistent with the PM's stated critical path; the only addition is an explicit "ADR before code" gate inside #1516 and the "#1517 after #1573" ordering.

---

## Green Light

**PROCEED** — with the following preconditions enforced by PM:

1. #1516 ADR is merged before #1513 or #1514 is opened by an agent.
2. #1517 is scheduled after #1573 (CSP allowlist must include Crisp origin on first deploy).
3. Backend-engineer treats #1519 / #1520 / #1513 partial-flag handling as a single contract — coordinate field name in the shared contract module before any of the three is coded.
4. #1488 is the LAST ticket scheduled in the wave.

No architectural blockers. Wave can start as soon as #1524 is green.
