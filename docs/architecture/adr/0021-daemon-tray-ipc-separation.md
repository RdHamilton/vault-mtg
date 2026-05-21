# ADR-0021: Daemon / Tray IPC Separation

**Status**: Proposed
**Date**: 2026-05-16
**Decider**: Ray Hamilton, Architect Agent
**Supersedes / Amends**: Amends ADR-0011 §6 ("Daemon stays headless in v0.4.0") — see Context.

## Context

The canonical system-tray daemon pattern mandates a strict three-layer
separation:

```
┌────────────────────────────────────────────────────────┐
│               Desktop Environment Panel                │
│  (GNOME Shell / KDE Plasma / Windows Taskbar / macOS)   │
└───────────────────────────▲────────────────────────────┘
                            │ OS Native Tray API
┌───────────────────────────▼────────────────────────────┐
│                  Tray Client / UI                      │
│             (Thin presentation layer)                  │
└───────────────────────────▲────────────────────────────┘
                            │ IPC (Unix socket / D-Bus / pipe)
┌───────────────────────────▼────────────────────────────┐
│                    Daemon Process                      │
│          (Core logic / persistent state)               │
└────────────────────────────────────────────────────────┘
```

The principle is that the **daemon owns all state and runs headlessly**, the
**tray client is a thin, disposable presentation layer**, and an **IPC boundary**
lets either side restart independently. If the tray crashes, log monitoring,
event dispatch, and collection sync continue uninterrupted.

VaultMTG's daemon does **not** currently follow this pattern. Three findings
from the code audit:

### Finding 1 — The tray runs *in-process* with the daemon

`services/daemon/cmd/daemon/main.go` constructs a single process where:

- `tray.New(...)` builds the `systray` app.
- `app.Run(onReady)` calls `systray.Run()`, which **takes ownership of the
  main OS thread** (a hard Cocoa requirement on macOS).
- The daemon's core loop, `svc.Run(ctx)`, is started **inside the tray's
  `onReady` callback, on a goroutine of the tray-owning process**.

There is no process boundary. The tray and the daemon are the same `os.Process`.
`systray.Run()` is the outermost call; `svc.Run()` is a child goroutine of it.

### Finding 2 — `TrayHooks` is in-process channel coupling

`daemon.TrayHooks` (`internal/daemon/collection.go`) wires the two layers
together with Go channels and function pointers:

- Inbound (UI → daemon): `SyncNow`, `GrantAccess`, `TryAgain` — buffered
  `chan struct{}` selected on directly inside `Service.Run`'s event loop and
  `retryKeychain`.
- Outbound (daemon → UI): `SetHelperInstalled`, `SetLastSync`,
  `SetKeychainError` — `func(...)` pointers the daemon calls synchronously,
  which mutate `systray.MenuItem` objects directly.

This is a clean *abstraction* (the daemon depends on the `TrayHooks` interface,
not on `systray`), but it is **not an IPC boundary**. The channels only work
because both ends share an address space. If the tray goroutine panics, the
outbound `func` pointers still point at a half-destroyed `systray` state and
the inbound channels stop being drained — the daemon's `SyncNow` select case
would silently never fire.

### Finding 3 — `localapi` is a *real* IPC layer, but only for the SPA

`services/daemon/internal/localapi/server.go` already runs an HTTP server on
`127.0.0.1:9001`. It is a genuine, working loopback IPC channel:

- `GET /health` — liveness probe.
- `GET /api/v1/system/*` — status, version, account, daemon connect/disconnect.
- `GET /api/v1/drafts/*` — live draft state (grade-pick, win-probability).
- `POST /api/v1/replay` — triggers a historical log replay.
- `POST /api/v1/system/uninstall` — clean uninstall.

Crucially, **this IPC layer's consumer is the React SPA in a browser, not the
tray.** The tray and `localapi` are two unrelated communication paths that
happen to live in the same process. `localapi` is the seed of the IPC layer the
target architecture needs — it is already ~80% of a daemon control API.

### The ADR-0011 contradiction

ADR-0011 §6 states: *"The daemon stays headless in v0.4.0… does not ship a
system tray icon."* That decision has since been **overridden by
implementation** — a `getlantern/systray` menubar UI now exists and is the
default `main.go` path on CGO builds. The headless path survives only as the
`!cgo` build tag stub (`tray_nocgo.go`), used for cross-compiled GoReleaser
artifacts. So today the project ships **two divergent daemon topologies from
one codebase**:

