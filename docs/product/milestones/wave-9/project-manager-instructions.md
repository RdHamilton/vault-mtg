# Project Manager Instructions — Wave 9 Initial Tickets

**Status**: Wave 9 kickoff doc is PENDING. These two tickets are pre-kickoff and tagged `wave-9` so they roll into the kickoff doc when it lands.
**Repo**: `RdHamilton/MTGA-Companion`
**Wave context**: Pre-beta hardening, before closed beta on 2026-08-18.

---

## Pre-flight (project-manager runs these in order)

1. **Create the `wave-9` label** if it does not exist:
   ```bash
   gh label create wave-9 -R RdHamilton/MTGA-Companion \
     --description "Wave 9 — pre-beta hardening (Aug 18 beta gate)" \
     --color "BFD4F2"
   ```
2. **Create the `pre-beta` label** if it does not exist:
   ```bash
   gh label create pre-beta -R RdHamilton/MTGA-Companion \
     --description "Required before 2026-08-18 closed beta" \
     --color "D93F0B"
   ```
3. **Find or create the Wave 9 milestone**. Likely name: `Wave 9 — Pre-Beta Hardening`. If a milestone does not yet exist for Wave 9, create one:
   ```bash
   gh api repos/RdHamilton/MTGA-Companion/milestones \
     -f title='Wave 9 — Pre-Beta Hardening' \
     -f description='Pre-beta hardening before closed beta on 2026-08-18' \
     -f state=open
   ```
   Capture the milestone number for ticket creation.
4. **Do not add to a project board yet** — Wave 9 board does not exist. Note in each issue body that the wave kickoff doc is pending; board assignment will follow.

---

## Ticket 1 — Daemon uninstall script (macOS + Windows verification)

**Title**: `feat(daemon): bundled uninstall script — macOS .pkg + Windows NSIS verification`

**Labels**: `wave-9`, `pre-beta`, `daemon`, `installer`, `agent:infrastructure`

**Milestone**: Wave 9 (number from pre-flight step 3)

**Assignee**: infrastructure (owns install scripts + pkgbuild)

**Body** (use exactly):

