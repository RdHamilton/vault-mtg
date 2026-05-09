# ADR-010: Draft Overlay Architecture

**Date**: 2026-05-06
**Status**: Accepted
**Deciders**: Ray Hamilton, Architect Agent
**Related**: ADR-006 (BFF connectivity / CORS), ADR-008 (frontend serving model), ADR-009 (Clerk auth)

---

## Context

VaultMTG needs a live draft pick assistant that surfaces card grades to the
user while they are drafting in MTG Arena. The daemon already tails
`Player.log` via `fsnotify` and emits `draft.pack` and `draft.pick` events.
The BFF already exposes a per-user SSE broker at `/api/v1/events`. The open
question is the **client surface** through which a user consumes pick
recommendations during a live draft.

Three delivery surfaces were considered: a browser extension, a native
desktop overlay (Electron), and a standalone SPA route opened in a separate
browser window or monitor alongside Arena. Each was evaluated against
Arena's runtime model (native desktop, fullscreen DirectX on Windows /
Metal on macOS), the existing VaultMTG stack, and solo-developer release
overhead.

---

## Decision

**Option C — Daemon + SPA route in a browser window — is the canonical
draft overlay architecture for VaultMTG.**

### Specifics

1. **Event source**: The desktop daemon continues to read `Player.log` via
   `fsnotify` and emits `draft.pack` / `draft.pick` events to the BFF. No
   change to daemon scope.
2. **Transport**: The BFF's existing per-user SSE broker at
   `/api/v1/events` is the live channel. Draft events are filtered by
   `account_id` server-side (per ADR-009 multi-tenancy invariants); the
   client subscribes via `EventSource` and consumes typed event payloads.
3. **Client surface**: A new authenticated SPA route at
   `app.vaultmtg.app/draft/live` renders pick recommendations in real time.
   Users open this route in a second browser window (or a second monitor)
   alongside the Arena client. No injection, no overlay, no process
   attachment.
4. **State**: A `useDraftEventStream` hook owns the EventSource lifecycle
   (subscribe on mount, reconnect with backoff, unsubscribe on unmount). A
   draft session state machine reconciles `draft.pack` / `draft.pick`
   events into the current pack, pick number, and pool.
5. **Auth**: SSE requests carry a Clerk session JWT (per ADR-009). The
   `EventSource` auth strategy is resolved in the
   "SSE EventSource auth spike" implementation ticket — either via a
   short-lived token query parameter or a hand-rolled `fetch`+`ReadableStream`
   wrapper. No long-lived tokens in URLs.
6. **Infra**: nginx in front of the BFF must allow long-lived connections
   for `/api/v1/events` (proxy buffering off, read timeout ≥ 1h). An
   audit ticket verifies the existing config is SSE-safe.

### What this changes

- **Adds** a `/draft/live` route to the SPA, gated by Clerk auth.
- **Adds** a `useDraftEventStream` hook and a draft session state machine
  to the SPA.
- **Verifies** the IngestHandler → SSE broker wiring fans out
  `draft.pack` / `draft.pick` events on the per-user channel.
- **Audits** nginx for SSE-safe timeouts and buffering.
- **Does not** ship a native desktop binary, an installer, an auto-updater,
  or any code that touches Arena's process or graphics pipeline.

---

## Consequences

### Positive

- **Zero Arena performance risk.** The daemon only tails a log file.
  Nothing attaches to Arena's process, GPU surface, or input pipeline.
  No anti-cheat false positives, no fullscreen focus stealing, no GPU
  stutter.
- **Reuses 100% of the existing stack.** Daemon event emission, BFF SSE
  broker, Clerk auth, React SPA — all already shipped. The draft surface
  is a route, not a new platform.
- **Ships in roughly one sprint.** Implementation is bounded to a hook,
  a route, a state machine, and a wiring audit.
- **Aligns with web-first monetization.** Stripe checkout, shareable
  draft recap pages, and SEO-discoverable stats all live in the same
  SPA. A desktop binary would fragment this.
