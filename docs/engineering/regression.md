# Manual Regression Test Plan

Run this plan before every release. P0 failures block the ship. P1 failures require investigation. P2 failures are documented and shipped anyway.

---

## Prerequisites (apply to all flows)

- BFF running (EC2 in prod, `go run ./services/bff/cmd/bff` locally)
- Frontend deployed or running locally (`cd frontend && npm run dev`)
- Daemon binary installed and started (`./mtga-companion service start`)
- MTGA installed and logged in with a test account
- Browser console open to catch JS errors throughout

---

## P0 — Must Pass Before Any Release

These failures mean do not ship.

---

### P0-01: BFF Health Endpoint

**Prerequisites**: BFF running

**Steps**:
1. `curl -s https://<production-bff-url>/api/v1/health`
2. Check HTTP status code

**Expected**: HTTP 200, response body contains `"status": "ok"` or similar healthy indicator

**Common failures**:
- 502/503 → BFF process crashed or EC2 instance is down; check systemd logs
- 404 → deployment used wrong binary or routing misconfigured
- CORS error in browser → check BFF `ALLOWED_ORIGINS` env var includes production frontend domain

---

### P0-02: SPA Loads at Production URL

**Prerequisites**: Frontend deployed to Vercel (or CDN)

**Steps**:
1. Open production URL in a fresh browser tab (incognito preferred)
2. Wait up to 15 seconds
3. Check that the app renders (navigation visible, no blank white screen)
4. Open DevTools console — verify no uncaught JS errors on load

**Expected**: App container visible, page title non-empty, no console errors

**Common failures**:
- Blank screen → check `VITE_API_BASE_URL` env var is set correctly on the Vercel deployment
- "Failed to fetch" console errors → CORS misconfigured; see P0-01
- 404 for all routes → Vercel rewrite rules missing; check `vercel.json`

---

### P0-03: Daemon Installs and Connects

**Prerequisites**: Clean machine (or uninstalled daemon), daemon release binary

**Steps**:
1. Download the daemon release binary for the current version
2. Run: `./mtga-companion service install`
3. Run: `./mtga-companion service start`
4. Run: `./mtga-companion service status`
5. Open the app in a browser
6. Navigate to Settings → Daemon Connection
7. Verify connection status

**Expected**:
- `service install` exits 0 with "Service installed successfully"
- `service status` shows "Status: Running"
- `curl http://localhost:9999/status` returns `{"status":"running",...}`
- App Settings page shows daemon as Connected (green indicator)

**Common failures**:
- Port 9999 in use → another process is occupying it; `lsof -i :9999`
- "Permission denied" → re-run install with appropriate privileges
- Daemon shows running but UI shows disconnected → check UI's configured daemon port matches

---

### P0-04: Draft — Pack Appears and Pick Is Recorded

**Prerequisites**: Daemon running and connected, MTGA open with an active Premier or Quick draft event

**Steps**:
1. Enter a draft event in MTGA
2. When the first pack opens, switch to the app's Draft tab
3. Observe the current pack display
4. Pick a card in MTGA
5. Return to app; observe pick recorded and pack advances

**Expected**:
- Cards in the current pack are visible in the Draft UI before picking
- After picking, the picked card appears in the "Pool" or picks list
- Pack number and pick number counter advance correctly

**Common failures**:
- No cards appear → daemon is not reading MTGA log; check `~/Library/Logs/MTGACompanionDaemon.log` (macOS) for parsing errors
- Cards visible but pick not recorded → `draft:pick` event not being emitted; check daemon logs for the event
- Draft tab shows loading spinner indefinitely → BFF `/api/v1/draft-ratings` unreachable; check BFF logs

---

### P0-05: Match Win/Loss Records and Win Rate Calculates

**Prerequisites**: Daemon running, MTGA open

**Steps**:
1. Note current match count and win rate in the Match History tab
2. Play and complete one match in MTGA (win or loss)
3. Return to the app's Match History tab
4. Refresh if needed

