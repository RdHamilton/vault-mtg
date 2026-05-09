# ADR-017: BFF Read Contract for Pre-computed Recommendations

**Date**: 2026-05-08
**Status**: Accepted
**Deciders**: Ray Hamilton, Architect Agent
**Related**: ADR-015 (Go Lambda batch pattern), ADR-016 (17lands bulk CSV), PRD-0005 (Smart Craft Next)

---

## Context

ADR-015 establishes that personalized recommendations are produced by a
nightly Go Lambda and written to per-feature tables (the first being
`craft_recommendations`). The BFF reads from those tables and serves the
results to the SPA. The PRD specifies `GET /v1/user/craft-next` with a
target latency of "< 100 ms" and a refresh cadence of "at most once per
24 hours per user."

The PRD does not specify the read-time behavior in the cases that
matter operationally:

1. The user opens the panel **before any batch run has produced rows**
   for their account (new user, first 24 hours).
2. The user opens the panel and the **most recent batch is older than
   24 hours** (Lambda failed last night, or the user's account was
   skipped due to a per-account error per ADR-015).
3. The user opens the panel and the **17lands fetch failed** so all
   rows for that user have `data_source = 'own_corpus'` only — no
   cold-start fallback rows are present (per ADR-016).
4. The user requests a format (e.g., Historic) for which **no
   recommendations exist** even though other formats do.
5. Two clients open the panel within seconds — the BFF should not
   issue duplicate DB reads if the response is cacheable.

Without an ADR, every future precomputed-feature endpoint
(`/v1/user/craft-next`, future `/v1/user/opponent-prediction`, future
`/v1/user/deck-grade`, etc.) will invent its own answer to these five
questions and the SPA will face an inconsistent read surface.

This ADR is the read-side companion to ADR-015. ADR-015 governs how
recommendations are produced; ADR-017 governs how the BFF serves them.

---

## Decision

**All BFF endpoints that serve pre-computed recommendation data follow a
single read contract: a typed response envelope carrying the
recommendation payload, a freshness descriptor, and a status code
distinguishing the five operational cases above. The contract is
implemented once in `services/bff/internal/api/precomputed/` and reused
by every precomputed-feature handler.**

### Specifics

1. **Response envelope** (shared by every precomputed endpoint):

   ```json
   {
     "data": [...],
     "freshness": {
       "computed_at": "2026-05-08T03:14:00Z",
       "age_seconds": 14400,
       "stale": false
     },
     "status": "ok",
     "data_source": "own_corpus"
   }
   ```

   - `data` — the recommendation payload (per-endpoint shape).
   - `freshness.computed_at` — ISO-8601 UTC timestamp pulled from the
     row's `computed_at` column. Single source of truth for "when was
     this generated."
   - `freshness.age_seconds` — server-computed `now - computed_at`
     in whole seconds. The SPA does not recompute this client-side
     (avoids clock-skew confusion).
   - `freshness.stale` — boolean. `true` when `age_seconds >
     stale_threshold_seconds` for the feature. Default threshold is
     **30 hours** (24h batch cadence + 6h grace for a missed run). Each
     feature may override in its handler config.
   - `status` — one of: `ok`, `not_yet_computed`, `stale`,
     `partial_fallback`, `format_unsupported`.
   - `data_source` — propagated from the row (per ADR-016). Optional;
     omitted when not applicable (e.g., feature does not blend external
     proxy data).

2. **Status codes — HTTP and envelope** (per the five operational cases):

   | Case | HTTP | `status` | `data` | When to use |
   |---|---|---|---|---|
   | Fresh batch row exists | 200 | `ok` | full payload | normal |
   | No row yet for this account | 200 | `not_yet_computed` | `[]` | new user, batch hasn't run |
   | Row older than threshold | 200 | `stale` | full payload | last batch failed; serve stale |
   | Row exists but external fallback unavailable | 200 | `partial_fallback` | partial payload | own_corpus rows present, 17lands rows missing |
   | User requested a format we never compute for | 200 | `format_unsupported` | `[]` | e.g., Pauper |

   **All five cases return HTTP 200.** A 4xx is reserved for actual
   client errors (auth missing, malformed query, account_id mismatch).
   A 5xx is reserved for the BFF being unable to read its own database.
   Empty payloads with explanatory `status` are not errors.

3. **No request-time computation, no fallback to live inference.** If
   the precomputed table is empty or stale, the BFF returns the stale
   row (or empty + `not_yet_computed`). It **does not** trigger a
   batch run, call Lambda, or call any external API at request time.
   Latency budget is < 100 ms p95, achievable only if the read is a
   single indexed `SELECT`.

4. **Caching layer**:
   - **v0.4.0**: no in-process or out-of-process cache. The read is a
     single indexed `SELECT` from RDS, expected to return < 100 rows
     per request. RDS handles this well at beta scale (≤ 50 concurrent
     panel opens per second).
   - **Deferred**: ElastiCache or in-process LRU is acceptable when
     beta MAU crosses 10k or p95 latency exceeds 100 ms. Add via a
     follow-up ADR; do not pre-build.
   - **Forbidden today**: ad-hoc per-handler memoization (e.g., a
     `sync.Map` keyed by account_id) — that is a cache without a TTL
     story and will leak.

