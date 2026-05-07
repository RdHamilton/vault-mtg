# How to Install the VaultMTG Daemon

## Quick Answer
The VaultMTG daemon is a small background program that reads your MTG Arena game log and sends your match and draft data to VaultMTG. You need to install and run it on the same computer where you play MTG Arena.

---

## Before You Start

- MTG Arena must already be installed on your computer.
- Launch MTG Arena at least once so its log file is created, then close it.
- You will need your VaultMTG beta invite link. If you do not have one, join the waitlist at vaultmtg.app.

---

## macOS Installation

### Step 1 — Download the installer

Open your beta invite email and click the macOS download link. Your browser will download a file named something like `VaultMTG-Daemon-1.x.x.dmg`.

### Step 2 — Open the installer

Double-click the `.dmg` file in your Downloads folder. A window will open showing the VaultMTG Daemon installer.

### Step 3 — Handle the Gatekeeper warning

Because VaultMTG is in beta, the installer is not yet signed with an Apple developer certificate. macOS will show a warning and refuse to open it by default.

**You will see a dialog that says something like:**
> "VaultMTG-Daemon-1.x.x.dmg" cannot be opened because Apple cannot check it for malicious software.

**How to get past this:**

1. Close the warning dialog by clicking OK.
2. Open **Finder** and navigate to your **Downloads** folder.
3. Find the `.dmg` file.
4. **Right-click** (or Control-click) the file and choose **Open** from the menu that appears.
5. A new dialog will appear that includes an **Open** button. Click **Open**.

macOS will remember your choice and you will not need to do this again for this installer.

### Step 4 — Install the daemon

Inside the `.dmg` window, drag the **VaultMTG Daemon** icon into the **Applications** folder shortcut. Wait for the copy to finish, then eject the `.dmg` (drag it to the Trash or right-click and choose Eject).

### Step 5 — Launch the daemon for the first time

1. Open **Finder** > **Applications** and double-click **VaultMTG Daemon**.
2. macOS may show a second Gatekeeper warning for the app itself. Follow the same right-click > Open steps from Step 3 if it does.
3. A small VaultMTG icon will appear in your menu bar (top-right of your screen). This means the daemon is running.

### Step 6 — Sign in

Click the menu bar icon and choose **Sign In**. Enter your VaultMTG account credentials. The status indicator in the menu bar will turn green when the daemon is connected.

### Step 7 — Verify the connection

Open vaultmtg.app in your browser. The health indicator in the top bar of the app should show a green dot. If it shows yellow or red, see the [troubleshooting guide](daemon-troubleshooting.md).

---

## Windows Installation

### Step 1 — Download the installer

Open your beta invite email and click the Windows download link. Your browser will download a file named something like `VaultMTG-Daemon-Setup-1.x.x.exe`.

### Step 2 — Handle the SmartScreen warning

Because VaultMTG is in beta, the installer is not yet signed with a Windows code-signing certificate. Windows SmartScreen will show a warning when you run it.

**You will see a blue screen that says:**
> Windows protected your PC
> Microsoft Defender SmartScreen prevented an unrecognized app from starting.

**How to get past this:**

1. Click **More info** (the small link below the main message).
2. A new button labeled **Run anyway** will appear at the bottom of the screen.
3. Click **Run anyway**.

If you do not see **More info**, your organization's IT policy may be blocking the installer. See the [troubleshooting guide](daemon-troubleshooting.md).

### Step 3 — Run the installer

The setup wizard will open. Follow the on-screen steps:

1. Click **Next** on the welcome screen.
2. Accept the license agreement and click **Next**.
3. Leave the install location at its default (or change it if you prefer) and click **Install**.
4. Click **Finish** when the install is complete.

The installer will create a shortcut on your Desktop and in your Start Menu.

### Step 4 — Launch the daemon

Double-click the **VaultMTG Daemon** shortcut on your Desktop (or find it in the Start Menu). A VaultMTG icon will appear in your system tray (bottom-right of your screen, near the clock). This means the daemon is running.

### Step 5 — Sign in

Right-click the system tray icon and choose **Sign In**. Enter your VaultMTG account credentials. The tray icon will turn green when connected.

### Step 6 — Verify the connection

Open vaultmtg.app in your browser. The health indicator in the top bar should show a green dot. If it shows yellow or red, see the [troubleshooting guide](daemon-troubleshooting.md).

---

## Start on Login (Recommended)

To make sure the daemon starts automatically when you turn on your computer:

**macOS:** Click the menu bar icon > **Preferences** > check **Launch at login**.

**Windows:** The installer adds the daemon to Windows startup by default. To confirm: open Task Manager > **Startup** tab and verify VaultMTG Daemon is set to **Enabled**.

---

## If That Doesn't Work

- See the [daemon troubleshooting guide](daemon-troubleshooting.md) for common problems.
- Post in Discord [#help](https://discord.gg/vaultmtg) with your operating system and version, and a description of what happened.
- Use the chat icon on vaultmtg.app to reach support directly.

## Related

- [Daemon Troubleshooting](daemon-troubleshooting.md)
- [Daemon Uninstall](daemon-uninstall.md)
- [FAQ](faq.md)
