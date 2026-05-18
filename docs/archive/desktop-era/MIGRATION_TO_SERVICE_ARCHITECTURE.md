# Migration to Service-Based Architecture

## Overview

VaultMTG has been upgraded to use a service-based architecture that separates data collection from display. This guide will help you understand the changes and migrate to the new system.

## What Changed

### Previous Architecture (Standalone Mode)

In the old architecture:
- GUI application contained both data collection and display
- Log monitoring only occurred when GUI was running
- Closing the GUI stopped data collection
- All functionality was coupled in a single process

### New Architecture (Service-Based)

The new architecture introduces two modes:

**1. Daemon Mode (Recommended)**
- CLI daemon runs as a background service
- Monitors MTGA logs 24/7, even when GUI is closed
- GUI connects to daemon via WebSocket for real-time updates
- Data collection is independent of GUI
- Auto-starts on system boot

**2. Standalone Mode (Fallback)**
- GUI runs with embedded log poller (legacy behavior)
- Same functionality as before
- Available when daemon is not installed
- Good for development or casual use

## Why We Changed

### Benefits of Service-Based Architecture

✅ **Zero Data Loss** - Daemon runs 24/7, capturing all matches even when GUI is closed

✅ **Better Resource Usage** - Daemon is lightweight (~10-20 MB), GUI only runs when needed

✅ **Auto-Start** - Daemon starts automatically on boot, no manual intervention

✅ **Crash-Resistant** - Service manager automatically restarts daemon if it crashes

✅ **Separation of Concerns** - Data collection and display are independent

✅ **Multiple Clients** - Multiple GUI instances can connect to the same daemon

✅ **Real-Time Updates** - WebSocket connection provides instant data updates

## Migration Steps

### Recommended: Install Daemon Service

This is the recommended setup for regular users who want complete match tracking.

#### macOS/Linux

1. **Install the daemon service:**
   ```bash
   cd /path/to/VaultMTG
   ./vaultmtg service install
   ```

2. **Start the daemon:**
   ```bash
   ./vaultmtg service start
   ```

3. **Verify it's running:**
   ```bash
   ./vaultmtg service status
   ```

   Expected output:
   ```
   Service Status:
     Status: ✓ Running
   ```

4. **Launch the GUI:**
   - Double-click `VaultMTG.app` (macOS)
   - Run `./VaultMTG` (Linux)

   The GUI will automatically connect to the daemon.

#### Windows

1. **Open PowerShell or Command Prompt as Administrator**
   - Right-click PowerShell → "Run as Administrator"

2. **Install the daemon service:**
   ```powershell
   cd C:\Path\To\VaultMTG
   .\vaultmtg.exe service install
   ```

3. **Start the daemon:**
   ```powershell
   .\vaultmtg.exe service start
   ```

4. **Verify it's running:**
   ```powershell
   .\vaultmtg.exe service status
   ```

5. **Launch the GUI:**
   - Double-click `VaultMTG.exe`

   The GUI will automatically connect to the daemon.

### Alternative: Continue Using Standalone Mode

If you prefer not to install the daemon service, you can continue using standalone mode:

1. **Simply launch the GUI normally**
   - The GUI will detect that no daemon is running
   - It will automatically start its embedded log poller
   - Functionality remains the same as before

**Note**: Standalone mode will be maintained for the foreseeable future, so you don't need to migrate if you prefer the old behavior.

## Verification

### Check Daemon Connection Status

After installing the daemon and launching the GUI:

1. **Look at the top navigation bar:**
   - 🟢 Green dot = Connected to daemon
   - 🟡 Yellow dot = Reconnecting
   - ⚪ White dot = Standalone mode

2. **Go to Settings → Daemon Connection:**
   - View detailed connection status
   - Change daemon port if needed
   - Manually reconnect or switch modes

### Test Data Collection

1. **Ensure daemon is running:**
   ```bash
   ./vaultmtg service status
   ```

2. **Play an MTGA match**

3. **Check GUI Match History:**
   - New matches should appear automatically
   - Statistics should update in real-time

4. **Close the GUI and play another match**

5. **Reopen the GUI:**
   - The match played while GUI was closed should be there
   - This confirms daemon collected data while GUI was closed ✅

## Managing the Daemon

### Service Commands

**Check status:**
```bash
./vaultmtg service status
```

**Start daemon:**
```bash
./vaultmtg service start
```

**Stop daemon:**
```bash
./vaultmtg service stop
```

**Restart daemon:**
```bash
./vaultmtg service restart
```

**Uninstall daemon:**
```bash
./vaultmtg service uninstall
```

### View Daemon Logs

**macOS:**
```bash
tail -f ~/Library/Logs/MTGACompanionDaemon.log
```

**Windows:**
- Open Event Viewer
- Navigate to Windows Logs → Application
- Filter for source "MTGACompanionDaemon"

**Linux:**
```bash
journalctl -u MTGACompanionDaemon -f
```

## Troubleshooting

### GUI Shows "Reconnecting" or "Standalone Mode"

**Possible causes:**
- Daemon is not running
- Daemon is running on a different port
- Firewall blocking WebSocket connection

**Solutions:**

1. **Check if daemon is running:**
   ```bash
   ./vaultmtg service status
   ```

2. **If stopped, start it:**
   ```bash
   ./vaultmtg service start
   ```

3. **Check daemon logs for errors:**
   ```bash
   # macOS
   tail -f ~/Library/Logs/MTGACompanionDaemon.log

   # Linux
   journalctl -u MTGACompanionDaemon -n 50
   ```