- **Matches the established companion-tool pattern.** 17Lands LiveTracker
  uses the same daemon + browser-window model; users already understand
  the alt-tab workflow.
- **No new release pipeline.** No Apple Developer cert, no Windows EV
  code-signing, no second auto-updater.

### Negative

- **Not a true overlay.** Users alt-tab to the second window or use a
  second monitor. Single-monitor windowed-Arena users have a
  context-switch cost on every pick.
- **SSE auth ergonomics.** `EventSource` does not natively support
  custom headers; the implementation must use one of the documented
  workarounds. Tracked in the SSE auth spike ticket.

### Deferred

- **Native overlay reconsideration.** Revisit Electron/native overlay
  **only** post-beta and **only** if both conditions hold:
  monthly active users > 10,000 **and** user research identifies
  alt-tab friction as the #1 driver of draft-feature churn. Until then,
  the cost-benefit does not clear the bar for a solo developer.

---

## Implementation Tickets

These tickets land in milestone v0.3.0 on project board #27. Project
Manager owns final ticket creation, milestone assignment, and sequencing;
this ADR is the source of truth.

| Ticket | Scope | Owner |
|---|---|---|
| **TBD-A** | Spike: `EventSource` + Clerk JWT auth strategy (short-lived token query param vs `fetch`+`ReadableStream` wrapper); pick one and document | front-engineer |
| **TBD-B** | Frontend: `useDraftEventStream` hook (subscribe, reconnect with backoff, unsubscribe, typed event payloads) | front-engineer |
| **TBD-C** | Frontend: `/draft/live` route, gated by Clerk auth, rendering current pack + pick recommendations | front-engineer |
| **TBD-D** | Frontend: draft session state machine reconciling `draft.pack` / `draft.pick` into pack/pick/pool state | front-engineer |
| **TBD-E** | Backend: verify IngestHandler fans `draft.pack` / `draft.pick` to the per-user SSE broker; add integration test if missing | backend-engineer |
| **TBD-F** | Infra: audit nginx config in front of BFF for SSE-safe timeouts (proxy buffering off, read timeout ≥ 1h) on `/api/v1/events` | infrastructure |
| **TBD-G** | Docs: user-facing "how to use the live draft assistant" guide (second window / second monitor workflow) | architect |

Each ticket gets acceptance criteria when the Project Manager files it.

---

## Alternatives Considered

### A. Browser extension

**Rejected.** MTG Arena is a native desktop application; it has no web
client. A browser extension has no surface to attach to and no way to
read Arena state. This option was a non-starter once the runtime model
was confirmed.

### B. Electron / native desktop overlay (always-on-top transparent window)

**Rejected.**

- **Performance and stability risk.** Always-on-top transparent windows
  rendered over fullscreen DirectX (Windows) and Metal (macOS) reliably
  cause GPU stutter, focus-stealing under fullscreen-exclusive mode, and
  anti-cheat false-positive risk. Mitigations exist but are
  per-platform and brittle.
- **Release-pipeline cost.** Requires an Apple Developer certificate,
  Windows EV code-signing, a second auto-updater, and platform-specific
  window-management code (per-DPI scaling, multi-monitor handling,
  fullscreen-exclusive vs borderless behavior). Disqualifying for a
  solo developer at this stage.
- **CI/CD timing.** Doubles the release surface at a moment when the
  CI/CD pipeline for the SPA + BFF + daemon is still being stabilized
  (cf. recent CI workflow remediation work).

### C. Daemon + SPA route in browser window

**Accepted.** See Decision section above.

---

## References

- ADR-006 — Vercel→BFF connectivity / CORS. CORS allow-list already
  covers `app.vaultmtg.app`; SSE inherits the same posture.
- ADR-008 — frontend serving model (S3+CloudFront for SPA). The
  `/draft/live` route ships in the same bundle.
- ADR-009 — Clerk user auth. The `/draft/live` route and `EventSource`
  connection are gated by Clerk session JWTs and resolve to an
  `account_id` server-side.
- 17Lands LiveTracker — reference implementation of the daemon +
  browser-window model in the MTG community.
