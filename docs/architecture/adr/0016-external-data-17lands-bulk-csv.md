# ADR-016: External Data Dependency — 17lands Bulk CSV

**Date**: 2026-05-08
**Status**: Accepted
**Deciders**: Ray Hamilton, Architect Agent
**Related**: ADR-015 (Go Lambda batch pattern), PRD-0005 (Smart Craft Next)

---

## Context

Smart Craft Next (PRD-0005) recommends wildcard spends based on a
user's personal play history. For users with sufficient sample size
in a given archetype — defined as **≥ 50 matches per archetype** —
the recommendation engine uses their own match data ("own_corpus").
Below that threshold the personal sample is too small to ground a
recommendation, so the engine falls back to a public proxy: archetype
win-rate and pick-rate signals derived from 17lands's bulk data
exports.

VaultMTG already integrates with 17lands. There is a JSON API client
at `services/sync/internal/seventeenlands/client.go` that consumes the
deck-builder API for archetype enumeration. That client is sufficient
for low-volume, per-request lookups but is the wrong tool for the
volume Smart Craft Next needs (millions of card-by-archetype rows
across all current Standard, Pioneer, and limited formats).

17lands publishes **free bulk CSV exports** at predictable URLs,
updated daily. Each export is a few hundred MB compressed. These are
the canonical source for the win-rate and pick-rate signals Smart
Craft Next needs.

Three approaches were considered: (A) hit the 17lands JSON API at
batch time and paginate through every archetype × card combination;
(B) cache the 17lands bulk CSV in S3 once a day and read from S3
during the batch; (C) ship a hard-coded snapshot in the deployment
package and update it manually when needed.

---

## Decision

**The Smart Craft Next batch Lambda fetches the 17lands bulk CSV
export once per day, caches it in S3, and reads from S3 during the
nightly recommendation run. The fetch is part of the same Lambda
invocation that produces recommendations. If 17lands is unavailable,
the batch degrades gracefully: users with sufficient personal sample
size still get recommendations from their own corpus; users below
the threshold get an empty recommendation list with a UI surface
explaining why.**

### Specifics

1. **Source URL**: 17lands bulk CSV exports, hosted at the public
   URL pattern documented at `https://www.17lands.com/public_datasets`.
   The exact URL per format is configured via SSM parameter under
   `/vaultmtg/smart-craft/seventeenlands-csv-url/<format>` so a
   change in 17lands hosting does not require a code deploy.
2. **Storage**: S3 bucket `vaultmtg-smart-craft-cache` (per-account
   bucket with versioning enabled). Object key pattern:
   `seventeenlands/<format>/<yyyy-mm-dd>.csv.gz` plus a stable alias
   key `seventeenlands/<format>/latest.csv.gz` updated to point at
   the freshest successful fetch.
3. **TTL**: 24 hours. The Lambda checks the `LastModified` of
   `latest.csv.gz` at the start of each invocation. If it is fresher
   than 20 hours, skip the fetch and reuse. Otherwise, fetch from
   17lands, write to a dated key, then update the alias.
4. **Schedule**: The fetch runs as the **first step** of the Smart
   Craft Next batch Lambda each night (per ADR-015, that Lambda runs
   at 03:00 UTC). No separate Lambda or schedule for the fetch — it
   is intrinsic to the batch.
5. **Fallback — never block the feature**: If the fetch fails (HTTP
   error, network timeout, 17lands outage, ToS-blocking response,
   anything), the batch logs the failure with a structured error
   code and continues. **Recommendations for users with ≥ 50
   matches per archetype are still produced from own_corpus.** Users
   below the threshold receive an empty recommendation list for that
   format on that day. The next successful fetch resolves it.
6. **Data source provenance**: The `craft_recommendations` table
   includes a `data_source` column with the values:
   - `'own_corpus'` — recommendation grounded entirely in the
     user's personal match history.
   - `'17lands_proxy'` — recommendation grounded in the 17lands
     bulk-data signal because the user lacks sufficient personal
     sample.
   The frontend reads `data_source` and renders an attribution
   badge in the UI when it equals `'17lands_proxy'`.