4. **Verify port configuration:**
   - Default port is 9999
   - Go to Settings → Daemon Connection
   - Ensure port matches daemon configuration

5. **Test connectivity:**
   ```bash
   curl http://localhost:9999/status
   ```

6. **Check firewall:**
   - Ensure port 9999 is not blocked by firewall

### Daemon Won't Start

**Check logs for errors:**

**macOS:**
```bash
cat ~/Library/Logs/MTGACompanionDaemon.log
```

**Linux:**
```bash
journalctl -u MTGACompanionDaemon -n 50
```

**Common issues:**

**Port already in use:**
```
Error: listen tcp :9999: bind: address already in use
```
Solution: Either stop the other process using port 9999, or configure daemon to use a different port.

**Permission denied:**
```
Error: Permission denied
```
Solution: Run installation with administrator/sudo privileges.

**Binary not found:**
```
Error: exec: "vaultmtg": executable file not found
```
Solution: Ensure binary exists at installation path.

### Database Lock Errors

If you see "database is locked" errors:

1. **Stop all instances:**
   ```bash
   ./vaultmtg service stop
   killall vaultmtg  # macOS/Linux
   ```

2. **Restart daemon:**
   ```bash
   ./vaultmtg service start
   ```

3. **Important**: Don't run both daemon and standalone mode simultaneously
   - Choose one mode or the other
   - Database can only be accessed by one process at a time

### Daemon Doesn't Auto-Start on Boot

**macOS:**
```bash
# Check if Launch Agent is loaded
launchctl list | grep MTGACompanionDaemon

# If not loaded:
launchctl load ~/Library/LaunchAgents/MTGACompanionDaemon.plist
```

**Windows:**
```powershell
# Check service startup type
sc.exe qc MTGACompanionDaemon

# Set to automatic:
sc.exe config MTGACompanionDaemon start= auto
```

**Linux:**
```bash
# Enable auto-start
sudo systemctl enable MTGACompanionDaemon

# Verify
sudo systemctl is-enabled MTGACompanionDaemon
```

### Reverting to Standalone Mode

If you prefer to remove the daemon and use standalone mode:

1. **Stop and uninstall daemon:**
   ```bash
   ./vaultmtg service stop
   ./vaultmtg service uninstall
   ```

2. **Launch GUI normally:**
   - GUI will automatically detect daemon is not available
   - It will fall back to standalone mode with embedded poller

## FAQ

### Do I need to migrate to daemon mode?

**No, it's optional.** The GUI still supports standalone mode (embedded poller) as a fallback. However, daemon mode is recommended for users who want complete match tracking without keeping the GUI open.

### Will my existing data be lost?

**No.** Both modes use the same database (`~/.vaultmtg/data.db`). Your existing match history, statistics, and settings are preserved.

### Can I switch between modes?

**Yes, anytime.** You can:
- Install/uninstall the daemon service at any time
- Switch between modes in GUI Settings → Daemon Connection
- GUI automatically falls back to standalone if daemon is unavailable

### Does daemon mode use more resources?

**Actually, less.** The daemon is a lightweight background service (~10-20 MB RAM) compared to the full GUI (~50-100 MB with WebView). You only run the GUI when you want to view stats.

### What happens if the daemon crashes?

The service manager (launchd/systemd/Windows Service) automatically restarts the daemon if it crashes.

### Can I run multiple GUIs connected to the same daemon?

**Yes.** Multiple GUI instances can connect to the same daemon simultaneously. This is useful for:
- Multiple monitors
- Sharing data with friends (if daemon is network-accessible)
- Development and testing

### How do I configure a custom port?

**Option 1 - Change daemon port:**
```bash
./vaultmtg daemon --port=8888
```

Then update GUI Settings → Daemon Connection → Port to match.

**Option 2 - Configure in service:**
Edit the service configuration to include the port flag:
- macOS: `~/Library/LaunchAgents/MTGACompanionDaemon.plist`
- Windows: Modify service with `sc.exe config`
- Linux: `/etc/systemd/system/MTGACompanionDaemon.service`

### Will standalone mode be deprecated?

**Not in the near future.** Standalone mode is maintained as a fallback and for development purposes. It will continue to work for the foreseeable future.

### How do I uninstall everything?

```bash
# Stop and uninstall daemon
./vaultmtg service stop
./vaultmtg service uninstall

# Remove binary
rm ./vaultmtg  # macOS/Linux
del vaultmtg.exe  # Windows

# Optional: Remove database and config
rm -rf ~/.vaultmtg
```

## Getting Help

If you encounter issues not covered in this guide:

1. Check [DAEMON_INSTALLATION.md](DAEMON_INSTALLATION.md) for detailed installation instructions
2. Check [TROUBLESHOOTING.md](../README.md#troubleshooting) for common issues
3. View daemon logs for error messages
4. Open an issue on GitHub: https://github.com/RdHamilton/vault-mtg/issues

Include:
- OS and version
- Daemon logs
- Service status output
- Steps to reproduce

## Next Steps

After migrating:

1. ✅ Daemon installed and running (or using standalone mode)
2. ✅ GUI connected and displaying data
3. ✅ Play MTGA and verify matches are tracked
4. ✅ Explore new features in Settings → Daemon Connection

For more information:
- [ARCHITECTURE.md](ARCHITECTURE.md) - Understand the system design
- [DAEMON_API.md](DAEMON_API.md) - WebSocket API reference
- [DEVELOPMENT.md](DEVELOPMENT.md) - Developer guide
