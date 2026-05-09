# ADR-020: Daemon Initial Auth Acquisition: PKCE Browser Redirect

**Date**: 2026-05-09
**Status**: Accepted
**Deciders**: Ray Hamilton
**Supersedes**: ADR-011 § "First-run config" (the SPA `/setup` mints the first API key path)
**Related**: ADR-009 (Clerk auth), ADR-011 (daemon distribution strategy)

---

## Context

ADR-011 specified that the daemon's first-run pairing flow works as follows:
on missing `daemon.json`, the daemon directs the user to `vaultmtg.app/setup`,
and the SPA `/setup` page mints a Clerk API key and posts it to the daemon's
local health endpoint.

This requires the daemon to be running and reachable on localhost before the
user can authenticate. It also requires the SPA to hold Clerk session state
and make an outbound call to the daemon's local port — a cross-origin call
from a web page to `localhost` that is increasingly restricted by browsers
and blocked by enterprise firewalls.

Ray's decision is to replace this SPA-mint-key path with a **PKCE browser
redirect** flow where the daemon itself drives OAuth login directly. This ADR
documents that decision and supersedes the first-run pairing section of ADR-011.

---

## Decision

**The daemon acquires its initial Clerk API key through a PKCE OAuth
browser-redirect flow. The daemon opens the system browser, the user logs
in via Clerk, the daemon captures the auth code on a localhost callback,
exchanges it for a Clerk session token, calls `POST /v1/daemon/register`
on the BFF, and the BFF mints and returns the first API key. The daemon
stores the key in the OS keychain.**

### Flow (step by step)

1. **First-run detection** — daemon starts, finds no `daemon.json` (or
   finds a stub with no `api_key`).

2. **PKCE setup** — daemon generates a cryptographically random
   `code_verifier` (32 bytes, base64url) and derives `code_challenge`
   (SHA-256 of verifier, base64url).

3. **Localhost callback server** — daemon binds a one-shot HTTP server on
   `localhost:PORT` (random ephemeral port). The redirect URI is
   `http://localhost:PORT/callback`.

4. **Browser open** — daemon constructs the Clerk OAuth authorization URL
   with `response_type=code`, `code_challenge`, `code_challenge_method=S256`,
   and `redirect_uri`. Daemon calls the OS "open URL" command
   (`open` on macOS, `start` on Windows) to launch the system browser.

5. **User authenticates** — user logs in to their Clerk account in the
   browser. Clerk redirects to `localhost:PORT/callback?code=AUTH_CODE`.

6. **Code capture** — daemon's callback server receives the request,
   extracts `code`, shuts down the listener.

7. **Token exchange** — daemon POSTs `code` + `code_verifier` to Clerk's
   token endpoint. Clerk returns a session token (JWT).

8. **Daemon registration** — daemon calls `POST /v1/daemon/register` on
   the BFF with the Clerk JWT in `Authorization: Bearer`. BFF verifies the
   JWT via `clerk-sdk-go v2`, creates (or retrieves) a per-machine API key
   scoped to the authenticated user's `account_id`, and returns the key in
   the response body.

9. **Keychain storage** — daemon stores the API key in the OS keychain
   using `go-keyring` (service name: `mtga-companion`, key: `api-key`).
   The key is NOT written to `daemon.json` in plaintext.

10. **Config write** — daemon writes `daemon.json` with `cloud_api_url`
    and `keychain: true` (flag indicating the API key lives in the keychain,
    not the config file). All subsequent restarts read from the keychain.

### What this supersedes in ADR-011

ADR-011 § "First-run config: zero installer prompts" stated:

> On first launch the daemon detects a missing `daemon.json`, writes a stub
> config, and immediately directs the user to `https://vaultmtg.app/setup`.
> The setup flow on the SPA mints a Clerk API key (per ADR-009) and writes
> the config to disk via the daemon's local health endpoint.

That path is replaced by the PKCE flow above. The SPA `/setup` page:

- **Retains**: daemon status polling (health endpoint checks)
- **Retains**: "First-time install warnings" section (Gatekeeper / SmartScreen)
- **Removes**: the API key minting and localhost-write flow
- **Removes**: TBD-G from ADR-011 ("daemon pairing flow that mints a Clerk
  API key and posts it to the daemon's local health endpoint")

TBD-G is replaced by the three ADR-020 implementation tickets below.

---

## Consequences

### Positive

- **No localhost cross-origin calls from SPA.** The daemon drives its own
  auth; the browser is a dumb redirect target, not a client making API calls
  to localhost.
- **Clerk session token never leaves the daemon process.** The JWT is
  exchanged and discarded; only the API key is persisted, and only in the
  OS keychain.
- **Works headlessly.** If the user is on a server or a machine without a
  browser, the daemon can print the authorization URL and accept a
  `?code=` paste — same PKCE flow, no browser dependency.
- **Simpler SPA.** The SPA no longer needs to hold daemon-pairing state,
  make localhost calls, or handle cross-origin errors.

### Negative / Trade-offs

- **Requires BFF endpoint.** `POST /v1/daemon/register` is a new endpoint.
  It must be protected by Clerk JWT verification and rate-limited to prevent
  key-minting abuse.
- **Requires OS keychain dependency.** `go-keyring` adds a CGo dependency
  on Linux (not a target platform today) and has platform-specific behavior.
  macOS Keychain and Windows Credential Manager are well-supported.
- **PKCE callback port conflicts.** Ephemeral port selection must handle the
  (rare) case that the chosen port is in use. Implementation must retry with
  a different port.

### Neutral

- The daemon binary size increases negligibly (one HTTP listener + PKCE
  crypto, both in stdlib).
- The user experience is unchanged from the user's perspective: they
  double-click the installer, the browser opens, they log in, the browser
  closes, the daemon is paired. Identical to Spotify/Slack desktop login.

---

## Implementation Tickets

| Ticket | Scope | Owner |
|---|---|---|
| **ADR020-1** | `feat(daemon): implement PKCE OAuth browser-redirect login flow` — generate verifier/challenge, bind localhost callback, open system browser, capture auth code, exchange for Clerk session token | backend-engineer |
| **ADR020-2** | `feat(daemon): store Clerk API key in OS keychain (go-keyring)` — write/read/delete from macOS Keychain and Windows Credential Manager; fallback error handling if keychain unavailable | backend-engineer |
| **ADR020-3** | `feat(bff): add POST /v1/daemon/register endpoint` — accept Clerk JWT in Authorization header, verify via `clerk-sdk-go v2`, mint per-machine API key scoped to account_id, return key in response body; rate-limit to 5 req/min per user | backend-engineer |

---

## Alternatives Considered

### A. SPA mints API key and POSTs to daemon localhost (ADR-011 original)

**Rejected (superseded by this ADR).** Browser-to-localhost cross-origin
calls are restricted by modern browsers (mixed-content and CORS policies).
Enterprise firewalls block localhost ports. The SPA holding Clerk session
state and making outbound calls to daemon ports is an architectural anti-pattern.

### B. User copies API key from SPA and pastes into daemon CLI

**Rejected.** Requires the user to understand what an API key is, find it
in the SPA, copy it without leaking it, and paste it into a terminal or
config file. Unacceptable UX for the target audience (MTG Arena players,
not engineers).

### C. Device Authorization Grant (OAuth Device Flow)

**Considered.** The device flow is designed for devices without browsers.
The daemon does have access to a browser (it can `open` URLs), so PKCE
is the more standard choice. PKCE also avoids the polling loop that device
flow requires. Device flow can be used as a fallback for headless
environments in a future iteration.

---

## References

- ADR-009 — Clerk user auth provider decision
- ADR-011 — Daemon distribution strategy (superseded first-run section)
- [Clerk PKCE documentation](https://clerk.com/docs/backend-requests/resources/session-tokens)
- [go-keyring](https://github.com/zalando/go-keyring) — cross-platform OS keychain library
- [RFC 7636](https://www.rfc-editor.org/rfc/rfc7636) — Proof Key for Code Exchange