**Expected**:
- New match appears in match history list with correct result (Win or Loss)
- Total match count increments by 1
- Win rate percentage recalculates correctly

**Common failures**:
- Match not appearing → daemon did not parse the log event; check daemon logs for `match:new` event emission
- Win rate stuck → stats endpoint returning cached data; check BFF `/api/v1/matches/stats` response
- Wrong result shown → log parsing bug; note the match ID and file a bug

---

### P0-06: Decks List Loads

**Prerequisites**: BFF running, at least one deck exists in the database

**Steps**:
1. Navigate to the Decks tab
2. Wait for load to complete

**Expected**:
- Deck list renders with deck names visible
- No error state or empty state when decks exist

**Common failures**:
- Error state shown → `GET /api/v1/decks` failing; check BFF logs
- Empty state when decks should exist → database query issue; check BFF DB connection
- Perpetual spinner → network timeout; check if BFF is reachable from the frontend

---

## P1 — Should Pass Before Shipping

Investigate any failures; use judgment on whether to delay the release.

---

### P1-01: Draft Ratings Render for Current Pack

**Prerequisites**: Active draft session as in P0-04

**Steps**:
1. Open the app to the Draft tab during a live draft pack
2. Verify each card in the current pack has a rating visible
3. Check that tier indicators (if applicable) are present
4. Check that color ratings panel shows correct mana distribution for picked cards

**Expected**:
- Every card in the pack has a visible rating badge (number, letter grade, or tier label)
- Color ratings panel reflects the colors of cards already picked
- No cards show "N/A" or missing rating unless the set data is genuinely absent

**Common failures**:
- All ratings missing → BFF `/api/v1/draft-ratings` returned empty; check set code is registered in the fetcher
- Some cards missing ratings → specific card IDs not in the ratings dataset; note the set and card IDs
- Color panel empty → no picks yet is expected; if picks exist and panel is empty, check `ColorRatingsPanel` data flow

---

### P1-02: Draft Pick Count Updates Per Pick

**Prerequisites**: Active draft session

**Steps**:
1. Note the current pick count display in the Draft UI
2. Make a pick in MTGA
3. Verify pick count increments in app
4. Continue through at least 3 picks

**Expected**: Pick counter increments 1-for-1 with each MTGA pick; pack number updates at pack boundaries (pick 15 → pack 2)

**Common failures**:
- Counter not incrementing → WebSocket `draft:pick` event not received by UI; check browser WS connection
- Counter skips values → duplicate events or missed events; check daemon log volume

---

### P1-03: Deck Detail Shows Cards

**Prerequisites**: At least one deck with cards in the database

**Steps**:
1. Navigate to the Decks tab
2. Click on a deck to open its detail view
3. Verify maindeck and sideboard cards render

**Expected**: Card names, quantities, and at minimum mana cost are visible; no blank card list

**Common failures**:
- Empty card list → `GET /api/v1/decks/{id}` returning deck without cards; check BFF handler and DB query
- Cards listed but no names → card metadata not joined; check if card IDs resolve to names

---

### P1-04: BFF Daemon Version Endpoint

**Prerequisites**: Daemon running, BFF running

**Steps**:
1. `curl -s http://<bff-url>/api/v1/daemon/version`
2. Note the version string in the response

**Expected**: Returns a JSON response containing the current daemon version string; HTTP 200

**Common failures**:
- 404 → handler not registered; check BFF router
- Empty version → daemon version not injected at build time; check release build flags

---

### P1-05: Daemon Version Check Logs Warning When Outdated

**Prerequisites**: An older daemon binary running, newer release published on GitHub

**Steps**:
1. Run an older daemon version
2. Wait up to 60 seconds (check fires on startup)
3. Check daemon logs

**Expected**: Log line like `WARN: new version available: X.Y.Z (current: A.B.C)` appears within startup

**Common failures**:
- No warning → version check disabled via env var (`MTGA_DAEMON_DISABLE_UPDATE_CHECK=1`) or GitHub API unreachable
- Warning shows wrong version → BFF version response stale; check BFF `/api/v1/daemon/version` response

