# v0.3.1 "Packaging" — Architect Wave 0 Review

**Date**: 2026-05-09
**Author**: Ray Hamilton, Architect
**Status**: APPROVED WITH CONDITIONS — see §7

---

## 1. Wave Sequencing Assessment

The wave order is **correct and internally consistent** with one structural concern.

**Confirmed correct order:**

- Wave 1 (CI Hardening: #1658, #1659) must precede all other waves. The `sign-macos` job bugs waste runner minutes and can silently hang any downstream release pipeline. This is the right call to fix first.
- Wave 2 (GoReleaser Foundation: #1639, #1642) must precede Wave 3 (installers). The `.pkg` and NSIS wrappers package the GoReleaser-produced binary; they cannot be validated end-to-end without it.
- Wave 3 (installers: #1640, #1641) must precede Wave 4 (PKCE auth). A working installer is the delivery vehicle for the binary that runs the PKCE flow. You cannot test first-run auth without an installed binary.
- Wave 4 (PKCE auth + BFF endpoint: #1643, #1650, #1651, #1652) has an internal sub-dependency: #1643 (config detection) and #1651 (keychain storage) must be implemented before #1650 (PKCE flow) can be fully integrated end-to-end. The kickoff correctly labels #1650 as `M` effort for this reason.
- Wave 5 (SPA setup page + download UX: #1644, #1645, #1646) correctly follows Wave 4. The SPA `/setup` page polls the daemon health endpoint and reflects PKCE flow state — this cannot be implemented without the BFF endpoint (#1652) being defined.
- Wave 6 (Storybook/Chromatic: #1621, #1622, #1625) is correctly sequenced after Wave 5. The component library baseline is a pre-beta quality gate, not a blocker for any other wave's feature work. Waves 5 and 6 can **partially overlap** — the Storybook spike (#1621) has no dependency on Wave 5 tickets and can start when Wave 4 closes.
- Waves 7 (staging validation) and 8 (release gate) are correctly terminal. Wave 7 has no tickets yet — PM must file them before Wave 6 closes (flagged below).

**Sequencing concern (non-blocking):** The PRD and kickoff use different wave numbering. The PRD labels Staging Validation as Wave 7 and Release Gate as Wave 8; the kickoff uses Wave 6 and Wave 7 respectively for the same waves. This is cosmetic but risks agent confusion when referencing wave numbers in ticket comments. PM should standardize on **PRD wave numbering (Waves 1–8)** and update kickoff.md accordingly.

---

## 2. Cross-Cutting Concerns

The following shared interfaces and data structures are depended on by multiple waves. They must be **fully defined before Wave 4 tickets move to In Progress**.

### 2a. `daemon.json` Schema

Multiple tickets read or write `daemon.json` (#1643, #1650, #1651, and the NSIS/pkg installers). The schema is partially specified across ADR-020 (step 10) and ticket bodies but is not consolidated in one place.

**Required canonical schema (define before Wave 4):**

```json
{
  "cloud_api_url": "https://api.vaultmtg.app",
  "keychain": true
}
```

The `api_key` field must NOT appear in `daemon.json` when `keychain: true`. Any pre-existing `daemon.json` with a plaintext `api_key` field must be treated as a migration case (read the key, write it to the keychain, rewrite `daemon.json` with `keychain: true`, zero out the plaintext field).

**Owner**: backend-engineer must agree on this schema before #1643 starts.

### 2b. Keychain Naming Convention

ADR-020 specifies `service: "mtga-companion", key: "api-key"`. Ticket #1651 specifies `service: "com.mtga-companion.daemon", account: "api-key"`. These disagree. The discrepancy will cause silent keychain misses (daemon writes to one service name, next run reads from a different one and finds nothing).

**Decision required before #1651 starts**: Adopt the `com.mtga-companion.daemon` / `api-key` convention (reverse-DNS naming is standard practice on both macOS Keychain and Windows Credential Manager). Update ADR-020 to match.

### 2c. `POST /v1/daemon/register` Request/Response Contract

ADR-020 describes the flow at the prose level but does not specify the wire format. Both the daemon (#1650) and BFF (#1652) implement against this contract independently — any mismatch causes a runtime integration failure that will only be caught in Wave 7 staging.

**Define before Wave 4:**

```
POST /v1/daemon/register
Authorization: Bearer <clerk-session-jwt>
Content-Type: application/json

Request body: {} (empty — identity comes from the JWT)

Response 201:
{
  "api_key": "sk_...",
  "account_id": "<clerk-user-id>"
}

Response 401: JWT invalid or expired
Response 429: Rate limit exceeded (5 req/min per user per ADR-020)
Response 409: (optional) key already exists for this machine — return existing key
```

**Owner**: Backend engineer must agree and document this before #1652 starts. Recommend adding a `Contracts` section to ADR-020 rather than leaving it in ticket prose.

### 2d. PKCE Redirect URI Registration in Clerk

Clerk's OAuth server must have `http://localhost/callback` (or a wildcard for localhost ports) registered as an allowed redirect URI. Since the daemon uses an **ephemeral (random) port**, the full redirect URI is dynamic. Clerk's authorization must support `http://localhost:*/callback` or the daemon must use a fixed port.

This is OQ-5 and it is a hard blocker for #1650. If Clerk does not permit wildcard localhost redirect URIs, the daemon **must** use a fixed port. This decision gates the entire Wave 4 auth implementation.

**Decision required before Wave 4 starts.** See §6 for recommendation.

### 2e. BFF API Key Storage — Missing Migration

The BFF currently has no `daemon_api_keys` table (migrations go through `000021`; no key-storage table exists). `POST /v1/daemon/register` requires persisting a per-machine API key scoped to `account_id`. A **new migration is required** before #1652 can be implemented. This migration is not currently a tracked ticket.

**Gap**: PM must file a DBA ticket for the `daemon_api_keys` schema before Wave 4 starts. Recommended schema:

```sql
CREATE TABLE daemon_api_keys (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id  TEXT NOT NULL,          -- Clerk user ID
  key_hash    TEXT NOT NULL,          -- bcrypt/argon2 hash of the key
  key_prefix  TEXT NOT NULL,          -- first 8 chars for display ("sk_live_xxxx...")
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_used   TIMESTAMPTZ,
  revoked_at  TIMESTAMPTZ,
  UNIQUE (account_id)                  -- one key per user for beta (per-machine expansion is post-GA)
);
```

---

## 3. ADR Compliance

| Concern | ADR | Status | Finding |
|---|---|---|---|
| Daemon distribution strategy | ADR-011 | Accepted | Tickets #1639–#1642 align. GoReleaser open-source + lipo for darwin universal is correct. No violation. |
| PKCE auth flow | ADR-020 | Accepted | Tickets #1650, #1651, #1652 align. Keychain naming discrepancy between ADR-020 and #1651 body must be resolved (see §2b). |
| Clerk auth provider | ADR-009 | Accepted | `clerk-sdk-go v2` JWT verification on BFF is correctly specified in #1652. No violation. |
| CloudFront SPA serving | ADR-008 | Accepted | SPA `/setup` page changes (#1644, #1645, #1646) serve from CloudFront/S3. No violation. |
| ADR-011 TBD-G supersession | ADR-020 | Accepted | ADR-020 explicitly supersedes TBD-G. Ticket #1645 correctly references this. Compliant. |
| **MISSING ADR** | — | Gap | No ADR exists for the `daemon_api_keys` storage model — how keys are stored on the BFF, their lifecycle, and revocation policy. ADR-020 describes the acquisition flow but stops at "BFF mints and returns the key." The storage, revocation, and key rotation behavior needs a short ADR addendum or a new ADR-021. This is needed before #1652 starts. |
| Rate limiting on `/v1/daemon/register` | ADR-020 | Noted | ADR-020 specifies 5 req/min per user. No BFF rate-limiting infrastructure is currently in place (no middleware exists for this). Ticket #1652 must either implement rate-limiting middleware or scope this to a TODO with an explicit tracking ticket. |
| GONOSUMDB/GOPRIVATE in CI | architect.md rule 12 | Required | Any new CI workflow added in Wave 2 (#1642) must include `GONOSUMDB: github.com/RdHamilton/vault-mtg` and `GOPRIVATE: github.com/RdHamilton/vault-mtg` on every Go step. Backend engineer and infra must not miss this. |

---

## 4. Risk Assessment

### R-A: Clerk Wildcard Redirect URI (CRITICAL — Wave 4 blocker)

Clerk's OAuth implementation may not permit wildcard localhost redirect URIs (`http://localhost:*/callback`). If only exact URIs are whitelisted, every user's ephemeral port would need to be registered in advance — which is impossible for random ports.

**Architect position**: Use a **fixed port** (recommend `51423`) for the PKCE callback. This is OQ-5. The UX tradeoff (user sees a fixed port URL, firewall instructions are simpler) is worth the implementation simplicity. If port 51423 is in use, the daemon should retry once with a fallback (e.g., `51424`), not random-walk. Fixed ports are also easier to document in firewall bypass instructions for enterprise users.

If Clerk does not support exact `http://localhost:51423/callback` as a redirect URI (some Clerk plan tiers restrict OAuth app redirect URIs), the fallback is a custom URL scheme (`vaultmtg://callback`) registered as a platform URI handler — this is architecturally more complex and must be scoped to a separate ticket if needed.

### R-B: `go-keyring` macOS Keychain Entitlement Requirement

On macOS, `go-keyring` uses the Security framework's `SecKeychainItem` API. Unsigned binaries can write to the Keychain at beta (no notarization required for Keychain writes), but **Gatekeeper-quarantine flag on the `.dmg`** may prevent the binary from executing at all before the user manually clears quarantine. The Keychain access itself is not the risk — the risk is that users never reach the Keychain write because the binary is quarantined.

This is distinct from R-1 in the PRD's risk register (which focuses on the installer). **The binary inside the installer also gets quarantined on download.** The `.pkg` postinstall script should call `xattr -dr com.apple.quarantine` on the installed binary as part of postinstall. This is not currently specified in #1640's acceptance criteria.

### R-C: Windows Credential Manager — go-keyring CGO-Free Status

`go-keyring` on Windows uses the Windows Credential Manager API via `golang.org/x/sys/windows`. It is CGO-free on Windows (uses syscalls). This is correct for cross-compilation from a Linux CI runner. However, on macOS-cross-compiled Windows builds (GoReleaser producing a Windows binary from the macOS runner), ensure the daemon's `go.mod` does not pull in any CGO dependency that would break the Windows cross-compile step.

**Action**: backend-engineer must run `GOOS=windows GOARCH=amd64 go build ./...` from the daemon module and verify zero CGO in the dependency tree before merging #1651.

### R-D: Missing `daemon_api_keys` Migration — Wave 4 Integration Failure Risk

If the DBA migration is not filed and merged before #1652 starts, the backend engineer will implement an endpoint against a table that does not exist. This will cause Wave 4 integration tests to fail. The migration must be an explicit prerequisite of #1652 in GitHub's dependency graph.

### R-E: Gatekeeper Quarantine on Binary (not just installer)

See R-B above. The `.pkg` postinstall script must clear the quarantine attribute on the installed binary. Without this, the LaunchAgent will fail to start the daemon after install on macOS 14+ even after the user has dismissed the Gatekeeper installer warning.

### R-F: Wave 7/8 Tickets Not Filed

Waves 7 (Staging Validation) and 8 (Release Gate) have no tickets on Project #33. The PRD correctly notes this. The risk is that if PM waits until Wave 6 closes to file them, engineering will be blocked waiting for ticket creation. PM must file Wave 7 tickets before Wave 5 closes (not Wave 6 — lead time needed).

### R-G: Storybook Wave Numbering (Low)

Per OQ-3, Storybook tickets #1621, #1622, #1625 were confirmed as v0.3.1. The BROADCAST and PRD now agree. No risk.

---

## 5. Interface Contracts to Define Upfront

These must be resolved and documented **before the first Wave 4 ticket moves to In Progress**:

| Contract | Where to define | Blocks |
|---|---|---|
| `daemon.json` canonical schema | Append to ADR-020, §Decision | #1643, #1650, #1651, NSIS/pkg tickets |
| Keychain service name + account key | Update ADR-020 §Decision step 9 | #1651 |
| `POST /v1/daemon/register` request/response JSON | Add `## Contracts` section to ADR-020 | #1650, #1652 |
| PKCE callback port (fixed vs random) | Resolve OQ-5; note in ADR-020 §Decision step 3 | #1650, Clerk redirect URI config |
| `daemon_api_keys` table schema | New DBA migration ticket + migration file | #1652 |
| API key scoping (per-machine vs per-user) | Resolve OQ-6 | #1652 schema design |
| Key revocation on reinstall | Resolve OQ-7 | #1652 (409 behavior), #1651 (keychain delete on reinstall) |

---

## 6. Recommended Pre-Wave-1 Decisions

The following decisions, if deferred past Wave 4, will cause rework across multiple tickets:

**Decision 1 — PKCE callback port** (OQ-5):
Use fixed port `51423` with one retry on `51424`. Update ADR-020 step 3 to specify this. Backend engineer registers `http://localhost:51423/callback` and `http://localhost:51424/callback` in the Clerk OAuth application config. This decision gates #1650.

**Decision 2 — API key scoping** (OQ-6):
Use **per-user, one-key-per-account** for beta. Rationale: simplest database schema (UNIQUE on account_id), and beta users are unlikely to run the daemon on multiple machines simultaneously. Per-machine expansion is post-GA. This decision gates the `daemon_api_keys` schema in §2e.

**Decision 3 — Key revocation on reinstall** (OQ-7):
On reinstall (daemon detects existing keychain entry), silently re-use the existing API key (do not revoke, do not re-pair). If the user explicitly wants to re-pair, they must delete the keychain entry manually or use a `--reset` flag (out of scope for v0.3.1). The BFF's `POST /v1/daemon/register` returns the existing key on a 200 (not 201) if one already exists for the account. This decision gates #1651 and #1652.

**Decision 4 — Quarantine clearing in postinstall** (new, not in OQ list):
The macOS `.pkg` postinstall script (#1640) must call `xattr -dr com.apple.quarantine "$INSTALL_DIR/mtga-companion-daemon"` after copying the binary. Add this to #1640's acceptance criteria.

**Decision 5 — go-keyring cross-compile validation** (new):
#1651's acceptance criteria must include: `GOOS=windows GOARCH=amd64 go build ./...` passes with zero CGO in the dependency graph. This is a CI correctness gate.

**Decision 6 — Rate-limit implementation for `/v1/daemon/register`**:
ADR-020 specifies 5 req/min per user but no rate-limit middleware exists in the BFF. For beta, implement a simple in-memory rate limiter keyed on `account_id` inside the handler (no Redis needed at beta scale). Add this to #1652's acceptance criteria.

---

## 7. APPROVED / BLOCKED Verdict

**APPROVED WITH CONDITIONS**

Engineering may begin **Wave 1** immediately (#1658, #1659 — CI hardening). These tickets have no upstream dependencies and no unresolved architecture questions.

Engineering may begin **Wave 2** (#1639, #1642 — GoReleaser) immediately after Wave 1 CI is green.

Engineering **MAY NOT begin Wave 4** (#1643, #1650, #1651, #1652) until ALL of the following are resolved:

| # | Condition | Owner | Deadline |
|---|---|---|---|
| C-1 | Keychain naming convention resolved and ADR-020 updated (service: `com.mtga-companion.daemon`, account: `api-key`) | Architect signs off, backend-engineer implements | Before #1651 In Progress |
| C-2 | PKCE callback port decision made (OQ-5) — recommend fixed `51423` | Ray decision | Before #1650 In Progress |
| C-3 | Clerk OAuth application configured with allowed redirect URI(s) for the chosen port | backend-engineer | Before #1650 In Progress |
| C-4 | API key scoping decision made (OQ-6) | Ray decision | Before #1652 In Progress |
| C-5 | Key revocation behavior decided (OQ-7) | Ray decision | Before #1652 In Progress |
| C-6 | `daemon_api_keys` migration ticket filed and merged to main | DBA / backend-engineer | Before #1652 In Progress |
| C-7 | `POST /v1/daemon/register` request/response JSON contract documented in ADR-020 | Architect + backend-engineer | Before Wave 4 starts |
| C-8 | `daemon.json` canonical schema documented (with migration path for legacy plaintext `api_key`) | Backend-engineer | Before #1643 In Progress |

Engineering **MAY begin Wave 3** (installer work: #1640, #1641) in parallel with Wave 2, with the following condition: #1640's acceptance criteria must be updated to include the `xattr -dr com.apple.quarantine` call in the postinstall script (C-9, non-blocking for Wave 3 start but must be merged before Wave 3 closes).

**Wave 5–8 are unblocked from an architecture standpoint** — no additional pre-conditions beyond completing prior waves.

---

## Appendix: Open Questions Status

| OQ | Status in this review |
|---|---|
| OQ-1 (Apple Developer Program) | BROADCAST confirms enrollment complete (#1648). Resolved. |
| OQ-2 (Azure Trusted Signing budget) | Wave 4 documents only — not a Wave 4 blocker. Remains open until GA prep. |
| OQ-3 (Storybook scope) | Resolved — v0.3.1 per Ray's direction. |
| OQ-4 (Gatekeeper hard-block on macOS 14+) | Remains open — confirmed as Wave 7 gate. |
| OQ-5 (PKCE callback port) | MUST RESOLVE before Wave 4. Architect recommends fixed port `51423`. |
| OQ-6 (API key scoping) | MUST RESOLVE before Wave 4. Architect recommends per-user/one-key-per-account for beta. |
| OQ-7 (Key revocation on reinstall) | MUST RESOLVE before Wave 4. Architect recommends silent re-use of existing key. |
