# Runbook: macOS Notarization (Apple Developer ID)

**Status**: DOCUMENTED — not yet active. Activates at GA.
**Ticket**: #1648
**Budget approved**: 2026-05-10 (Ray Hamilton)
**Cost**: $99/yr (Apple Developer Program)

---

## Overview

This runbook covers the end-to-end Apple Developer ID notarization workflow
for the VaultMTG daemon `.dmg` installer. When active, users install the daemon
with zero Gatekeeper warnings on macOS 14+.

The CI pipeline (`sign-macos` job in `.github/workflows/daemon-release.yml`)
is already implemented and tag-guarded. It runs on every `daemon/v*` release
tag. It is inert until the Apple credentials are populated in GitHub Secrets.

---

## Prerequisites

- Apple Developer Program membership enrolled ($99/yr)
- Developer ID Application certificate issued by Apple
- Developer ID Installer certificate issued by Apple
- App-specific password generated at appleid.apple.com for `notarytool`

---

## Step 1: Enroll in Apple Developer Program

1. Go to https://developer.apple.com/programs/enroll/
2. Sign in with the Apple ID that will own the certificates
3. Complete enrollment ($99 charge)
4. Wait for approval (usually same-day for individuals)

---

## Step 2: Create Certificates in Xcode / Keychain

```
Xcode > Settings > Accounts > Manage Certificates
  + Developer ID Application
  + Developer ID Installer
```

Export each certificate as a `.p12` file with a strong passphrase.
Store the passphrases in a password manager (1Password, Bitwarden, etc.).

---

## Step 3: Generate an App-Specific Password for notarytool

1. Sign in at https://appleid.apple.com
2. Go to App-Specific Passwords > Generate
3. Label: `vaultmtg-notarytool`
4. Save the generated password immediately (shown only once)

---

## Step 4: Populate SSM Parameters

Use the `personal` AWS profile. All secret values use `SecureString` type.

```bash
# Team ID (find in Apple Developer portal under Membership)
aws ssm put-parameter --profile personal --region us-east-1 \
  --name /vaultmtg/prod/apple-team-id \
  --value "XXXXXXXXXX" \
  --type String --overwrite

# Apple ID used for notarytool
aws ssm put-parameter --profile personal --region us-east-1 \
  --name /vaultmtg/prod/apple-notarization-apple-id \
  --value "ray@example.com" \
  --type String --overwrite

# App-specific password
aws ssm put-parameter --profile personal --region us-east-1 \
  --name /vaultmtg/prod/apple-notarization-password \
  --value "xxxx-xxxx-xxxx-xxxx" \
  --type SecureString --overwrite

# Developer ID Application certificate (base64-encoded .p12)
aws ssm put-parameter --profile personal --region us-east-1 \
  --name /vaultmtg/prod/apple-developer-certificate \
  --value "$(base64 -i dev-id-application.p12)" \
  --type SecureString --overwrite

# Developer ID Application certificate passphrase
aws ssm put-parameter --profile personal --region us-east-1 \
  --name /vaultmtg/prod/apple-developer-certificate-password \
  --value "YOUR_P12_PASSPHRASE" \
  --type SecureString --overwrite

# Developer ID Installer certificate (base64-encoded .p12)
aws ssm put-parameter --profile personal --region us-east-1 \
  --name /vaultmtg/prod/apple-installer-certificate \
  --value "$(base64 -i dev-id-installer.p12)" \
  --type SecureString --overwrite

# Developer ID Installer certificate passphrase
aws ssm put-parameter --profile personal --region us-east-1 \
  --name /vaultmtg/prod/apple-installer-certificate-password \
  --value "YOUR_INSTALLER_P12_PASSPHRASE" \
  --type SecureString --overwrite
```

---

## Step 5: Populate GitHub Actions Secrets

Load the SSM values into GitHub Secrets. These map 1:1 to the env vars
read by the `sign-macos` job:

| GitHub Secret | SSM Path |
|---|---|
| `APPLE_TEAM_ID` | `/vaultmtg/prod/apple-team-id` |
| `APPLE_NOTARIZATION_APPLE_ID` | `/vaultmtg/prod/apple-notarization-apple-id` |
| `APPLE_NOTARIZATION_PASSWORD` | `/vaultmtg/prod/apple-notarization-password` |
| `APPLE_DEVELOPER_CERTIFICATE` | `/vaultmtg/prod/apple-developer-certificate` |
| `APPLE_DEVELOPER_CERTIFICATE_PASSWORD` | `/vaultmtg/prod/apple-developer-certificate-password` |
| `APPLE_INSTALLER_CERTIFICATE` | `/vaultmtg/prod/apple-installer-certificate` |
| `APPLE_INSTALLER_CERTIFICATE_PASSWORD` | `/vaultmtg/prod/apple-installer-certificate-password` |

```bash
# Example: set via gh CLI
gh secret set APPLE_TEAM_ID \
  --repo RdHamilton/MTGA-Companion \
  --body "$(aws ssm get-parameter --profile personal --region us-east-1 \
    --name /vaultmtg/prod/apple-team-id --query Parameter.Value --output text)"
```

Repeat for each secret listed above.

---

## Step 6: Verify the sign-macos CI Job End-to-End

1. Push a `daemon/v*` tag to trigger the `daemon-release.yml` workflow:
   ```bash
   git tag daemon/v0.4.0-rc1
   git push origin daemon/v0.4.0-rc1
   ```

2. Monitor the `sign-macos` job in the GitHub Actions UI.
   The job has a 30-minute timeout (notarization queue can take 5-15 min).

3. After the job completes, download the `.dmg` from the GitHub Release page.

4. On a clean macOS 14+ VM (or freshly created user account with no prior
   GKE approval for vaultmtg-daemon):
   ```bash
   # Mount and run
   hdiutil attach vaultmtg-daemon-darwin-universal.dmg
   open /Volumes/MTGA\ Companion\ Daemon/vaultmtg-daemon-darwin-universal.pkg
   ```
   Gatekeeper must NOT warn ("Apple cannot verify..."). If the warning appears,
   notarization or stapling failed — check the `notarytool submit` log output
   in the CI run.

5. Verify stapling independently:
   ```bash
   xcrun stapler validate vaultmtg-daemon-darwin-universal.dmg
   # Expected: The validate action worked!
   ```

---

## Sign-macos Job: What the CI Does

The job is fully implemented in `.github/workflows/daemon-release.yml`.
Steps in order:

1. Download darwin universal binary artifact from the `goreleaser` job
2. Import Developer ID Application certificate into a temporary keychain
3. Import Developer ID Installer certificate into the same keychain
4. `codesign --options runtime` the universal binary
5. `pkgbuild` creates a `.pkg` installer signed with Developer ID Installer
6. `hdiutil create` wraps the `.pkg` in a `.dmg`
7. `xcrun notarytool submit <.dmg> --wait` submits to Apple's notary service
8. `xcrun stapler staple <.dmg>` attaches the notarization ticket
9. `gh release upload` attaches both `.pkg` and `.dmg` to the GitHub Release
10. Temporary keychain is deleted (always, even on failure)

The job is tag-guarded (`if: startsWith(github.ref, 'refs/tags/daemon/v')`)
so it never runs on `workflow_dispatch` without a tag (prevents quota waste).

---

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `security: SecKeychainItemImport` fails | Wrong P12 passphrase in secret | Re-export P12 with correct passphrase; update SSM + GH secret |
| `codesign: error ... no identity found` | Team ID mismatch or cert not imported | Verify `APPLE_TEAM_ID` matches cert CN; re-import |
| `notarytool: ... authentication failure` | Wrong Apple ID or app-specific password | Regenerate app-specific password at appleid.apple.com; update SSM |
| `notarytool: ... invalid` (submission rejected) | Missing `--options runtime` on codesign | Already in CI; verify the codesign step ran without skipping |
| Gatekeeper still warns after staple | Staple succeeded but .dmg was re-created | Never re-wrap .dmg after stapling; the staple is inside the .dmg |
| Notarization times out (>30 min) | Apple notary queue busy | Rare; re-run the sign-macos job manually or wait and retry |

---

## References

- ADR-011: docs/architecture/adr/0011-daemon-distribution-strategy.md
- CI workflow: .github/workflows/daemon-release.yml (sign-macos job)
- GoReleaser config signing docs: services/daemon/.goreleaser.yml (header comments)
- Apple notarytool docs: https://developer.apple.com/documentation/security/notarizing_macos_software_before_distribution
- Apple Developer Program: https://developer.apple.com/programs/