| Build tag | Topology | Used for |
|---|---|---|
| `cgo` (default native) | Daemon + tray in one process, `systray.Run` owns main thread | macOS dev / native builds |
| `!cgo` | Headless daemon, no-op tray stub | GoReleaser cross-compiled artifacts |

This split is invisible and undocumented. The `!cgo` artifact has no UI at all;
the `cgo` artifact couples the UI to the daemon lifecycle. Neither matches the
target three-layer pattern.

## Decision

**Adopt the three-layer separation as the target architecture, and reach it in
two phases. Do the foundational, low-risk work in v0.3.x; defer the full
process split to v0.4.0.**

### Target state

```
mtga-daemon (headless process)              mtga-tray (thin client process)
─────────────────────────────              ───────────────────────────────
• launchd LaunchAgent at login              • launchd LaunchAgent at login
• owns Player.log poller                    • owns systray.Run() / main thread
• owns event dispatch to BFF                • renders daemon state in menu
• owns draft/GRE/collection state           • translates clicks → daemon cmds
• serves control API on loopback   ◄──IPC──► • polls daemon control API
• runs whether or not tray is up            • can crash/restart freely
```

- **Daemon process**: no `systray` import, no CGO requirement. One uniform
  binary for all platforms. Spawns at login via launchd (macOS) / Scheduled
  Task (Windows). Owns 100% of persistent state.
- **IPC layer**: the existing `localapi` HTTP server, extended with the
  *command* surface the tray needs (sync-now, grant-access, retry-keychain as
  `POST` endpoints; helper/keychain/last-sync status in `GET /api/v1/system/status`).
  Stays loopback-only (`127.0.0.1`). A Unix-domain-socket transport is a
  possible hardening follow-up but TCP-on-loopback is acceptable for v1 — it is
  what `localapi` already does and what the SPA already depends on.
- **Tray client**: a separate small binary (`mtga-tray`) that imports
  `systray`, owns the main OS thread, and is a pure consumer of the daemon
  control API. It holds **no state** — every menu label is rendered from a
  daemon poll; every click is an HTTP `POST` to the daemon. It can be killed
  and relaunched without touching the daemon.
- **Independence**: killing `mtga-tray` leaves `mtga-daemon` fully operational.
  Killing `mtga-daemon` leaves `mtga-tray` showing a "daemon offline" state
  (the control-API poll fails) until launchd respawns the daemon.

### Migration path

**Phase 1 — Foundational (v0.3.x, low-risk, no process split).**
Make the daemon *capable* of running headless and make the tray a *pure
client of `localapi`*, without yet splitting the process. This is mostly
internal plumbing and de-risks Phase 2.

1. **Move tray commands onto `localapi`.** Add `POST /api/v1/system/sync-now`,
   `POST /api/v1/system/grant-access`, `POST /api/v1/system/retry-keychain`.
   Add `helper_installed`, `keychain_error`, `last_sync` fields to the
   `GET /api/v1/system/status` response. The daemon's event loop selects on
   these the same way it selects on `TrayHooks` channels today — `TrayHooks`
   becomes an internal adapter fed by `localapi` rather than by the tray
   directly.
2. **Make `localapi` the single source of tray state.** The in-process tray
   keeps working, but it now reads/writes the daemon **only through the
   loopback API**, never through shared channels. After this step the tray
   could run in any process — it just happens not to yet.
3. **Document the topology.** Update ADR-0011 §6 to reflect that a tray now
   exists and to reference this ADR for the target separation.

**Phase 2 — Process split (v0.4.0, larger refactor).**

4. **Extract `mtga-tray` as a separate `main` package** under
   `services/daemon/cmd/tray/`. It imports `systray` + the `localapi`
   *client* (a new thin `localapi/client` package). It owns `systray.Run()`.
5. **Strip `systray` from the daemon binary.** `cmd/daemon/main.go` no longer
   imports `tray`; it runs `svc.Run(ctx)` directly on the main goroutine.
   The `cgo` / `!cgo` tray build-tag split is **deleted** — the daemon is one
   uniform binary.
6. **Ship two LaunchAgents / Scheduled Tasks.** The installer registers
   `com.vaultmtg.daemon` (headless, `KeepAlive=true`) and
   `com.vaultmtg.tray` (`KeepAlive=true`, depends on a logged-in GUI session).
   Both auto-start at login. Quit-from-tray stops only the tray; an explicit
   "Stop daemon" item stops the daemon.