---

### P1-06: Deck Builder — Create and View Deck

**Prerequisites**: BFF running

**Steps**:
1. Navigate to the Deck Builder page
2. Create a new deck (enter name, select format)
3. Add at least 2 cards via the card search
4. Save the deck
5. Navigate away and return to the deck

**Expected**: Deck persists with the correct name, format, and card list after navigation

**Common failures**:
- Save fails → `POST /api/v1/decks` error; check BFF logs and DB write permissions
- Deck lost after navigation → not persisted (only held in local state); confirm save was called
- Card search returns no results → card search index not populated; check BFF card search endpoint

---

### P1-07: Match History — Empty State

**Prerequisites**: A fresh account with no matches

**Steps**:
1. Open Match History tab with an account that has no matches recorded

**Expected**: Empty state UI renders ("No matches yet" or equivalent), not an error state or crash

**Common failures**:
- Error state instead of empty state → API returning 500 for empty result; check BFF handler null safety
- Crash (blank page) → component not handling empty array; file a P1 bug

---

### P1-08: Collection Tab Loads

[PENDING: Phase 3 — collection sync not yet fully implemented]

**Steps** (when implemented):
1. Navigate to the Collection tab
2. Verify card collection data is visible

**Expected**: Collection cards visible with set, quantity, and rarity

---

## P2 — Nice to Verify

Document failures, ship anyway. Track in GitHub issues.

---

### P2-01: Auth — Login and Logout Flow

[PENDING: Clerk integration — Phase 3]

When Clerk is wired:
1. Visit the app unauthenticated
2. Verify redirect to login
3. Log in with test credentials
4. Verify authenticated state in UI
5. Log out
6. Verify session cleared and redirect to login

---

### P2-02: Draft Analytics Page Renders

**Steps**:
1. Navigate to Draft Analytics
2. Verify charts and statistics load for completed drafts

**Expected**: At least one chart renders if draft history exists; empty state shown if none

---

### P2-03: Meta Page Loads

**Steps**:
1. Navigate to the Meta tab
2. Verify format distribution and metagame data loads

**Expected**: Charts or tables render; no perpetual spinner

---

### P2-04: Settings Persist Across Reload

**Steps**:
1. Open Settings and change at least one preference
2. Save
3. Hard-reload the page
4. Return to Settings

**Expected**: Changed value persists

---

### P2-05: Responsive Layout — Mobile Viewport

**Steps**:
1. Open the app at 375px width (DevTools mobile emulation)
2. Navigate through: Home, Drafts, Decks, Match History, Settings

**Expected**: No horizontal overflow, navigation accessible, critical content readable

---

### P2-06: Daemon Auto-Start on Boot

**Steps**:
1. Install daemon
2. Restart the machine
3. After login, wait 30 seconds
4. Check `./mtga-companion service status`

**Expected**: Status shows "Running" without manual start

**Common failures**:
- Not auto-started on macOS → Launch Agent plist not loaded; `launchctl list | grep MTGACompanionDaemon`
- Not auto-started on Windows → service startup type not set to Automatic; check `sc.exe qc MTGACompanionDaemon`

---

### P2-07: Daemon Reconnects After UI Reload

**Steps**:
1. With daemon running and UI showing Connected
2. Hard-reload the browser
3. Wait up to 15 seconds

**Expected**: UI reconnects to daemon without requiring a daemon restart

---

### P2-08: Download Page — Daemon Binary Link Works

**Steps**:
1. Navigate to the Download page in the app
2. Click the download link for the current platform

**Expected**: Download starts or redirects to GitHub releases page with the correct version

---

## Notes

- Automated smoke tests cover P0-01 (health), P0-02 (SPA loads), and decks endpoint connectivity. See `frontend/tests/e2e/smoke.spec.ts`.
- Flows marked `[PENDING: Phase N]` should be converted to standard P0/P1 items once the feature ships.
- For daemon flows, always test with a fresh install of the release binary, not a dev build.
