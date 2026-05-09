# ADR-018: Pagination, Filtering, and Sorting Standard for BFF List Endpoints

**Date**: 2026-05-08
**Status**: Accepted
**Deciders**: Ray Hamilton, Architect Agent
**Related**: ADR-009 (Clerk auth), ADR-017 (precomputed read contract), ticket #1516

---

## Context

Wave 4 of v0.4.0 ships seven new analytics endpoints (#1513, #1514) on
top of the existing match, draft, deck, and collection list endpoints.
Today every list-style handler in the BFF invents its own
pagination, filtering, and sorting contract. The match list takes
`?limit=&offset=`; the draft list takes `?page=`; the deck list
takes neither and returns everything; the collection endpoint takes
`?cursor=` but never advances it. The SPA's data-fetching code
contains five bespoke shapes for "fetch a list" with five different
empty-state and end-of-list signals.

Ticket #1516 ("pagination/filtering/sorting standard across all list
endpoints") is the explicit prerequisite for #1513 and #1514. Without
a standard locked in before those endpoints are built, the analytics
surface inherits the same fragmentation and the SPA grows another
five bespoke list clients.

This ADR is the read-side companion to **list endpoints that return
unbounded user data** (matches, drafts, decks, collection rows,
analytics aggregations). Precomputed recommendation reads are bounded
by their batch and use the envelope from ADR-017 instead.

---

## Decision

**All BFF list endpoints adopt cursor-based pagination with a single
shared envelope, a single filter syntax, and a single sort parameter.
The standard is implemented once in
`services/bff/internal/api/listing/` and reused by every list handler.
Existing endpoints migrate to the standard as part of #1516; new
endpoints (analytics, anything Wave 4 forward) ship on it from day
one.**

### Specifics

1. **Pagination — keyset (cursor) only**:
   - Every list endpoint accepts `?cursor=<opaque>` and `?limit=<int>`.
   - `cursor` is an opaque base64-encoded JSON object containing the
     keyset (e.g., `{"id": "...", "occurred_at": "..."}`). Clients
     pass it back unmodified. **Clients do not parse or construct
     cursors** — that is a server-side detail.
   - `limit` is bounded: default 50, max 200. Requests above 200 are
     clamped silently to 200 (not an error — defends against accidental
     `limit=999999` calls).
   - **Offset pagination is forbidden.** Skipping N rows is O(N) at
     the database level and breaks under concurrent inserts. Keyset
     pagination is O(log N) and stable under writes.

2. **Response envelope** (shared by every list endpoint):

   ```json
   {
     "data": [...],
     "page": {
       "next_cursor": "eyJpZCI6Ii4uLiJ9",
       "has_more": true,
       "limit": 50
     }
   }
   ```

   - `data` — the array of rows for this page.
   - `page.next_cursor` — pass to the next request to fetch the
     following page. `null` when `has_more` is `false`.
   - `page.has_more` — boolean. `true` when more rows exist after
     this page. The handler computes this by fetching `limit + 1`
     rows internally and returning `limit`.
   - `page.limit` — the effective limit applied to this response
     (post-clamping). The SPA renders this verbatim.
   - **No `total_count`.** Counting all rows for a user's match
     history is expensive and rarely needed. Endpoints that genuinely
     need a count add a separate `GET /count` route — they do not
     include it in every list response.

3. **Filtering — typed query parameters only**:
   - Filters are individual typed query parameters: `?format=standard`,
     `?from=2026-04-01`, `?archetype=azorius_soldiers`. **No generic
     `filter` DSL** (no `?filter=format:eq:standard`).
   - Each handler declares its allowed filters in a per-endpoint
     allowlist. Unknown filter parameters return 400.
   - Date/time filters use ISO-8601 UTC. Range filters use
     `?from=&to=` (both optional, both inclusive).
   - Multi-value filters use repeated parameters: `?format=standard&format=historic`.
     The handler interprets repeated values as `OR` within a field
     and `AND` across fields.
   - Filter values are validated against an allowlist where the field
     has a fixed domain (e.g., `format`). Free-text filters
     (`?archetype=`) are passed through but always parameterized in
     SQL — no string concatenation, ever.

4. **Sorting — single sort parameter**:
   - `?sort=<field>` and `?order=asc|desc`. Default order is `desc`.
   - Each handler declares its allowed sort fields in a per-endpoint
     allowlist. Unknown sort fields return 400.
   - Default sort is `occurred_at desc` for time-series lists and
     handler-defined for non-temporal lists (e.g., decks default to
     `updated_at desc`).
   - **No multi-column sort in v0.4.0.** Sorting by two fields is a
     v0.5.0 concern; today's surface does not require it.

5. **Account scoping** (per ADR-009):
   - Every list query is scoped by `account_id` resolved from
     `ClerkAuthMiddleware`. The handler does **not** accept
     `account_id` as a query parameter.
   - The cursor itself is **not** sufficient for cross-account
     isolation — the cursor is a keyset, not an authentication token.
     The `account_id = ?` predicate is added unconditionally in the
     repository layer, and the cursor's keyset values are applied
     **on top of** that predicate.

6. **Errors**:
   - 400 — malformed cursor, unknown filter, unknown sort field,
     invalid filter value (e.g., `?format=pauper` when `pauper` is not
     allowed), `limit < 1`.
   - 401 — missing or invalid Clerk session.
   - 5xx — DB unavailable.
   - **No 404 for empty lists.** A user with zero matches gets
     `{"data": [], "page": {"next_cursor": null, "has_more": false,
     "limit": 50}}`. Empty is the answer, not an error.

7. **Performance posture**:
   - Every list endpoint must serve the first page in < 200 ms p95.
   - Every list endpoint must have a covering index for its default
     sort + account_id predicate. The DBA reviews any new list
     endpoint's index plan as part of the ticket.
   - **Analytics aggregations are list endpoints** for the purpose of
     this ADR — they return arrays of grouped rows. They use the same
     envelope and the same cursor semantics where pagination applies.
     Aggregations bounded by group cardinality (e.g., "by format" with
     6 formats) return the full set in one page with `has_more: false`.

8. **Code layout**:
   - Shared listing helper at `services/bff/internal/api/listing/`.
     Exposes `Paginate[T any]` that takes a query function returning
     `[]T`, a keyset extractor, and the shared envelope wrapper.
   - Per-feature handler at `services/bff/internal/api/handlers/`
     (e.g., `matches.go`) declares its filter allowlist, sort
     allowlist, default sort, and keyset shape.
   - Envelope type lives in `services/contract/listing.go` so the SPA
     imports it.

### What this changes

- Adds `services/bff/internal/api/listing/` with the shared paginator
  and envelope.
- Adds `services/contract/listing.go` with `ListEnvelope[T]` and
  `Page` types.
- Migrates existing list endpoints (matches, drafts, decks,
  collection) to the standard during #1516. The migration is
  backward-incompatible at the wire level — see the migration plan
  below.
- Wave 4 analytics endpoints (#1513, #1514) ship on this standard
  from day one. They do **not** invent their own pagination contract.

### What this does not change

- Precomputed recommendation reads (per ADR-017). Those are bounded
  by the batch and use the recommendation envelope, not this listing
  envelope. The two envelopes intentionally have different shapes
  because they encode different operational facts (freshness vs
  pagination position).
- Single-resource reads (`GET /v1/match/:id`). They return the
  resource directly, no envelope.
- Ingest endpoints. Writes are out of scope.

### Migration plan for existing list endpoints

The wire format changes for `/v1/matches`, `/v1/drafts`, `/v1/decks`,
`/v1/collection`. Strategy:

1. Stand up the new endpoints under `/v2/` paths first (`/v2/matches`,
   etc.) using the standard. SPA is updated in lockstep to call `/v2`
   and consume the envelope.
2. Once SPA is fully on `/v2`, the `/v1/` paths are kept for one
   release as a deprecation shim (return the new envelope at the old
   path), then removed.
3. The daemon does not call list endpoints — it only calls ingest.
   No daemon coordination required.
4. Total migration window: one release (v0.4.0 ships both `/v1` and
   `/v2`; v0.4.1 removes `/v1`).

This migration is bundled into ticket #1516.

---

## Consequences

### Positive

- **One client pattern, every list.** SPA developers write one
  `usePaginatedList<T>(endpoint, filters)` hook and reuse it across
  matches, drafts, decks, analytics, anywhere. Today there are five
  bespoke clients.
- **Stable under writes.** Keyset pagination does not skip or
  duplicate rows when new matches land mid-scroll. Offset pagination
  does both.
- **Defensive defaults.** Limit clamping and unknown-filter rejection
  catch lazy or malicious clients without surprising real users.
- **Account isolation by construction.** Cursors are scoped by the
  authenticated `account_id` predicate; a cursor leaked from one
  account is useless against another because the keyset is applied
  on top of the account_id filter.
- **Index discipline.** The "must have a covering index" rule forces
  the DBA review on every new list endpoint. We catch O(N) scans
  before they hit production.

### Negative

- **Cursor opacity is a footgun for ad-hoc API users.** Curl-from-the-
  command-line for debugging is harder when the cursor is a base64
  blob. Mitigation: a dev-only `?debug_cursor=1` flag (set in
  non-production environments) returns the cursor as decoded JSON in
  the response body for inspection.
- **Migration window is real.** Maintaining `/v1` and `/v2` for one
  release is engineering tax. Acceptable; the alternative (breaking
  the SPA on a single deploy) is worse.
- **No total_count by default.** UIs that show "1–50 of 1,247" must
  call a separate `/count` endpoint. Most VaultMTG list views are
  infinite scroll today, so this is not a v0.4.0 problem; revisit if
  product needs explicit pagination UI.
- **Analytics endpoints inherit a pagination contract they may not
  fully need.** A "by format" aggregation has 6 rows; the
  `has_more: false` envelope on every response is mild boilerplate.
  Worth it for consistency with the rest of the listing surface.

### Deferred

- **Generic filter DSL.** A `?q=format:standard AND archetype:soldiers`
  syntax is occasionally requested. Premature for v0.4.0; typed
  parameters cover every concrete use case. Reconsider if a future
  feature genuinely needs ad-hoc filter composition.
- **Multi-column sort.** Reconsider when a v0.5.0+ feature needs it.
- **Server-side projection / sparse field selection.** GraphQL-style
  `?fields=id,winrate` is not on the v0.4.0 roadmap. JSON payloads
  are small enough today.

---

## Implementation Tickets

These tickets land in milestone v0.4.0 on project board #30. Project
Manager owns final ticket creation, milestone assignment, and
sequencing; this ADR is the source of truth.

| Ticket | Scope | Owner |
|---|---|---|
| **TBD-A** | Add `services/contract/listing.go` with `ListEnvelope[T]` and `Page` types | backend-engineer |
| **TBD-B** | Add `services/bff/internal/api/listing/` shared paginator: `Paginate[T any]` with keyset extractor, limit clamping, unknown-filter rejection | backend-engineer |
| **TBD-C** | Migrate `/v1/matches` to `/v2/matches` using the standard; keep `/v1/matches` as a deprecation shim returning the new envelope | backend-engineer |
| **TBD-D** | Migrate `/v1/drafts`, `/v1/decks`, `/v1/collection` to `/v2/` using the standard | backend-engineer |
| **TBD-E** | Wave 4 analytics endpoints (#1513, #1514) ship on `/v2/analytics/*` using the standard from day one | backend-engineer |
| **TBD-F** | SPA: `usePaginatedList<T>` hook consuming the shared envelope; refactor existing list views to use it | front-engineer |
| **TBD-G** | DBA: covering-index audit for every existing list endpoint and every new analytics endpoint; add missing indexes | dba |
| **TBD-H** | Test pack: paginator unit tests for limit clamping, unknown filter, unknown sort, malformed cursor; cross-account cursor leak test | backend-engineer |
| **TBD-I** | Remove `/v1/` deprecation shims in v0.4.1 release | backend-engineer |

Each ticket gets acceptance criteria when the Project Manager files
it. **TBD-A and TBD-B block TBD-E** — analytics endpoints cannot
ship until the standard is in place.

---

## Alternatives Considered

### A. Offset pagination (`?page=&per_page=`)

**Rejected.** Skipping N rows in PostgreSQL costs O(N). At beta
scale that is fine; at 50K MAU the analytics endpoints (deep history)
break. Keyset pagination is O(log N) and stable under concurrent
writes. There is no scenario in which offset wins on the v0.4.0
through GA roadmap.

### B. Page-number pagination with caching layer

**Rejected.** A caching layer to make page-number pagination
performant introduces cache-coherence bugs (new match arrives, page
2 now contains a row that was on page 1). The cache is solving a
problem keyset pagination does not have.

### C. GraphQL connection-style pagination (`edges`, `pageInfo`,
`startCursor`, `endCursor`, `hasPreviousPage`)

**Rejected.** Connection-style pagination is the right answer in a
GraphQL context where bidirectional traversal is a real feature.
VaultMTG's SPA is forward-scroll-only for every list view. The extra
fields are dead weight without a corresponding feature.

### D. Generic filter DSL (`?filter=format:eq:standard,archetype:in:[a,b]`)

**Rejected for v0.4.0.** Typed query parameters cover every concrete
filter use case in v0.4.0. The DSL is a future generalization that
can be added without breaking the typed-parameter contract.
Premature; deferred.

### E. Per-handler invented pagination (status quo)

**Rejected.** Status quo is what #1516 is explicitly fixing. Five
bespoke clients today; eight or ten by GA if not standardized now.

---

## References

- ADR-009 — Clerk auth. Account scoping is resolved from request
  context; never from a query parameter.
- ADR-017 — Precomputed read contract. The complementary envelope for
  bounded recommendation reads. List and precomputed envelopes are
  intentionally distinct.
- v0.4.0 kickoff (`docs/prd/v0.4.0-kickoff.md`) — ticket #1516 is
  the prerequisite for the analytics endpoints; this ADR is the
  design that ticket implements.
- PostgreSQL keyset pagination — well-trodden pattern; see
  Markus Winand's "use the index, Luke" for the canonical reference.
