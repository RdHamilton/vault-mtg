# Live Draft View — User Guide

The Live Draft view (`/draft/live`) is a real-time pick assistant that runs alongside MTG Arena. It reads your draft as you play and shows card grades and pick recommendations for every pack you open.

---

## Before You Start

- The **VaultMTG daemon** must be installed and connected (green dot in the app header). If it is not, see the [daemon installation guide](daemon-installation.md).
- You must be **signed in** to VaultMTG at `app.vaultmtg.app`.
- MTG Arena must be **running** on the same computer as the daemon.

---

## How to Open the Live Draft View

The Live Draft view is designed to run in a **second browser window** or on a **second monitor** next to MTG Arena.

### Step 1 — Open the Live Draft view

1. Go to `app.vaultmtg.app` in your browser.
2. Click **Draft** in the left navigation, then choose **Live Draft** from the submenu.
   - Direct URL: `app.vaultmtg.app/draft/live`

### Step 2 — Move it to a second window or monitor (recommended)

**Two-monitor setup:**

1. With the Live Draft tab open, right-click the tab in your browser and choose **Move tab to new window**.
2. Drag the new window to your second monitor.
3. Resize it to fit next to Arena.

**Single-monitor setup:**

1. Use **windowed** or **borderless windowed** mode in MTG Arena (not fullscreen exclusive) so you can alt-tab without losing focus.
2. Open the Live Draft view in a second browser window and snap it to half your screen.
3. Alt-tab between Arena and the browser when you need to check grades.

> The Live Draft view works in any modern browser (Chrome, Firefox, Edge, Safari). No extension is required.

### Step 3 — Start a draft in Arena

Once you enter a draft in MTG Arena, the Live Draft view will automatically detect it and switch from the waiting screen to the active draft display. No button needs to be clicked.

---

## What the Live Draft View Shows

### Header

| Element | What it means |
|---|---|
| **Set code** (e.g. `BLB`) | The set being drafted, detected automatically from the pack |
| **Format** (e.g. `Premier Draft`, `Quick Draft`) | The draft format, detected automatically |
| **Pack X · Pick Y** | Your current position in the draft |
| **Stream status** | Connection state: `open` (connected), `connecting` (reconnecting), `error` |

### Current Pack

Each card in the current pack is listed with:

| Element | What it means |
|---|---|
| **Card name** | The card's name pulled from VaultMTG's ratings database |
| **Grade** (A+, A, A−, B+, … F) | Letter grade derived from the card's Game-In-Hand Win Rate (GIHWR) as sourced from 17Lands |
| **GIHWR %** | The raw win-rate number behind the grade |
| **Top Pick badge** | Shown on whichever card in the pack has the highest GIHWR |

**Grade scale:**

| Grade | GIHWR |
|---|---|
| A+ | 65% or higher |
| A  | 62–64.9% |
| A− | 59–61.9% |
| B+ | 57–58.9% |
| B  | 55–56.9% |
| B− | 53–54.9% |
| C+ | 51–52.9% |
| C  | 49–50.9% |
| C− | 47–48.9% |
| D  | 45–46.9% |
| F  | Below 45% |

Cards with no rating data show `—` for the grade. This is normal for new cards or sets that 17Lands has not yet fully sampled.

### Pick History

Below the current pack, the **Picks** panel shows every card you have selected this draft, in order, with its name and grade. This is useful for reviewing your pool between picks.

### Draft complete

When your draft ends, the page shows a confirmation message. Your full pick history is available in [Draft History](../COLLECTION.md) after the session closes.

---

## Supported Draft Formats

The Live Draft view works with any draft format that the daemon reports:

- **Premier Draft** — traditional 8-player booster draft
- **Quick Draft** — bot draft
- **Traditional Draft** — best-of-three premier draft

Sealed and other non-draft formats are not supported and will not trigger the live view.

---

## Troubleshooting

### Grades are not appearing / all cards show `—`

**Most likely cause:** The card ratings for the current set have not loaded yet.

1. Check the stream status indicator in the header. If it shows `connecting` or `error`, the daemon is not sending events — see below.
2. Ratings load automatically when the first pack is detected. If the pack is displayed but all grades are `—`, the set may not yet have ratings data on 17Lands. Ratings become available after a set has been in the wild for a week or two.
3. If ratings loaded for a previous set but not the current one, try refreshing the page after the first pack appears.

### The page says "No active draft"

**Possible causes:**

- The daemon is not running or not connected. Check the health indicator on `app.vaultmtg.app`. A gray or red dot means the daemon is offline. See [daemon troubleshooting](daemon-troubleshooting.md).
- You have not entered a draft in Arena yet. The page waits for a `draft.started` event and will activate automatically when you join a pod or start a bot draft.
- You are running a Sealed event, which is not supported.

### The stream status shows `error` or `connecting`

The Live Draft view reconnects automatically with exponential backoff (up to 30 seconds between attempts). If it stays in `error` for more than a minute:

1. Confirm the daemon is running (menu bar icon on macOS, system tray icon on Windows).
2. Confirm the daemon status dot in VaultMTG is green.
3. Refresh the Live Draft page.
4. If the problem persists, restart the daemon and refresh.

### A card in my pack is missing its name

If a card shows as `#<number>` instead of its name, the card's Arena ID is not yet in VaultMTG's ratings database. This is rare and typically resolves with the next ratings update. The grade will show `—` for the same reason. The card is still shown in the pack grid so you can see your full options.

### The pick history is empty even though I have made picks

The pick history is populated from live events. If you opened the Live Draft view after making picks, earlier picks in the session will not appear — the view only captures events received while the page is open. For a full pick log, use Draft History after the session ends.

---

## Tips

- Open the Live Draft view **before** you queue for a draft so it is ready when the first pack drops.
- On a single monitor, use **borderless windowed** mode in Arena. Fullscreen exclusive mode may lose focus when you alt-tab.
- The Top Pick badge highlights the statistically strongest card in the pack based on win-rate data, but it does not account for your current pool or archetype. Use it as a starting point, not a final answer.
- Grades update as soon as a new pack event arrives — you do not need to refresh.

---

## Related

- [Daemon Installation](daemon-installation.md)
- [Daemon Troubleshooting](daemon-troubleshooting.md)
- [FAQ](faq.md)
