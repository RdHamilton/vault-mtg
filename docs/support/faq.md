# VaultMTG Beta — Frequently Asked Questions

---

## 1. What is VaultMTG and what does it do?

### Quick Answer
VaultMTG is a companion tool for MTG Arena players. It tracks your matches, drafts, and deck performance so you can review your results and improve over time.

### How It Works
A small background program called the daemon runs on your computer alongside MTG Arena. It reads Arena's log file in real time and sends your game data to vaultmtg.app, where you can view your stats, draft history, and match records.

VaultMTG does not interact with or modify MTG Arena in any way. It only reads a log file that Arena creates itself.

---

## 2. What data does VaultMTG collect?

### Quick Answer
VaultMTG collects only your MTG Arena gameplay data — match results, draft picks, deck lists, and in-game events. It does not collect anything unrelated to your MTG Arena sessions.

### Specifically, VaultMTG collects:
- Match results (opponent's deck archetype, outcome, turns played)
- Draft pick order and card pool
- Deck lists used in each match
- In-game events written to Arena's Player.log

### VaultMTG does not collect:
- Personal information beyond your account email
- Activity outside of MTG Arena sessions
- Keystrokes, screenshots, or clipboard contents
- Any data from other apps running on your computer
- Payment card details (billing is handled by a third-party processor)

Your data is used only to power VaultMTG features. It is never sold to third parties.

---

## 3. Is VaultMTG safe to install? Why does Windows or macOS warn me about it?

### Quick Answer
VaultMTG is safe. The warnings from macOS (Gatekeeper) and Windows (SmartScreen) appear because the beta build is not yet signed with a paid developer certificate, not because the software is harmful.

### Why the warnings appear
Apple and Microsoft require developers to purchase code-signing certificates to suppress these warnings. During the beta period, the VaultMTG installer does not have this certificate yet. The warnings are about the certificate status, not about the software itself.

### How to proceed safely
- **macOS:** Right-click the installer, choose Open, then click Open in the dialog. See the [installation guide](daemon-installation.md) for screenshots.
- **Windows:** Click More info in the SmartScreen dialog, then click Run anyway. See the [installation guide](daemon-installation.md) for screenshots.

If you prefer to wait, production releases will have signed installers. You can join the Discord to be notified.

---

## 4. Why does the daemon need to run in the background?

### Quick Answer
MTG Arena writes game events to its log file continuously while you play. The daemon must be running to read those events as they happen — there is no way to retrieve them after the fact.

### More detail
When you finish a match or draft, Arena has already written all of the relevant data to `Player.log`. If the daemon was not running during that session, that data is gone — the log file is overwritten each time Arena starts. Because the daemon needs to be present for every session you want to track, it runs at login so it is always ready.

The daemon uses very little CPU and memory (typically under 1% CPU and around 20 MB RAM) and has no measurable effect on MTG Arena's frame rate or network performance.

---

## 5. Does the daemon affect MTG Arena's performance?

### Quick Answer
No. The daemon reads a log file on disk — it does not inject code into Arena, intercept network traffic, or use the Arena API.

### Details
- The daemon reads `Player.log` sequentially as Arena appends to it. This is a standard file read with negligible system impact.
- The daemon sends small JSON payloads over HTTPS to VaultMTG's servers. This uses a trivial amount of bandwidth.
- The daemon does not hook into Arena's process, modify game files, or access Arena's memory.

VaultMTG complies with Wizards of the Coast's third-party tool policies. Reading Arena's log file is explicitly permitted.

---

## 6. Is my data stored securely?

### Quick Answer
Yes. Data is transmitted over encrypted HTTPS connections and stored in a secured database accessible only to your account.

### Specifics
- All communication between the daemon and VaultMTG servers is over TLS (HTTPS).
- Your account is protected by email + password authentication (and optionally two-factor authentication).
- Match data is stored in a database scoped to your account — other users cannot see your data.
- Passwords are never stored in plain text.
- VaultMTG does not store your MTG Arena credentials. The daemon reads the log file directly; it never asks for your Arena username or password.

During the beta period, security practices are actively reviewed. If you discover a security concern, please report it privately via the in-app chat or email support@vaultmtg.app rather than posting publicly.

---

## 7. What happens to my data if I uninstall VaultMTG?

### Quick Answer
Your match history and account data remain on VaultMTG's servers. Uninstalling the daemon removes local files only.

### What is removed from your computer
- The daemon executable
- The local config file (`daemon.json`) containing your auth token and settings
- The startup entry that launches the daemon at login

### What is not removed
- Your VaultMTG account
- Your match history, draft records, and deck data stored on VaultMTG's servers
- MTG Arena and its files (VaultMTG never modifies Arena)

If you reinstall later, sign in with the same account and all your data will be there.

To request permanent deletion of your account and all associated data, contact support@vaultmtg.app.

---

## 8. The daemon is connected but my match data is not showing up. What do I do?

### Quick Answer
The daemon only captures games played while it is running. Data from sessions where the daemon was not connected cannot be recovered.

### Step by step
1. Confirm the status indicator in VaultMTG shows a green dot.
2. Play a complete match in MTG Arena with the daemon running.
3. Wait up to 10 seconds after the game ends.
4. Click the refresh icon in the VaultMTG top bar if the data still has not appeared.

If data is still missing after following these steps, see the full [troubleshooting guide](daemon-troubleshooting.md) or post in Discord [#help](https://discord.gg/vaultmtg).

---

## 9. How do I report a bug?

### Quick Answer
Post in Discord [#bugs](https://discord.gg/vaultmtg) or use the in-app chat on vaultmtg.app. Include your OS, app version, and what you expected vs. what happened.

### What to include in your report
1. Steps to reproduce — exactly what you did before the bug appeared
2. Expected behavior — what you thought would happen
3. Actual behavior — what actually happened
4. Operating system — e.g., macOS 14.4 or Windows 11 23H2
5. VaultMTG version — found in Settings > About
6. Screenshot or screen recording if the issue is visual

### What happens after you report
- The support team will acknowledge your report within 48 hours on weekdays.
- If the bug is reproducible, a tracking issue is created and you will receive the issue number.
- When the bug is fixed, you will be notified in the same thread or channel.

---

## 10. How do I find my MTG Arena Player.log file?

### Quick Answer
The log file is in a fixed location based on your operating system.

### macOS
```
~/Library/Logs/Wizards Of The Coast/MTGA/Player.log
```

To navigate there in Finder:
1. Open Finder > click **Go** in the menu bar > **Go to Folder**.
2. Paste: `~/Library/Logs/Wizards Of The Coast/MTGA/`
3. Press Enter.

### Windows (standard install)
```
%LOCALAPPDATA%\Wizards Of The Coast\MTGA\Player.log
```

To navigate there in File Explorer:
1. Press Windows + R.
2. Paste: `%LOCALAPPDATA%\Wizards Of The Coast\MTGA\`
3. Press Enter.

### Notes
- The file only exists after MTG Arena has been installed and launched at least once.
- The file is overwritten each time Arena starts. VaultMTG reads it while Arena is running.
- If you installed Arena via the Epic Games Store, the path may differ slightly. Post in Discord [#help](https://discord.gg/vaultmtg) if you cannot locate the file.

---

## Related

- [Daemon Installation](daemon-installation.md)
- [Daemon Troubleshooting](daemon-troubleshooting.md)
- [Daemon Uninstall](daemon-uninstall.md)
