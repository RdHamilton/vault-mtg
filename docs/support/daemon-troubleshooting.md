# Daemon Troubleshooting

## Quick Answer
The colored dot in the VaultMTG app tells you the daemon's status: green means connected, yellow means reconnecting, red means disconnected. Most problems are caused by the daemon not running, a firewall block, or an incorrect config file.

---

## Understanding the Status Indicator

| Color | Meaning |
|---|---|
| Green | Daemon is running and connected to VaultMTG. Data is syncing. |
| Yellow | Daemon is trying to reconnect. Wait 30 seconds — it usually recovers on its own. |
| Red | Daemon is not connected. Action required — work through the checklist below. |

---

## Problem: Red dot — daemon is not connecting

Work through this checklist in order.

### 1. Is the daemon running?

**macOS:**
1. Open **Activity Monitor** (press Command + Space, type "Activity Monitor", press Enter).
2. In the search box at the top-right, type `vaultmtg-daemon`.
3. If it appears in the list, it is running. If not, launch it from Applications > VaultMTG Daemon.

**Windows:**
1. Open **Task Manager** (press Ctrl + Shift + Esc).
2. Click the **Details** tab.
3. Look for `vaultmtg-daemon.exe` in the list.
4. If it is not there, open the Start Menu, search for **VaultMTG Daemon**, and launch it.

### 2. Is a firewall or antivirus blocking the daemon?

The daemon makes outbound HTTPS connections to vaultmtg.app. Some security tools block unsigned executables from making network requests.

**Windows Defender / Windows Firewall:**
1. Open **Windows Security** from the Start Menu.
2. Click **Firewall and network protection** > **Allow an app through firewall**.
3. Click **Change settings**, then find `vaultmtg-daemon.exe` in the list.
4. If it is not listed, click **Allow another app** and browse to the daemon executable (default location: `C:\Program Files\VaultMTG\vaultmtg-daemon.exe`).
5. Check both **Private** and **Public** boxes, then click OK.

**macOS firewall:**
1. Open **System Settings** > **Network** > **Firewall**.
2. Click **Options** (or **Firewall Options** on older macOS).
3. Look for `vaultmtg-daemon` in the list. If it shows **Block incoming connections**, click the entry and change it to **Allow incoming connections**.

**Third-party antivirus (Malwarebytes, Avast, Norton, etc.):**
Add `vaultmtg-daemon` (macOS) or `vaultmtg-daemon.exe` (Windows) to the exceptions or exclusions list. Refer to your antivirus documentation for the exact steps.

### 3. Is the config file correct?

The daemon reads its configuration from a JSON file on your machine.

**macOS path:**
```
~/.vaultmtg/daemon.json
```
To open it: press Command + Space, type "Terminal", press Enter, then run:
```
open ~/.vaultmtg/
```