7. **Harden IPC (optional).** Consider migrating the loopback transport from
   TCP to a Unix-domain socket for the daemon↔tray path (file-permission
   access control instead of relying on the loopback firewall). The SPA path
   stays TCP — browsers cannot speak Unix sockets.

### Recommendation: defer the split to v0.4.0; do Phase 1 now

**Do Phase 1 in v0.3.x. Defer Phase 2 (the actual process split) to v0.4.0.**

Rationale:

- **Phase 1 is cheap and high-value.** Routing tray commands through `localapi`
  removes the hidden in-process coupling, makes the daemon genuinely
  headless-capable, and de-risks Phase 2 — all without a process boundary,
  installer changes, or new LaunchAgents. It is 3–4 small, well-scoped tickets.
- **Phase 2 touches the installer and the release pipeline.** A second
  LaunchAgent/Scheduled Task, a second binary in the `.pkg`/installer, a second
  GitHub Release artifact, and second-process update logic are all v0.4.0-sized
  work that intersects ADR-0011's distribution strategy. Doing it mid-v0.3.x
  risks destabilizing the installer right before beta.
- **The current coupling is not an active production fire.** The `!cgo`
  artifact is already headless, and the `cgo` tray is dev-facing. The cost of
  waiting one minor version is low; the cost of a rushed installer change is
  high.
- **BROADCAST Wave 0 is explicitly "architecture brittleness fixes before new
  feature work."** Phase 1 fits Wave 0 perfectly — it removes brittleness
  (silent channel coupling) without adding feature surface. Phase 2 is a
  feature-shaped change and belongs in a planned wave.

## Consequences

### What becomes easier after Phase 1

- The daemon is provably headless-capable on every platform — the `localapi`
  surface is the only control path, no shared channels.
- `TrayHooks` shrinks to an internal adapter; the tray no longer reaches into
  daemon internals.
- Phase 2 becomes a mechanical extraction rather than a redesign.

### What becomes easier after Phase 2

- **Crash isolation.** A `systray` panic, a Cocoa-thread deadlock, or a tray
  bug can no longer take down log monitoring or event dispatch.
- **One uniform daemon binary.** The `cgo`/`!cgo` build-tag fork disappears;
  cross-compilation stops being special-cased.
- **Independent release cadence.** Tray UI fixes ship without re-releasing the
  daemon and vice versa.
- **Cleaner update story.** The headless daemon can self-update without a UI
  process holding file handles.

### What becomes harder / costs

- **Two processes to install, supervise, and update.** The installer, the
  uninstaller (`localapi/uninstall_*.go`), and the auto-update logic must each
  handle two binaries and two LaunchAgents/Tasks.
- **A new failure mode: "daemon up, tray down" and vice versa.** The tray must
  render a graceful "daemon offline" state; the daemon must not assume a tray
  exists.
- **IPC versioning.** Once the tray is a separate binary it can be a different
  version than the daemon. The `localapi` contract must be versioned and
  backward-compatible (it already lives under `/api/v1/`, which helps).
- **macOS still pins `systray.Run` to the main thread** — Phase 2 does not
  remove that constraint, it just moves it into the *tray* process where it
  belongs instead of the daemon process.

### Risk assessment

**What breaks if the tray crashes *today* (current architecture):**

- `systray.Run()` is the outermost call in the process. A `systray` panic or a
  Cocoa main-thread fault **terminates the entire process**, including the
  `svc.Run` goroutine — log monitoring, event dispatch, GRE flushing, and
  collection sync all stop.
- Even a non-fatal tray-goroutine wedge (`tray.App.loop` blocked) stops
  `SyncNow`/`GrantAccess`/`TryAgain` from being delivered, while the daemon's
  `select` cases for them silently never fire — a hard-to-diagnose hang.
- launchd would respawn the process (`KeepAlive=true`), but that is a full
  cold restart: lost session ID, re-snapshot of `Player.log`, re-auth checks.

**What improves after separation:**

- Tray crash → daemon keeps running; launchd respawns only the tray. Zero data
  loss, zero session reset.
- Daemon crash → tray survives and shows "daemon offline"; launchd respawns the
  daemon; tray reconnects on its next poll.
- The blast radius of any UI bug is contained to the UI process.

**Risks introduced by the migration itself:**

- *Installer regression* — mitigated by deferring Phase 2 to v0.4.0 and keeping
  it out of the pre-beta v0.3.x window.