5. **Account scoping**:
   - Every read is scoped by `account_id` resolved by
     `ClerkAuthMiddleware` (per ADR-009). The handler does **not**
     accept `account_id` as a query parameter — it reads it from
     context only.
   - Cross-account reads are impossible by construction. There is no
     admin or "view as user" mode in v0.4.0.

6. **Pagination & limits**:
   - Precomputed recommendation responses are bounded by the writer
     (per ADR-015, the Smart Craft Next batch writes ~20 rows per
     account per format). Endpoints serve **all rows for the
     requested account+format in a single response** — no pagination.
   - A hard server-side cap of **100 rows per response** is enforced
     in the shared read helper. Hitting the cap is an internal
     invariant violation and emits a structured warn-level log; it is
     not a user-facing error.
   - Sorting and filtering are out of scope for the precomputed read
     surface. The batch is the place to apply ranking and selection.
     If the SPA needs different sort orders, the batch writes them in
     the order the SPA wants and the BFF returns rows verbatim.

7. **Format parameter**:
   - Endpoints that scope by Magic format accept `?format=` as a
     required query parameter. Allowed values are an explicit allowlist
     in the shared helper: `standard`, `historic`, `explorer`,
     `pioneer`, `brawl`, `premier_draft`. Any other value returns 400.
   - Default formats are not assumed. The SPA must pass `format`
     explicitly. This avoids the "user looked at Standard yesterday and
     today the panel silently shows Historic" footgun.

8. **Observability**:
   - Each precomputed read emits a structured log line with
     `feature`, `account_id_hash`, `format`, `status`, `age_seconds`.
     `account_id` is hashed to keep raw IDs out of logs (per the
     PostHog/PII posture).
   - CloudWatch metric `PrecomputedReadStaleRatio` (per feature
     dimension) tracks the share of reads with `status=stale`. Alarm
     when it exceeds 5% over a 1-hour window — that is the signal that
     the batch Lambda is failing to keep up.

9. **Code layout**:
   - Shared read helper at `services/bff/internal/api/precomputed/`.
     Exposes `Read[T any](ctx, query, args...) (Envelope[T], error)`
     where `T` is the row type.
   - Per-feature handler at `services/bff/internal/api/handlers/`
     (e.g., `craft_next.go`) calls the shared helper and adds
     feature-specific query construction.
   - The envelope type lives in `services/contract/precomputed.go` so
     the SPA's typed client can import it (per the existing contract
     module pattern).

### What this changes

- Adds `services/bff/internal/api/precomputed/` package as the single
  implementation of the read envelope.
- Adds `services/contract/precomputed.go` (or extends the existing
  contract package) with the typed envelope so the SPA can share types.
- Adds `PrecomputedReadStaleRatio` CloudWatch metric per feature.
- Establishes that any future precomputed feature endpoint
  (opponent prediction, deck grading, etc.) reuses this envelope and
  does not invent its own.

### What this does not change

- The Smart Craft Next batch Lambda (ADR-015) and the 17lands fetch
  (ADR-016) are upstream — this ADR consumes their output and changes
  nothing about how they produce it.