**Windows path:**
```
%APPDATA%\vaultmtg\daemon.json
```
To open it: press Windows + R, paste `%APPDATA%\vaultmtg\`, press Enter.

> If you installed the daemon before the VaultMTG rename (mid-2026) and have not reinstalled since, the config may still live at `~/.mtga-companion/daemon.json` (macOS) or `%APPDATA%\mtga-companion\daemon.json` (Windows). The daemon migrates configuration forward on first startup; reinstalling the current daemon is the cleanest fix.

Open `daemon.json` in a text editor (Notepad on Windows, TextEdit on Mac). Confirm:
- The `bff_url` value starts with `https://` and does not have a trailing slash.
- The file is valid JSON (no missing commas or brackets). If you are unsure, paste the contents into [jsonlint.com](https://jsonlint.com) to check.

If the file is missing entirely, delete and reinstall the daemon — the installer recreates it.

### 4. Restart the daemon

**macOS:** Click the menu bar icon > **Quit VaultMTG Daemon**, then relaunch from Applications.

**Windows:** Right-click the system tray icon > **Exit**, then relaunch from the Start Menu.

---

## Problem: Data not syncing after matches

The daemon is connected (green dot) but games are not appearing in VaultMTG.

### 1. Is MTG Arena writing to its log file?

The log file only updates while Arena is actively running. After quitting Arena, no new data is written.

- Open VaultMTG settings and confirm the Player.log path is correct for your system:

  **macOS:**
  ```
  ~/Library/Logs/Wizards Of The Coast/MTGA/Player.log
  ```

  **Windows (most users):**
  ```
  %LOCALAPPDATA%\Wizards Of The Coast\MTGA\Player.log
  ```

  **Windows (Epic Games Store install):** The log may be in a different location. Try:
  ```
  %LOCALAPPDATA%\Packages\Wizards Of The Coast.MTGArena_*\LocalState\Wizards Of The Coast\MTGA\Player.log
  ```

- Navigate to that folder and confirm `Player.log` exists and has a recent **Date modified** timestamp.

### 2. Did you play after the daemon was running?

The daemon only tracks games that start while it is connected. Games played before launching the daemon are not imported.

### 3. Wait a few seconds

There is a brief delay (usually under 10 seconds) between a game ending in Arena and the data appearing in VaultMTG.

### 4. Force a refresh

Click the refresh icon in the VaultMTG top navigation bar to trigger a data reload.

### 5. Check that you are signed in

Open VaultMTG Settings > Account and confirm you are logged in with the same account you used during daemon setup.

---

## Problem: Daemon crashes immediately on startup

### Missing config file

If `daemon.json` does not exist, the daemon cannot start. Reinstall the daemon — the installer creates a fresh config file.

### Wrong BFF URL in config

Open `daemon.json` and verify the `bff_url` field. It should point to `https://api.vaultmtg.app`. If it is blank or points to `localhost`, the daemon will fail to connect and may crash.

### Expired or invalid auth token

Your sign-in session may have expired. Click the menu bar icon (macOS) or system tray icon (Windows) and choose **Sign Out**, then **Sign In** again.

If the icon is not visible (because the daemon crashed before showing it), delete the `daemon.json` file and relaunch — you will be prompted to sign in again.

---

## Problem: macOS — "cannot verify developer" warning

See [Installation Guide — Step 3](daemon-installation.md#step-3--handle-the-gatekeeper-warning) for the full walkthrough.

**Short version:**
1. Click OK to dismiss the initial warning.
2. In Finder, right-click the `.dmg` or the app in Applications.
3. Choose **Open** from the right-click menu.
4. In the dialog that appears, click **Open**.

---

## Problem: Windows — "Windows protected your PC" SmartScreen warning

See [Installation Guide — Step 2](daemon-installation.md#step-2--handle-the-smartscreen-warning) for the full walkthrough.

**Short version:**
1. Click **More info** in the SmartScreen dialog.
2. Click **Run anyway**.

If **More info** is not visible, your machine may have strict SmartScreen settings enforced by an IT policy. Try right-clicking the `.exe` and choosing **Properties** > check **Unblock** at the bottom of the General tab > click OK, then run the installer again.

---

## How to Check If the Daemon Is Running

**macOS — Activity Monitor:**
1. Press Command + Space, type "Activity Monitor", press Enter.
2. Search for `vaultmtg-daemon` in the search box.
3. If it appears in the list, the process is running.

**Windows — Task Manager:**
1. Press Ctrl + Shift + Esc to open Task Manager.
2. Click the **Details** tab.
3. Look for `vaultmtg-daemon.exe`.

---

## How to Restart the Daemon Manually

**macOS:**
1. Click the VaultMTG icon in the menu bar.
2. Choose **Quit VaultMTG Daemon**.
3. Open Finder > Applications > double-click **VaultMTG Daemon**.

**Windows:**
1. Right-click the VaultMTG icon in the system tray (bottom-right, near the clock).
2. Choose **Exit**.
3. Open the Start Menu, search for **VaultMTG Daemon**, and click it to relaunch.

**If the icon is not visible (daemon is not running at all):**
- macOS: Open Applications > VaultMTG Daemon.
- Windows: Start Menu > search VaultMTG Daemon > click to launch.

---

## If That Doesn't Work

Post in Discord [#help](https://discord.gg/vaultmtg) and include:
- Your operating system and version (e.g., macOS 14.4, Windows 11 23H2)
- VaultMTG version (Settings > About)
- The status indicator color you are seeing
- What you have already tried from this guide

You can also open a support chat at vaultmtg.app using the chat icon.

## Related

- [Daemon Installation](daemon-installation.md)
- [Daemon Uninstall](daemon-uninstall.md)
- [FAQ](faq.md)