- *IPC contract drift between daemon and tray versions* — mitigated by the
  `/api/v1/` versioned namespace and a compatibility check on tray startup.
- *Phase 1 introduces a behavior change in command routing* — mitigated by
  keeping `TrayHooks` as the internal adapter so the daemon event loop is
  unchanged; only the *feed* into those channels changes.

## Alternatives Considered

### A. Keep the in-process tray; just add a panic recover around `systray`

Wrap `systray.Run` and `tray.App.loop` in `recover()`. **Rejected** — a
`recover` cannot save the process when the Cocoa main thread itself faults, and
it does nothing for the silent "channel not drained" hang. It treats a symptom,
not the coupling. It also leaves the `cgo`/`!cgo` fork in place.

### B. Full process split now, in v0.3.x

Do Phase 1 and Phase 2 together before beta. **Rejected** — Phase 2 changes the
installer, adds a second LaunchAgent/Task and a second release artifact, and
intersects ADR-0011's distribution work. Landing that immediately before the
beta cut is unnecessary risk for a problem that is not an active production
fire. Phase 1 captures most of the de-risking value at a fraction of the cost.

### C. Make the SPA the only UI; delete the tray entirely

Return to ADR-0011's original "headless daemon, SPA owns all surfaces"
position and delete `internal/tray` outright. **Rejected as a decision, noted
as a fallback** — a menubar/tray icon is the conventional discoverability
surface for a background tool, and ADR-0011 §"Consequences" already flagged
"no tray icon means some users will not realize the daemon is running" as a
known downside. If product decides the tray is not worth two-process
complexity, deleting it is cleaner than half-coupling it — but that is a
product call, not an architecture default. The recommendation here assumes the
tray stays.

### D. D-Bus / OS-native IPC instead of loopback HTTP

Use D-Bus (Linux) / XPC (macOS) / named pipes (Windows) for the daemon↔tray
channel. **Rejected for v1** — `localapi` already exists, already works, and is
already a dependency of the SPA. Three platform-specific IPC transports is more
surface than a single loopback HTTP API. A Unix-domain-socket transport is kept
as an *optional* Phase 2 hardening step (Decision step 7) because it adds
file-permission access control with minimal new code, but it is not required.

## Implementation Tickets

Filed by PM on the appropriate board. **Phase 1 = v0.3.x / Wave 0; Phase 2 =
v0.4.0.**

### Phase 1 — foundational (do now, v0.3.x)

| # | Title | Effort | Agent |
|---|---|---|---|
| P1-1 | localapi: add `POST /api/v1/system/{sync-now,grant-access,retry-keychain}` command endpoints wired to daemon channels | ~2h | backend-engineer |
| P1-2 | localapi: extend `GET /api/v1/system/status` with `helper_installed`, `keychain_error`, `last_sync` fields | ~1.5h | backend-engineer |
| P1-3 | daemon: make `TrayHooks` an internal adapter fed by `localapi` (tray reads/writes daemon only via loopback API, never shared channels) | ~2h | Ray (daemon, architectural) |
| P1-4 | docs: amend ADR-0011 §6 to reflect that a tray exists; cross-link ADR-0021 | ~0.5h | Ray |

### Phase 2 — process split (defer to v0.4.0)

| # | Title | Effort | Agent |
|---|---|---|---|
| P2-1 | Create `localapi/client` package — thin typed Go client for the daemon control API | ~2h | backend-engineer |
| P2-2 | Extract `mtga-tray` as a standalone `cmd/tray/` binary owning `systray.Run`, consuming `localapi/client` | ~3h | Ray (daemon) |
| P2-3 | Strip `systray` from `cmd/daemon/main.go`; run `svc.Run` on the main goroutine; delete the `cgo`/`!cgo` tray build-tag fork | ~2h | Ray (daemon) |
| P2-4 | Installer: register a second LaunchAgent (`com.vaultmtg.tray`) / Scheduled Task; ship the tray binary in the `.pkg`/installer | ~3h | infrastructure |
| P2-5 | Auto-update: handle two binaries / two services in the update flow | ~2h | backend-engineer |
| P2-6 | Tray: graceful "daemon offline" state when the control-API poll fails | ~1.5h | backend-engineer |
| P2-7 (optional) | Migrate the daemon↔tray transport from loopback TCP to a Unix-domain socket | ~3h | Ray (daemon) |
