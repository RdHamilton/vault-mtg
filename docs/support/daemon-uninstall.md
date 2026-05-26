# How to Uninstall the VaultMTG Daemon

## Quick Answer
To fully remove the VaultMTG daemon, you need to stop it, remove the app, delete the config folder, and remove the startup entry. Your VaultMTG account and match history on vaultmtg.app are not affected by uninstalling — that data lives in the cloud.

---

## Quick uninstall from the app

> **Supported platforms:** macOS and Windows only. On Linux the **Danger Zone — Uninstall Daemon** action returns *"automatic uninstall not supported on this platform"* (HTTP 400) because there is no Linux installer to undo today — Linux users should remove the daemon manually using the steps later in this document.

If the daemon is running and you have the VaultMTG web app open:

1. Sign in to [app.vaultmtg.app](https://app.vaultmtg.app) and open **Settings → Data Recovery**.
2. Scroll to the **Danger Zone — Uninstall Daemon** subsection.
3. Click **Uninstall VaultMTG Daemon**.
4. (Optional) Tick **Also wipe my local config + cached data** to delete the daemon's local config folder.
5. Click **Confirm Uninstall**.

The daemon will:
- Stop itself.
- Remove its launchd plist (macOS) or Task Scheduler entry (Windows) so it does not restart at next login.
- Wipe the local config folder if you ticked the box.
- Exit within ~200 ms.

After that, the binary itself stays on disk. To remove it:
- **macOS:** drag VaultMTG from `/Applications` to the Trash.
- **Windows:** use **Settings → Apps → Apps & features → VaultMTG → Uninstall**.

If the daemon is **not** running, the in-app uninstall button is disabled. Fall back to the manual steps below.

---

## macOS Uninstall

### Step 1 — Quit the daemon

1. Click the VaultMTG icon in the menu bar (top-right of your screen).
2. Choose **Quit VaultMTG Daemon**.
3. Confirm the process has stopped: open Activity Monitor and search for `vaultmtg-daemon`. It should not appear.

### Step 2 — Remove the app from Applications

1. Open **Finder** > **Applications**.
2. Find **VaultMTG Daemon**.
3. Drag it to the **Trash**, or right-click and choose **Move to Trash**.

### Step 3 — Remove the launchd startup item

The daemon registers a launchd plist so it starts at login. Remove it:

1. Press Command + Space, type **Terminal**, press Enter.
2. Run the following two commands one at a time:

```bash
launchctl unload ~/Library/LaunchAgents/app.vaultmtg.daemon.plist
```

```bash
rm ~/Library/LaunchAgents/app.vaultmtg.daemon.plist
```

If the first command returns an error saying the file does not exist, it means the startup item was never created or was already removed — that is fine, continue to the next step.

### Step 4 — Delete the config folder

This removes your auth token, daemon settings, and any locally cached data.

1. In Terminal, run:

```bash
rm -rf ~/.vaultmtg
# If you installed the daemon before the VaultMTG rename (mid-2026) the legacy
# folder may also exist; remove it as well:
rm -rf ~/.mtga-companion
```

If you prefer not to use Terminal:
1. Open **Finder**, click the **Go** menu in the menu bar, choose **Go to Folder**.
2. Paste `~/.vaultmtg` and press Enter. Press Command + Delete to move the folder to the Trash.
3. If you have an older install, repeat with `~/.mtga-companion`.

### Step 5 — Empty the Trash

Right-click the Trash in the Dock and choose **Empty Trash** to permanently delete all removed files.

---

## Windows Uninstall

### Step 1 — Exit the daemon

1. Right-click the VaultMTG icon in the system tray (bottom-right of your screen, near the clock).
2. Choose **Exit**.
3. Confirm the process has stopped: open Task Manager (Ctrl + Shift + Esc) > **Details** tab and check that `vaultmtg-daemon.exe` is no longer listed.

### Step 2 — Uninstall via Windows Settings

1. Click the **Start Menu** and open **Settings** (the gear icon).
2. Go to **Apps** > **Installed apps** (or **Apps and features** on Windows 10).
3. Search for **VaultMTG**.
4. Click the three-dot menu next to **VaultMTG Daemon** and choose **Uninstall**.
5. Follow the uninstaller prompts. Click **Yes** on any User Account Control (UAC) prompt.

### Step 3 — Remove the Scheduled Task or startup entry

The installer adds VaultMTG Daemon to Windows startup. Remove it:

**Option A — Task Manager (simpler):**
1. Open Task Manager (Ctrl + Shift + Esc) > **Startup** tab.
2. Right-click **VaultMTG Daemon** and choose **Disable** or **Delete**.

**Option B — Task Scheduler (if the entry persists):**
1. Press Windows + S, type **Task Scheduler**, press Enter.
2. In the left panel, click **Task Scheduler Library**.
3. Look for a task named **VaultMTG Daemon**.
4. Right-click it and choose **Delete**.

### Step 4 — Delete the config folder

This removes your auth token, daemon settings, and any locally cached data.

1. Press **Windows + R**, paste `%APPDATA%\vaultmtg`, press Enter.
2. A File Explorer window will open. Go up one level (click **vaultmtg**'s parent folder in the address bar).
3. Right-click the `vaultmtg` folder and choose **Delete**.

Alternatively, press Windows + R, paste the following, and press Enter:
```
cmd /c rmdir /s /q "%APPDATA%\vaultmtg"
```
Click **Yes** on any confirmation prompts.

If you installed the daemon before the VaultMTG rename (mid-2026), the legacy `%APPDATA%\mtga-companion` folder may also exist. Remove it the same way:
```
cmd /c rmdir /s /q "%APPDATA%\mtga-companion"
```

### Step 5 — Empty the Recycle Bin (optional)

Right-click the Recycle Bin on your Desktop and choose **Empty Recycle Bin**.

---

## What Is Removed vs. What Is Kept

| Item | Removed by uninstall | Notes |
|---|---|---|
| Daemon binary | Yes | The app itself |
| Config file (`daemon.json`) | Yes — Step 4 | Contains auth token and settings |
| Locally cached data | Yes — Step 4 | Temporary files only |
| Startup entry | Yes — Step 3 | Prevents auto-start after reboot |
| Your VaultMTG account | No | Account and cloud data are not affected |
| Match history in VaultMTG | No | Stored on VaultMTG servers, not your machine |
| MTG Arena | No | VaultMTG does not modify Arena |
| MTG Arena Player.log | No | VaultMTG only reads this file, never modifies it |

---

## Reinstalling After Uninstall

If you want to reinstall in the future, follow the [installation guide](daemon-installation.md). Your VaultMTG account and match history will still be there — just sign in after reinstalling.

---

## If That Doesn't Work

If any step fails or you see unexpected errors, post in Discord [#help](https://discord.gg/vaultmtg) with your operating system and a description of the error.

## Related

- [Daemon Installation](daemon-installation.md)
- [Daemon Troubleshooting](daemon-troubleshooting.md)
- [FAQ](faq.md)