- The BFF's general request/response patterns for non-precomputed
  endpoints (analytics aggregations, list endpoints, ingest endpoints).
  Those are governed separately (analytics by the pagination ADR
  pending in #1516).

---

## Consequences

### Positive

- **One contract, every precomputed feature.** SPA developers learn
  the envelope once. The "is this stale? is the empty list because
  the batch has not run yet or because there is genuinely nothing to
  recommend?" question has a single, machine-readable answer.
- **Operationally honest.** Returning a stale row with
  `freshness.stale = true` and `status = stale` is more useful to
  users than failing the request — they see the same recommendations
  they saw yesterday with a clear "last updated 31 hours ago" note.
- **No cache coherence problem.** The BFF holds no precomputed-feature
  state in memory. The batch is the writer; RDS is the source of
  truth; the read is a single indexed `SELECT`. No cache invalidation
  to debug.
- **Fast by design.** A single indexed `SELECT` against an RDS table
  with `(account_id, format)` as the lookup key returns in low single
  digit milliseconds at beta scale. The 100 ms budget has substantial
  headroom.
- **Scope creep guard.** Without this ADR, the first reflex when the
  read is "slow" is to add a cache. With it, the answer is "the read
  is a single indexed `SELECT`; if it's slow, fix the index, do not
  add a cache."

### Negative

- **Stale data shown to users.** When the batch fails, users see
  yesterday's recommendations. Mitigation: the `stale` flag and
  `age_seconds` give the SPA everything it needs to show a "last
  updated" banner and a refresh CTA. The alternative — failing the
  request — is worse for users.
- **No graceful per-account recovery.** A single account that the
  batch repeatedly fails to process will see `status = not_yet_computed`
  forever until the batch error is fixed. This is by design — the
  alternative is a synchronous fallback path that violates the "no
  request-time computation" rule. Operationally, the
  `BatchAccountsErrored` alarm from ADR-015 surfaces this.
- **Not a general-purpose envelope.** This contract is specifically
  for precomputed reads. Analytics endpoints (#1513, #1514) and list
  endpoints have different needs (pagination, filtering) and do not
  use this envelope. Two contracts is fine; pretending one envelope
  fits both would be worse.

### Deferred

- **Cache layer.** ElastiCache or in-process LRU when scale demands
  it. Today's read is fast enough.
- **Conditional GET / ETag support.** `freshness.computed_at` is
  enough to drive an ETag header in a future iteration if the SPA
  needs cheap "did this change?" polling. Not a v0.4.0 concern.
- **Push notifications.** Notifying the SPA when a fresh batch lands
  (via SSE) is a v0.5.0 candidate. v0.4.0 SPA polls or refreshes on
  panel open; that is sufficient at beta scale.

---

## Implementation Tickets

These tickets land in milestone v0.4.0 on project board #30. Project
Manager owns final ticket creation, milestone assignment, and
sequencing; this ADR is the source of truth.

| Ticket | Scope | Owner |
|---|---|---|
| **TBD-A** | Add `services/contract/precomputed.go` with `Envelope[T]`, `Freshness`, `Status` enum (`ok`, `not_yet_computed`, `stale`, `partial_fallback`, `format_unsupported`) | backend-engineer |
| **TBD-B** | Add `services/bff/internal/api/precomputed/` shared read helper: `Read[T any]` returns the envelope, computes `age_seconds`, applies the per-feature stale threshold | backend-engineer |
| **TBD-C** | Wire `GET /v1/user/craft-next` handler at `services/bff/internal/api/handlers/craft_next.go` using the shared helper. Format query param required. Account scoped from Clerk context. | backend-engineer |
| **TBD-D** | Add `PrecomputedReadStaleRatio` CloudWatch metric publisher in the read helper; alarm >5% over 1h per feature | infrastructure |
| **TBD-E** | SPA: typed client for `/v1/user/craft-next` consuming the shared envelope; render the `stale` banner and `last updated` line | front-engineer |
| **TBD-F** | SPA: handle `not_yet_computed` empty state — show the PRD-defined placeholder card explaining the user needs more matches | front-engineer |
| **TBD-G** | Test pack: BFF handler unit tests for all five status cases; SPA component tests for each empty/stale render path | backend-engineer + front-engineer |

Each ticket gets acceptance criteria when the Project Manager files
it.

---

## Alternatives Considered

### A. Per-handler envelope (no shared contract)

**Rejected.** Each precomputed feature would invent its own freshness
field, its own stale flag, its own empty-state semantics. The SPA
would have to special-case each endpoint's quirks. The first
duplicated bug in stale handling would force a refactor to a shared
contract anyway. Cheaper to do it once now.

### B. HTTP status codes for stale / not-yet-computed

**Rejected.** Returning 204 for "not yet computed" or 503 for "stale"
overloads HTTP semantics in ways that confuse browser and middleware
behavior. CloudFront caches 204 the wrong way. Browsers retry 503.
The five cases are all "we successfully read your data, here is what
we have, here is what you should know about it" — that is a 200 with
a body, every time.

### C. Trigger batch run on read miss

**Rejected.** Defeats the entire purpose of pre-computation.
Synchronous Lambda invocation from the request path adds 1–10 seconds
to the response time, eats Lambda concurrency budget, and races with
the scheduled batch run. The PRD explicitly forbids per-request
inference. Empty list + `not_yet_computed` is the right answer.

### D. ElastiCache or in-process cache from day one

**Rejected for v0.4.0.** RDS handles the read load comfortably at
beta scale. Adding a cache before there is a measured latency problem
introduces cache-coherence bugs (stale cache after a successful batch
write) for no benefit. Reconsider when p95 read latency exceeds
100 ms or RDS read load becomes a measurable issue.

### E. SSE push from BFF when fresh batch lands

**Deferred to v0.5.0.** Useful for keeping a long-open panel current,
but the panel-open use case (read once when the user navigates to it)
is dominant. Not worth wiring batch-completion notification through
the SSE broker for v0.4.0.

---

## References

- ADR-009 — Clerk auth. The read handler resolves `account_id` from
  the Clerk-injected request context only.
- ADR-015 — Go Lambda batch pattern. The producer side of the
  precomputed-data contract this ADR consumes.
- ADR-016 — 17lands bulk CSV. Source of the `data_source` field that
  this envelope propagates verbatim.
- PRD-0005 — Smart Craft Next. The first consumer of this read
  contract.
- v0.4.0 kickoff (`docs/prd/v0.4.0-kickoff.md`) — ticket #1513,
  #1514, #1516 reference the broader BFF read surface this ADR
  partially addresses (the precomputed slice).
