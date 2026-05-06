# VaultMTG — Frequently Asked Questions

---

## 1. The daemon is not connecting after I installed it

### Quick Answer
The daemon needs permission to read your MTG Arena log file. A firewall, antivirus, or incorrect log path is blocking it in most cases.

### Step by Step

1. **Check that MTG Arena has been launched at least once.** The log file does not exist until Arena runs. Open Arena, play a game or navigate to your collection, then close it.

2. **Verify the daemon is running.**
   - Mac: Open Activity Monitor and search for `vaultmtg-daemon`. If it is not listed, relaunch the VaultMTG app.
   - Windows: Open Task Manager > Details tab and search for `vaultmtg-daemon.exe`.

3. **Check the log path in VaultMTG settings.**
   Open VaultMTG > Settings > Daemon. Confirm the Player.log path matches the location on your machine (see FAQ #3 for the correct path on Mac and Windows).

4. **Allow the daemon through your firewall/antivirus.**
   Some antivirus tools (Malwarebytes, Windows Defender, Avast) block unknown executables from reading files. Add `vaultmtg-daemon` (or `vaultmtg-daemon.exe`) to your exceptions list.

5. **Restart the daemon.**
   Quit VaultMTG completely and relaunch it.

### If That Doesn't Work
Post in Discord #help with your operating system, VaultMTG version (Settings > About), and a screenshot of the Daemon settings screen. You can also use the chat icon on vaultmtg.app.

---

## 2. The daemon connected but no data is showing in the app

### Quick Answer
The app only shows data from games played after the daemon was running. Past games are not imported automatically.

### Step by Step

1. **Play a game in MTG Arena with VaultMTG running.**
   The daemon reads your log in real time. Data from games played before the daemon was connected is not available.

2. **Wait a few seconds after the game ends.**
   There is a short delay between the game ending in Arena and the data appearing in VaultMTG (usually under 10 seconds).

3. **Check your connection status.**
   In VaultMTG, the top bar shows a green dot when the daemon is connected. If it shows red or grey, see FAQ #1.

4. **Force a refresh.**
   Click the refresh icon in the top navigation bar of VaultMTG to trigger a data reload.

5. **Check that you are logged in.**
   Your data is tied to your VaultMTG account. Open Settings > Account and confirm you are signed in.

[Screenshot placeholder: VaultMTG top bar showing daemon connection status]

### If That Doesn't Work
Post in Discord #help with your OS, VaultMTG version, and the type of game mode you were playing (Draft, Ranked, Casual, etc.).

---

## 3. How do I find my MTG Arena Player.log path?

### Quick Answer
The Player.log file is in a fixed location based on your operating system. Copy the path below and paste it into VaultMTG Settings > Daemon > Log Path.

### Mac

```
~/Library/Logs/Wizards of the Coast/MTGA/Player.log
```

To navigate there in Finder:
1. Open Finder
2. Click Go in the menu bar, then Go to Folder
3. Paste: `~/Library/Logs/Wizards of the Coast/MTGA/`
4. Press Enter
5. You should see `Player.log` in this folder

[Screenshot placeholder: Mac Finder with the MTGA log folder open]

### Windows

```
%APPDATA%\..\LocalLow\Wizards of the Coast\MTGA\Player.log
```

To navigate there in File Explorer:
1. Press Windows + R to open Run
2. Paste: `%APPDATA%\..\LocalLow\Wizards of the Coast\MTGA\`
3. Press Enter
4. You should see `Player.log` in this folder

[Screenshot placeholder: Windows File Explorer with the MTGA log folder open]

### Notes
- The file only exists after MTG Arena has been installed and launched at least once.
- The file is overwritten each time Arena launches. VaultMTG reads it while Arena is running.

### If That Doesn't Work
If you installed MTG Arena to a non-standard location, the path may differ. Post in Discord #help with your OS and Arena install location.

---

## 4. Which platforms are supported? What are the minimum OS versions?

### Quick Answer
VaultMTG supports Mac and Windows. Linux is not supported at this time.

### Supported Platforms

| Platform | Minimum Version | Notes |
|---|---|---|
| macOS | macOS 12 Monterey | Apple Silicon (M1/M2/M3) and Intel both supported |
| Windows | Windows 10 (64-bit) | Windows 11 also supported |
| Linux | Not supported | No current timeline |
| iOS / Android | Not supported | Mobile is not on the current roadmap |

### MTG Arena Requirement
VaultMTG requires MTG Arena to be installed and run on the same machine. It does not work with Arena on a separate computer.

### If You Are on an Unsupported Platform
Post in Discord #feedback if you want to be notified if Linux or mobile support is added in the future.

---

## 5. How do I report a bug?

### Quick Answer
Post in Discord #bugs or use the in-app chat widget with your OS, app version, and steps to reproduce the issue.

### What to Include

A good bug report helps us fix the issue faster. Include:

1. **Steps to reproduce** — exactly what you did before the bug happened
2. **Expected behavior** — what you thought would happen
3. **Actual behavior** — what actually happened
4. **Your OS** — Mac or Windows, and the version (e.g., macOS 14.4, Windows 11)
5. **VaultMTG version** — found in VaultMTG > Settings > About
6. **MTG Arena version** (if relevant) — found in the Arena client
7. **Screenshot or screen recording** — if the bug is visual

### Where to Report

- **Discord #bugs** — fastest way to reach the team; a GitHub issue is created for every reproducible bug
- **In-app chat widget** — click the chat icon on any page of vaultmtg.app
- **Email** — support@vaultmtg.app (placeholder — set up before beta launch)

### What Happens After You Report

1. The support team acknowledges your report within 48 hours (weekdays)
2. If reproducible, a GitHub issue is created and you will receive the issue number
3. When the bug is fixed, you will be notified in the same thread or channel

### If Your Bug Involves Personal Data
If your bug report requires sharing account information or log files containing personal data, use the in-app chat (private) rather than Discord (public).