7. **Cache validation**: Each downloaded CSV is validated before it
   replaces the alias: row count is non-zero, expected columns are
   present (`set_code`, `archetype`, `card_name`, `win_rate`,
   `pick_rate`, sample-size column), and a checksum is recorded in
   the S3 object metadata. A failed validation discards the new
   download and leaves the previous `latest.csv.gz` in place.
8. **Cost envelope**: Bulk CSVs are a few hundred MB compressed per
   format. With ~5 actively tracked formats and one stored copy per
   day plus the alias, expected S3 storage is well under $1/month
   per format at standard tier. CloudFront is not used; the Lambda
   reads directly from S3 via the AWS SDK.

### Terms of service and attribution

17lands publishes its bulk data under a clear non-commercial / personal-use
allowance. VaultMTG's use as a recommendation proxy is consistent with
that allowance during the closed beta but **must be reviewed by
counsel before GA**, particularly once a paid tier exists. The
following attribution requirements are non-negotiable:

- **UI attribution**: When `data_source = '17lands_proxy'`, the
  recommendation card displays a "Data: 17lands" badge linking to
  `https://www.17lands.com`. This is part of the acceptance criteria
  for the front-end ticket and is not a UX flourish that may be
  trimmed during polish.
- **Documentation attribution**: VaultMTG's public docs and any
  marketing copy that references win-rate or pick-rate proxy data
  cite 17lands as the source.
- **Respect rate limits**: The bulk CSV is fetched at most once per
  format per 24 hours. The Lambda must not retry failed fetches in a
  tight loop; on failure, it waits for the next scheduled run.

A pre-GA legal review ticket is filed as part of this ADR's
implementation work (see Implementation Tickets, TBD-G). Until that
review clears, **no paid tier may surface 17lands-derived
recommendations**. The closed beta (free, invite-only per project
memory) is unaffected.

### What this changes

- Adds a new S3 bucket `vaultmtg-smart-craft-cache` to the
  CloudFormation footprint.
- Adds 17lands bulk CSV fetch + validation logic to the Smart Craft
  Next batch Lambda (per ADR-015).
- Adds the `data_source` column to `craft_recommendations` and the
  attribution surface to the frontend recommendation card.
- Adds a non-blocking external dependency to the batch — explicitly
  designed to degrade gracefully.

### What this does not change

- The existing JSON API client at
  `services/sync/internal/seventeenlands/client.go`. That client
  remains the right tool for low-volume request-time lookups (e.g.
  archetype enumeration during set ingestion). It is not used by
  Smart Craft Next.
- The own_corpus data path. Users with sufficient personal sample
  never hit 17lands at all — the fallback exists only for users
  below the threshold.

---

## Consequences

### Positive

- **Cold-start users get useful recommendations.** A user who has
  played 200 matches but never touched a particular archetype still
  gets a real signal for that archetype, sourced from the broader
  17lands community.
- **Graceful degradation.** A 17lands outage does not take down
  Smart Craft Next. Returning users keep their own_corpus
  recommendations; only the cold-start surface goes dark for the
  duration of the outage.
- **Cheap.** S3 storage cost for the cache is negligible (<$1/mo
  total). The 17lands fetch happens once per format per day,
  amortized across the entire user base.
- **Provenance is explicit.** Every recommendation row carries its
  `data_source`. The product team and the user both know which
  numbers are grounded in personal play and which are grounded in
  the public proxy.

### Negative

- **External dependency risk.** 17lands could change its bulk-data
  hosting URL, change CSV schema, change ToS, or shut down. Mitigated
  by: SSM-configured URLs (no code change to point elsewhere), CSV
  validation (catches schema drift before it corrupts the
  recommendation table), and graceful fallback (outage does not break
  the feature).
- **Legal review required before GA.** The closed beta is fine, but
  monetizing recommendations that are partly derived from 17lands
  data needs a counsel sign-off. This is a real GA blocker that must
  be on the v1.0.0 milestone, not deferred.
- **Sample-size threshold is a tuning knob, not a fact.** "≥ 50
  matches per archetype" is the v0.4.0 starting point. If the
  recommendations from own_corpus turn out to be noisy at that
  threshold, raise it; if cold-start is too long, lower it. Plan
  for a tuning pass after the first weeks of beta usage.

### Deferred