```markdown
## Summary

Today there is no clean way for a beta user to remove the daemon. They'd have to manually:

- `launchctl bootout gui/$(id -u) ~/Library/LaunchAgents/com.mtga-companion.daemon.plist`
- `rm ~/Library/LaunchAgents/com.mtga-companion.daemon.plist`
- `sudo rm /usr/local/bin/vaultmtg-daemon`
- `rm -rf ~/.mtga-companion`
- `rm ~/Library/Logs/vaultmtg-daemon.log`

Without an uninstall path, beta cancellations bleed into negative word-of-mouth and inflate support load. Ship a bundled `uninstall.sh` script — NOT a daemon subcommand, NOT a SPA button. Cheapest path that unblocks beta.

**Decision is locked**: bundled script only. Do not scope-creep into a daemon `uninstall` subcommand or an in-app removal flow.

> Wave 9 kickoff doc is PENDING. This ticket is tagged `wave-9` so it rolls into the kickoff doc when it lands; board assignment will follow.

## Acceptance Criteria

- [ ] `services/daemon/install/macos/uninstall.sh` exists and handles all five teardown steps above safely (idempotent — re-runs do not error or print confusing failures)
- [ ] Script exits non-zero only when a step truly fails (a missing file from a partial install is NOT a failure — log and continue)
- [ ] Script is included in the .pkg payload, installed to `/usr/local/share/vaultmtg/uninstall.sh` (or comparable system-wide path under `/usr/local/`)
- [ ] Postinstall script echoes the uninstall path so users see it after install
- [ ] `frontend/src/components/DaemonDownload.tsx` getting-started section documents the uninstall command for the user
- [ ] Windows NSIS installer is verified to auto-generate an uninstaller in Add/Remove Programs. If it does not, add an NSIS `Uninstall` section that mirrors the macOS teardown. Do NOT block macOS work on this — file a follow-up if Windows requires substantial work
- [ ] Manual test on macOS: install rc11, run uninstall.sh, verify all five locations are clean
- [ ] Manual test documented in PR description with before/after `ls` output for each of the five locations

## Test Plan

1. On a fresh macOS test machine, install daemon rc11 via the .pkg
2. Verify daemon is running: `launchctl list | grep mtga-companion`
3. Run `sudo /usr/local/share/vaultmtg/uninstall.sh` (or wherever script lives per AC)
4. Verify each of the five locations:
   - `launchctl list | grep mtga-companion` → no match
   - `~/Library/LaunchAgents/com.mtga-companion.daemon.plist` → absent
   - `/usr/local/bin/vaultmtg-daemon` → absent
   - `~/.mtga-companion/` → absent
   - `~/Library/Logs/vaultmtg-daemon.log` → absent
5. Re-run uninstall.sh — confirm it exits 0 and does not print scary errors
6. On Windows: install via NSIS package, open Add/Remove Programs, confirm `VaultMTG Daemon` appears with a working uninstall entry
7. Run the Windows uninstall and verify daemon is removed from the system

## Risks

- **NSIS auto-uninstaller may not exist**: depends on installer template. If missing, scope grows by ~1 day to write the Uninstall section. Mitigation: timebox NSIS verification at 2 hours; if uninstaller is missing, file a follow-up ticket for Windows-only and ship macOS first.
- **Permission edge case**: `launchctl bootout` may fail silently if daemon is already stopped. Script should treat "already stopped" as success, not as an error.
- **`/usr/local/bin` write permission**: removing the binary requires sudo. Script must either run under sudo or guide the user to re-run with sudo. Document this in the postinstall message.

## Notes for Implementer

- Pattern after existing `services/daemon/install/macos/postinstall.sh` for shell style and idempotency idioms
- Postinstall echo is the ONLY user-facing surface change in the .pkg flow — keep the message short ("To uninstall: sudo /usr/local/share/vaultmtg/uninstall.sh")
- Frontend doc update in `DaemonDownload.tsx` getting-started section: add a brief "Need to remove the daemon?" subsection with the command. No new UI components.
```

---

## Ticket 2 — Daemon bug reporting (Sentry crash capture + diagnostics bundle)

**Title**: `feat(daemon): Sentry crash capture + Copy Diagnostics button — pre-beta observability`

**Labels**: `wave-9`, `pre-beta`, `daemon`, `observability`, `agent:backend-engineer`, `agent:front-engineer`

**Milestone**: Wave 9 (number from pre-flight step 3)

**Assignees**: backend-engineer (daemon Sentry + endpoint), front-engineer (Diagnostics panel)

**Body** (use exactly):

