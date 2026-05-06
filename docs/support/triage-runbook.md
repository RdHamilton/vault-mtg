# Support Triage Runbook

This runbook is for internal use by the customer-success team. It provides decision trees for the most common incoming support requests.

---

## How to Use This Runbook

1. Read the user's message and identify the category (Daemon, Data, Crash, Bug)
2. Walk through the checklist for that category
3. If resolved: thank the user, close the ticket
4. If not resolved: escalate using the path at the bottom of each section

Response time targets: acknowledge within 24 hours, first diagnostic attempt within 48 hours.

---

## Category 1: Daemon Not Connecting

**Trigger phrases**: "daemon", "not connecting", "red dot", "can't connect", "daemon offline"

### Checklist

- [ ] **Has MTG Arena been launched at least once since install?**
  If no: ask user to launch Arena and play one game, then check again.

- [ ] **Is the daemon process running?**
  Mac: Activity Monitor > search `vaultmtg-daemon`
  Windows: Task Manager > Details > search `vaultmtg-daemon.exe`
  If no: ask user to quit and relaunch VaultMTG.

- [ ] **Is the Player.log path correct?**
  Ask user to open VaultMTG > Settings > Daemon and share what path is shown.
  Compare to FAQ #3 expected paths:
  - Mac: `~/Library/Logs/Wizards of the Coast/MTGA/Player.log`
  - Windows: `%APPDATA%\..\LocalLow\Wizards of the Coast\MTGA\Player.log`
  If wrong: guide user to correct it using FAQ #3.

- [ ] **Is antivirus or firewall blocking the daemon?**
  Ask if user has antivirus software (Malwarebytes, Avast, Windows Defender real-time protection).
  If yes: ask user to add `vaultmtg-daemon` to exceptions and retry.

- [ ] **Is Arena currently running?**
  The daemon needs Arena to be active to read the log. Ask user to confirm Arena is open.

- [ ] **Has the user tried restarting both apps?**
  Quit VaultMTG and Arena fully, relaunch Arena first, then VaultMTG.

### Escalation
If none of the above resolves it:
1. Ask user to share: OS + version, VaultMTG version, antivirus software name, screenshot of Daemon settings screen
2. Create a GitHub issue with label `bug` and tag `daemon-connection`
3. Assign to engineering for log-level investigation
4. Tell user: "I've logged this as issue #NNN — the engineering team will investigate."

---

## Category 2: Data Missing or Not Showing

**Trigger phrases**: "no data", "nothing showing", "stats missing", "history empty", "matches not appearing"

### Checklist

- [ ] **Was the daemon connected when the games were played?**
  Data is only captured in real time. Games played before daemon connection are not available.
  If no: inform user (see FAQ #2 language) and ask them to play a game with the daemon running.

- [ ] **How recent are the missing games?**
  There is a short delay (under 10 seconds) between game end and data appearing.
  If very recent: ask user to wait 30 seconds and refresh.

- [ ] **Is the user logged in to their VaultMTG account?**
  Ask user to check Settings > Account. Data is scoped to their account.
  If not logged in: guide them to sign in.

- [ ] **Has the user tried the refresh button?**
  Ask user to click the refresh icon in the top nav bar.

- [ ] **What game mode was being played?**
  Note: some game modes may have limited support in early beta. If the user was playing an unsupported mode, inform them.

- [ ] **Is the daemon status green or red during gameplay?**
  If red during gameplay, re-route to Category 1 (Daemon Not Connecting).

### Escalation
If data is confirmed missing after a session where daemon was green:
1. Ask for: OS, VaultMTG version, game mode played, approximate time of the session
2. Create a GitHub issue with label `bug` and tag `data-missing`
3. Include game mode and timing in the issue body
4. Tell user: "I've logged this as issue #NNN — the team will investigate."

---

## Category 3: App Crash

**Trigger phrases**: "crashed", "closes itself", "won't open", "freezes", "spinning wheel", "not responding"

### Information to Collect Before Escalating

Do not escalate without all of the following:

| Item | How to get it |
|---|---|
| OS and version | Mac: Apple menu > About This Mac; Windows: Settings > System > About |
| VaultMTG version | VaultMTG > Settings > About (if app opens at all) |
| MTG Arena version | Arena client settings if relevant |
| Crash log (Mac) | Applications > Utilities > Console > Crash Reports > vaultmtg |
| Event Viewer log (Windows) | Start > Event Viewer > Windows Logs > Application > filter by vaultmtg |
| Steps before crash | Exactly what the user was doing when it crashed |
| Is it reproducible? | Does it happen every time, or was it a one-time event? |

### Checklist

- [ ] **Is this the first launch after install?**
  Ask user if they are launching for the first time. A permissions dialog may have been missed.

- [ ] **Does it crash on a specific action?**
  Ask user to describe what they were doing. Note if it was: opening a draft, viewing match history, connecting daemon, signing in.

- [ ] **Has the user restarted their machine?**
  Ask user to restart and try again before collecting logs.

- [ ] **Is the app version current?**
  Ask user to check for updates in VaultMTG > Settings > About or re-download from vaultmtg.app.

### Escalation
1. Collect all items from the table above
2. Create a GitHub issue with label `bug` and tag `crash`
3. Attach or quote the crash log in the issue body
4. Mark as high priority if multiple users report the same crash
5. Tell user: "I've logged this as issue #NNN — the engineering team will investigate as a priority."

---

## Category 4: General Bug Report

**Trigger phrases**: "bug", "broken", "doesn't work", "wrong", "incorrect data", "not working as expected"

### Checklist

- [ ] **Can you reproduce it?**
  Ask for exact steps to reproduce. If the user cannot reproduce it, log it as a one-time anomaly and monitor for recurrence.

- [ ] **Is there a screenshot or recording?**
  Ask the user to share one if the bug is visual.

- [ ] **Do you have enough information to file a GitHub issue?**
  Required: steps to reproduce, expected behavior, actual behavior, OS, app version.

### Creating a GitHub Issue

Use this command template (fill in fields before running):

```bash
gh issue create \
  --repo RdHamilton/MTGA-Companion \
  --title "bug: [concise description]" \
  --body "## Bug Report

**Reported by**: [Discord username / Crisp ticket ID]
**Date**: YYYY-MM-DD

## Steps to Reproduce
1. 
2. 
3. 

## Expected Behavior
[What should happen]

## Actual Behavior
[What actually happens]

## Environment
- App version: 
- OS: 
- MTG Arena version (if relevant): 

## Additional Context
[Screenshot, error message, frequency]" \
  --label "bug"
```

After creating the issue:
1. Add it to Project #27 (or current release project)
2. Reply to the user: "Thanks for reporting this — I've logged it as issue #NNN and the team will investigate."

### Escalation Thresholds

| Condition | Action |
|---|---|
| 1 report | Log issue, standard triage |
| 3-4 reports of same bug | Flag to engineering in standup; increase priority |
| 5+ reports | Escalate immediately; post acknowledgment in Discord #bugs; consider status page update |
| Data corruption or data loss | Treat as P0; alert engineering immediately regardless of report count |

---

## Closing a Ticket

A ticket is closed when:
- The issue is resolved and the user confirms it
- The bug is fixed in a released version and the user has updated
- The request was a feature request (acknowledged, logged, user thanked)
- No response from user after 7 days of attempted follow-up

Always send a closing message to the user before closing.