- **Multiple-source proxy blending.** A future version could blend
  17lands signal with MTGGoldfish, MTGTop8, or other public sources
  to reduce single-source dependency. Not a v0.4.0 concern.
- **Self-hosted proxy data.** If 17lands access ever becomes
  unavailable on commercially acceptable terms, VaultMTG could
  derive its own proxy signal from aggregated user data once we have
  enough scale. Pre-mature today.
- **Per-user sample-size tuning.** The 50-match threshold is global
  for v0.4.0. Future work could adapt the threshold per-archetype
  (some archetypes converge with fewer matches than others). Not on
  the v0.4.0 critical path.

---

## Implementation Tickets

These tickets land in milestone v0.4.0 on project board #30. Project
Manager owns final ticket creation, milestone assignment, and
sequencing; this ADR is the source of truth.

| Ticket | Scope | Owner |
|---|---|---|
| **TBD-A** | CloudFormation: add `vaultmtg-smart-craft-cache` S3 bucket (versioning, lifecycle on dated keys, IAM read/write for the batch Lambda role) | infrastructure |
| **TBD-B** | SSM parameters: provision `/vaultmtg/smart-craft/seventeenlands-csv-url/<format>` for each tracked format | infrastructure |
| **TBD-C** | Lambda: implement 17lands fetch step (HTTP GET, write to dated S3 key, update alias) with validation + checksum | backend-engineer |
| **TBD-D** | Lambda: implement graceful-fallback path — on fetch failure, log structured error, continue batch, mark cold-start users for empty recommendations on this run | backend-engineer |
| **TBD-E** | Schema migration: add `data_source` column to `craft_recommendations` with values `'own_corpus'` or `'17lands_proxy'` | dba |
| **TBD-F** | Frontend: render "Data: 17lands" attribution badge on recommendation cards where `data_source = '17lands_proxy'` | front-engineer |
| **TBD-G** | Legal review (pre-GA blocker on v1.0.0 milestone): counsel sign-off on 17lands data use under a paid tier | architect |
| **TBD-H** | Observability: CloudWatch metric `SeventeenLandsFetchFailures`, alarm on three consecutive failures | infrastructure |
| **TBD-I** | Docs: short attribution + data-source explainer page on `vaultmtg.app` for users curious about the badge | front-engineer |

Each ticket gets acceptance criteria when the Project Manager files
it.

---

## Alternatives Considered

### A. Hit the 17lands JSON API per archetype × card

**Rejected.** Volume is wrong by two orders of magnitude. A
per-archetype enumeration of every card in every tracked format is
millions of requests; we would either rate-limit ourselves out of
the API or hammer it in a way that violates the spirit of the free
public access. Bulk CSV is exactly the right tool 17lands provides
for this exact volume.

### B. Cache 17lands bulk CSV in S3, read from S3 during batch

**Accepted.** See Decision section above.

### C. Ship a hard-coded snapshot in the Lambda deployment package

**Rejected.** Snapshot goes stale within hours of release. Every set
release, every metagame shift, every pro tour weekend would require a
re-deploy. Operationally untenable and product-quality unacceptable.

### D. Skip the fallback entirely; only support own_corpus users

**Rejected.** This kills the feature for new and lapsed-returning
users — exactly the audience most likely to need recommendations.
Smart Craft Next's value proposition collapses without a cold-start
answer.

---

## References

- ADR-015 — Go Lambda batch pattern. The Smart Craft Next Lambda
  that fetches and consumes the 17lands cache is the second instance
  of that pattern.
- PRD-0005 — Smart Craft Next. The product requirements that drive
  the cold-start fallback need.
- `services/sync/internal/seventeenlands/client.go` — existing JSON
  API client; remains in use for low-volume request-time lookups
  unrelated to Smart Craft Next.
- 17lands public datasets — `https://www.17lands.com/public_datasets`.
  Source URLs are SSM-configured and not embedded in code.
- VaultMTG mission (project memory) — north star is 50,000 monthly
  active users; cold-start support is essential to acquisition.
- Beta monetization (project memory) — closed beta v0.4.0 is free
  and invite-only; the legal review blocker (TBD-G) sits on
  v1.0.0, not v0.4.0.
