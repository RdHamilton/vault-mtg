# Design Note: Daemon Auto-Update Version Check (Issue #975)

**Date**: 2026-05-05
**Author**: Architect Agent
**Status**: Proposed (pre-implementation note, not a full ADR)
**Scope**: Issue #975 — Phase 1 only (version check + user notification). Phase 2 (auto-install) is deferred.

---

## Decision

**Use Option B: BFF-hosted version endpoint.**

- New BFF endpoint: `GET /api/v1/daemon/version` (no auth required — version metadata is public).
- Daemon embeds its own version at build time via `-ldflags` (`main.Version`).
- On startup and on a 24-hour ticker, the daemon `GET`s the BFF endpoint, compares semver, and logs a `WARN`-level "update available" line if `latest > current`.
- No silent auto-update. Phase 2 (download + replace) is a separate ticket.

## Rejected Alternatives

| Option | Why rejected |
|---|---|
| **A. GitHub Releases API** | Repo is private (`RdHamilton/vault-mtg`). The Releases API requires a `Bearer` token for private repos. Baking a PAT into a public binary is unsafe; rotating it requires re-shipping every daemon. Public Release assets are downloadable without auth, but the *listing* endpoint is gated. |
| **C. Signed manifest in S3** | Adds a new piece of infrastructure (S3 bucket + signing key + publish step in `daemon-release.yml`) for a problem the BFF already solves. We have no other use case for S3 manifests; not worth the cost. |
| **D. Embedded in JWT/auth response** | Couples daemon update notifications to the JWT refresh cadence (currently every ~hour but only fires when the JWT is near-expiry). Also pollutes the auth contract with an unrelated concern. Easy to add later as an optimisation if `/daemon/version` traffic becomes a concern. |

## Why B Wins

1. **No auth headache** — The BFF is already the daemon's source of truth. The version endpoint is public-readable; no PAT, no signing key, no rotation.
2. **Single deploy point** — `daemon-release.yml` is the only thing that publishes a new daemon. The BFF can read the latest version from a config value, an env var, or a tiny KV table — whichever the backend agent picks.
3. **Daemon already trusts the BFF** — Daemon auth and ingest already point at `cfg.CloudAPIURL`. Reusing it for a version check adds zero new failure modes.
4. **Cheap to implement** — Single GET handler in BFF + a 30-line poller in the daemon. Well under 2 hours per side.

## Daemon Behaviour

1. **Build-time injection.** `services/daemon/Makefile` and `daemon-release.yml` build with:
   ```
   -ldflags="-s -w -X main.Version=$VERSION"
   ```
   `$VERSION` derives from the `daemon/v*` git tag (strip the `daemon/` prefix). For local dev / `go build`, `Version` defaults to `"dev"`.
2. **Startup log line.** First action in `main()` after config load: `log.Printf("[mtga-daemon] version=%s", Version)`.
3. **Version check.** New package `services/daemon/internal/updatecheck/`:
   - `Check(ctx, baseURL, currentVersion)` performs `GET {baseURL}/api/v1/daemon/version` with a 5-second timeout.
   - Compares semver (use `golang.org/x/mod/semver` — already an indirect dependency).
   - If `latest > current`, logs `WARN` with the new version and the GitHub Releases URL.
   - All errors are logged at `INFO` and swallowed. **Never fatal.**
4. **Cadence.** Run once on startup (after registration) and on a 24-hour `time.Ticker`. Reuse the existing `select` loop in `service.go`.
5. **Version `"dev"` skips the check** — no point comparing local builds.

## BFF Behaviour

1. **New handler.** `services/bff/internal/api/handlers/daemon_version.go`:
   ```go
   func DaemonVersionHandler(w http.ResponseWriter, r *http.Request) {
       json.NewEncoder(w).Encode(VersionResponse{
           Latest:      cfg.DaemonLatestVersion, // e.g. "0.3.0"
           ReleasedAt:  cfg.DaemonReleasedAt,    // RFC3339
           DownloadURL: "https://github.com/RdHamilton/vault-mtg/releases/tag/daemon/v" + cfg.DaemonLatestVersion,
           Changelog:   "", // optional
       })
   }
   ```
2. **Route.** Mount under the public router (no auth middleware) at `GET /api/v1/daemon/version`.
3. **Source of truth for `DaemonLatestVersion`.** Two acceptable implementations — backend agent picks:
   - **Simple**: env var `BFF_DAEMON_LATEST_VERSION`, set at deploy time. Bumped manually when a new daemon release ships.
   - **Better**: a `daemon_releases` table the release workflow writes to. Defer this to a follow-up ticket; ship the env-var version first.
4. **CORS.** Endpoint must respond to the daemon's `User-Agent: mtga-daemon/<ver>`. No CORS concern (daemon is not a browser).
5. **Caching.** Add `Cache-Control: public, max-age=300` so a CDN / nginx can cache for 5 minutes if needed.

## Contract

```json
GET /api/v1/daemon/version

200 OK
{
  "latest": "0.3.0",
  "released_at": "2026-05-01T12:00:00Z",
  "download_url": "https://github.com/RdHamilton/vault-mtg/releases/tag/daemon/v0.3.0",
  "changelog": ""
}
```

A typed Go struct lives in `services/contract/daemon_version.go` and is published with the next `mtga-contract` tag.

## Acceptance Criteria (for Issue #975, Phase 1)

- [ ] Daemon binary embeds version via `-ldflags -X main.Version=...` (Makefile + `daemon-release.yml` updated).
- [ ] Daemon logs `version=<X>` on startup.
- [ ] BFF exposes `GET /api/v1/daemon/version` returning the latest published daemon version.
- [ ] Daemon performs version check on startup and every 24 hours.
- [ ] If a newer version is available, daemon logs a single `WARN` line per check naming the version and the release URL.
- [ ] Version-check failures (network, 5xx, malformed JSON) are logged at `INFO` and never crash or affect ingest.
- [ ] BFF endpoint has unit tests for happy path and missing-config path.
- [ ] Daemon `updatecheck` package has unit tests covering: equal version (no log), newer remote (logs WARN), network error (no log fatal), `"dev"` build (skipped).
- [ ] Documentation: `docs/DAEMON_INSTALLATION.md` mentions the version-check behaviour and how to disable it (env var `MTGA_DAEMON_DISABLE_UPDATE_CHECK=1`).

## Splitting Into Sonnet-Ready Tickets

This work splits cleanly into three tickets, each under 2 hours:

1. **[backend-engineer] BFF: `GET /api/v1/daemon/version` endpoint + contract type** (~1h, ~4 files: contract type, handler, route registration, test).
2. **[backend-engineer / daemon] Daemon: embed version + version-check package** (~1.5h, ~5 files: Makefile, `daemon-release.yml`, `cmd/daemon/main.go`, `internal/updatecheck/check.go`, `internal/updatecheck/check_test.go`).
3. **[backend-engineer] Daemon: wire updatecheck into Service.Run + docs** (~1h, ~3 files: `internal/daemon/service.go`, integration test, `docs/DAEMON_INSTALLATION.md`).

Phase 2 (download + atomic self-replace + rollback) gets its own design note and ticket later — it's a security-sensitive change and not in scope for #975 acceptance criteria as written.

## Out of Scope

- Auto-installing a new binary (Phase 2; deferred).
- Reading the version from a database table on the BFF side (env var is fine for v1; promote later if needed).
- Notifying the frontend / web app of an outdated daemon (separate UX ticket).
- Code-signing daemon binaries (separate infra ticket).
