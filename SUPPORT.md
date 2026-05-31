# VaultMTG Support

## How to report issues

If you encounter a bug or unexpected behavior, please include a diagnostics bundle with your report. This helps the team triage issues much faster.

### Using the Copy Diagnostics feature (recommended)

1. Open the VaultMTG app and navigate to **Settings**.
2. Expand the **Copy Diagnostics** accordion section.
3. Click **Copy Diagnostics**. The button will briefly show "Fetching diagnostics…" while it contacts the local daemon.
4. Once the button returns to its normal label, your clipboard contains a support bundle with:
   - Daemon version, OS, and architecture
   - Daemon uptime and start time
   - Cloud API endpoint
   - The last 200 lines of the daemon log (secrets are scrubbed automatically)
5. Paste the clipboard contents into your bug report.

> **Note:** The daemon must be running for this feature to work. If you see an error, start the VaultMTG daemon and try again.

### Reporting a bug

- **GitHub Issues:** [github.com/RdHamilton/vault-mtg-tickets/issues/new](https://github.com/RdHamilton/vault-mtg-tickets/issues/new)
- **Discord:** Post in the `#support` channel and paste your diagnostics bundle there.

When filing a report, please include:

- Steps to reproduce the issue
- What you expected to happen
- What actually happened
- Your diagnostics bundle (copied via the steps above)
- Any screenshots or screen recordings if applicable

## Security vulnerabilities

Please do **not** file security vulnerabilities as public GitHub issues.
Email `security@vaultmtg.app` instead with a description and reproduction steps.