```markdown
## Summary

Without daemon-side observability, every beta bug report becomes a 30-minute log-hunt. Frontend already has `VITE_SENTRY_DSN` wired up. The daemon is dark.

**Decision is locked**: wire Sentry into the Go daemon AND add a "Copy diagnostics" button in the SPA settings. NO in-app feedback form — Discord/email remains the support channel.

> Wave 9 kickoff doc is PENDING. This ticket is tagged `wave-9` so it rolls into the kickoff doc when it lands; board assignment will follow.

## Acceptance Criteria

### Daemon side (backend-engineer)

- [ ] Daemon imports `github.com/getsentry/sentry-go`
- [ ] Sentry initialized with `SENTRY_DSN` env var baked in at build time, following the same pattern as `CLERK_FRONTEND_API`
- [ ] Top-level panic recovery sends to Sentry with the `release` tag set to the daemon version
- [ ] Sentry user context is attached after PKCE auth completes — set the Clerk `user_id` as the Sentry user id
- [ ] New HTTP endpoint `GET /diagnostics` returns recent log tail + version info as JSON. Auth required — daemon API key, same auth model as other daemon endpoints
- [ ] `GET /diagnostics` response includes: daemon version, OS (runtime.GOOS + runtime.GOARCH), last 200 lines of `~/Library/Logs/vaultmtg-daemon.log` (or platform equivalent), uptime, current Clerk user id (if authed)
- [ ] Endpoint MUST NOT expose secrets — Clerk session tokens, API keys, or env values must be filtered from the log tail before returning
- [ ] Endpoint Go unit test verifies log tail is read correctly and secrets are filtered
- [ ] Integration test verifies panic → Sentry path (mock Sentry transport)

### Frontend side (front-engineer)

- [ ] New "Diagnostics" panel in SPA settings (`frontend/src/pages/Settings.tsx` or comparable existing settings route)
- [ ] "Copy diagnostics" button copies a JSON blob to clipboard containing:
  - daemon version
  - OS
  - last 200 lines of daemon log (fetched via the new `GET /diagnostics` endpoint)
  - browser user agent
  - frontend build SHA
- [ ] Button shows a clear "Copied!" confirmation after click (existing toast/snackbar pattern)
- [ ] Component test (Vitest) verifies blob structure and copy invocation
- [ ] Playwright E2E test covers the happy path: open Settings → Diagnostics → click Copy → confirmation visible

### Documentation (shared)

- [ ] `CONTRIBUTING.md` or new `SUPPORT.md` documents how to file a bug report — include the Copy Diagnostics flow, the Discord/email channel, and what to paste

## Test Plan

### Daemon
1. Run daemon locally with `SENTRY_DSN` set to a test project
2. Trigger a deliberate panic via test endpoint; verify event appears in Sentry within 60s with correct `release` tag
3. Sign in via PKCE; verify subsequent Sentry events include Clerk user_id
4. `curl -H 'X-Daemon-Key: <key>' localhost:PORT/diagnostics` returns JSON with version + log tail
5. `curl` without auth returns 401
6. Verify log tail does NOT include any `Bearer ` tokens, API keys, or Clerk secrets

### Frontend
1. Open SPA → Settings → Diagnostics
2. Click "Copy diagnostics"
3. Paste into a text editor; verify all six fields present
4. Verify "Copied!" confirmation appears
5. Run Playwright suite — diagnostics test must pass

### End-to-end
1. Kill daemon mid-request; verify panic captured in Sentry with stack trace and release tag
2. Open Settings → Diagnostics → Copy; paste into a Discord channel; verify a support agent can read it cleanly

## Risks

- **Log filtering**: the secret-redaction pass must catch every credential pattern in the daemon's logs. Use the existing `pkg/logfilter` package if one exists; otherwise file a follow-up to add one. PII leakage in support bundles is a privacy violation — non-negotiable.
- **Sentry quota**: daemon panics under bad logfile conditions could spam Sentry. Set client-side sampling to 1.0 for errors but use Sentry's rate limiter to cap at 100 events/hour per release.
- **Build-time env injection**: `SENTRY_DSN` baked at build time means CI must have access to the production DSN secret. Confirm with infrastructure that the secret is available in the daemon build workflow before merging.
- **Endpoint auth**: the `/diagnostics` endpoint exposes raw log content. The daemon API key MUST be required — no unauthenticated path.

## Notes for Implementers

- Sentry Go SDK pattern: see `github.com/getsentry/sentry-go` quickstart; mirror frontend init style (eager init at boot, defer flush on shutdown)
- Frontend can hit `/diagnostics` via the existing daemon REST adapter (per project rule: use the REST API adapter for new components)
- "Copy diagnostics" should produce a JSON blob the user can paste verbatim — do NOT base64-encode it, support agents need to read it
- Discord/email channel info goes in SUPPORT.md or CONTRIBUTING.md; growth-marketing owns the public-facing channel names
```

---

## After Both Issues Are Filed

1. Capture the two issue URLs and return them to PM (Najah)
2. Do NOT add either issue to a project board — Wave 9 board does not exist yet. PM will create the board with the Wave 9 kickoff doc
3. Do NOT move tickets to In Progress — they remain Todo until kickoff doc lands and architect implications note is delivered
4. Confirm neither issue body contains a "Generated with Claude Code" footer
