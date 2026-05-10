# Runbook: Windows Code Signing (Azure Trusted Signing)

**Status**: DOCUMENTED — not yet active. Activates at GA.
**Ticket**: #1649
**Budget approved**: 2026-05-10 (Ray Hamilton)
**Azure identity validation**: approved 2026-05-10
**Cost**: $9.99/mo (~$120/yr)

---

## Overview

This runbook covers the Azure Trusted Signing workflow for the VaultMTG daemon
Windows installer (`.exe`). When active, the Windows binary and NSIS installer
are signed before release, eliminating the SmartScreen "Windows protected your
PC" warning for end users.

Per ADR-011: EV certificates are explicitly rejected. Azure Trusted Signing
achieves equivalent SmartScreen reputation at a fraction of the cost ($120/yr
vs $300-600/yr) after Microsoft removed the SmartScreen EV advantage in 2024.

The GA workflow step is documented in `services/daemon/.goreleaser.yml`
(header comments) and in this runbook. It is not yet wired into
`.github/workflows/daemon-release.yml` — that happens at GA.

---

## Prerequisites

- Azure subscription (existing AWS-primary account or a new Azure account)
- Azure Trusted Signing account ($9.99/mo) created in Azure Portal
- Service principal (app registration) in Azure AD with signing permissions
- Identity validation completed (Microsoft reviews the publisher identity)

---

## Step 1: Create Azure Trusted Signing Account

1. Sign in at https://portal.azure.com
2. Search for "Trusted Signing" and select the service
3. Create a new account:
   - Subscription: choose the billing subscription
   - Resource group: `vaultmtg-signing`
   - Account name: `vaultmtg-signing`
   - Region: East US (closest to us-east-1 for minimal latency)
   - SKU: Basic ($9.99/mo)
4. Create a Certificate Profile:
   - Profile name: `vaultmtg-daemon`
   - Profile type: Public Trust

---

## Step 2: Identity Validation

Microsoft requires identity validation before issuing a Trusted Signing
certificate. For an individual/small publisher:

1. In the Trusted Signing account, go to Identity Validation
2. Submit: organization name, address, email, and a government ID scan
3. Wait for approval (typically 1-3 business days)

**Note**: Identity validation was approved 2026-05-10. When the Azure account
is created at GA, the validation process starts from scratch for the new
account. Allow 3-5 business days.

---

## Step 3: Create Service Principal for CI

```bash
# Login to Azure
az login

# Create app registration for CI
az ad app create --display-name "vaultmtg-daemon-signing"

# Get the app ID
APP_ID=$(az ad app list --display-name "vaultmtg-daemon-signing" \
  --query "[0].appId" -o tsv)

# Create service principal
az ad sp create --id "$APP_ID"

# Create client secret (valid 2 years — set a calendar reminder to rotate)
az ad app credential reset --id "$APP_ID" \
  --display-name "github-actions-signing" \
  --years 2 \
  --query "password" -o tsv
# Save the output password immediately

# Get tenant ID
az account show --query "tenantId" -o tsv
```

---

## Step 4: Grant Service Principal Signing Permissions

In Azure Portal:
1. Go to the Trusted Signing account > Access Control (IAM)
2. Add role assignment:
   - Role: Trusted Signing Certificate Profile Signer
   - Assign to: the service principal created above

---

## Step 5: Populate SSM Parameters

```bash
# Tenant ID
aws ssm put-parameter --profile personal --region us-east-1 \
  --name /vaultmtg/prod/azure-tenant-id \
  --value "YOUR_TENANT_ID" \
  --type String --overwrite

# Client ID (app registration)
aws ssm put-parameter --profile personal --region us-east-1 \
  --name /vaultmtg/prod/azure-client-id \
  --value "YOUR_CLIENT_ID" \
  --type String --overwrite

# Client secret
aws ssm put-parameter --profile personal --region us-east-1 \
  --name /vaultmtg/prod/azure-client-secret \
  --value "YOUR_CLIENT_SECRET" \
  --type SecureString --overwrite

# Trusted Signing account name
aws ssm put-parameter --profile personal --region us-east-1 \
  --name /vaultmtg/prod/azure-trusted-signing-account \
  --value "vaultmtg-signing" \
  --type String --overwrite

# Certificate profile name
aws ssm put-parameter --profile personal --region us-east-1 \
  --name /vaultmtg/prod/azure-certificate-profile \
  --value "vaultmtg-daemon" \
  --type String --overwrite
```

---

## Step 6: Populate GitHub Actions Secrets

| GitHub Secret | SSM Path |
|---|---|
| `AZURE_TENANT_ID` | `/vaultmtg/prod/azure-tenant-id` |
| `AZURE_CLIENT_ID` | `/vaultmtg/prod/azure-client-id` |
| `AZURE_CLIENT_SECRET` | `/vaultmtg/prod/azure-client-secret` |
| `AZURE_TRUSTED_SIGNING_ACCOUNT` | `/vaultmtg/prod/azure-trusted-signing-account` |
| `AZURE_CERTIFICATE_PROFILE` | `/vaultmtg/prod/azure-certificate-profile` |

---

## Step 7: Add Signing Step to daemon-release.yml

Add the following step to the `goreleaser` job in
`.github/workflows/daemon-release.yml`, AFTER the `Run GoReleaser` step
and BEFORE the `Upload darwin universal binary` step.

This step is guarded to only run on real tags (not snapshot/dry-run):

```yaml
- name: Sign Windows binary (Azure Trusted Signing)
  if: startsWith(github.ref, 'refs/tags/daemon/v')
  uses: microsoft/trusted-signing-action@v0
  with:
    azure-tenant-id: ${{ secrets.AZURE_TENANT_ID }}
    azure-client-id: ${{ secrets.AZURE_CLIENT_ID }}
    azure-client-secret: ${{ secrets.AZURE_CLIENT_SECRET }}
    endpoint: https://eus.codesigning.azure.net/
    trusted-signing-account-name: ${{ secrets.AZURE_TRUSTED_SIGNING_ACCOUNT }}
    certificate-profile-name: ${{ secrets.AZURE_CERTIFICATE_PROFILE }}
    files-folder: dist/
    files-folder-filter: exe
    file-digest: SHA256
    timestamp-rfc3161: http://timestamp.acs.microsoft.com
    timestamp-digest: SHA256
```

Note: The `endpoint` uses `eus` (East US) matching the Trusted Signing
account region. Update if the account is created in a different region.

After signing, GoReleaser's `extra_files` glob picks up the `.exe` installer
produced by the NSIS hook and uploads it to the GitHub Release. The signing
action modifies the `.exe` in-place before the upload step.

---

## Step 8: Verify End-to-End on a Clean Windows VM

1. Push a `daemon/v*` tag
2. Download `vaultmtg-daemon-setup-<version>.exe` from the GitHub Release
3. On a clean Windows 11 VM (or a new user account):
   - Double-click the installer
   - SmartScreen must NOT show "Windows protected your PC"
   - If SmartScreen appears with "More info": signing is present but
     reputation has not yet accumulated (normal for new certificates;
     resolves after a few hundred downloads)
   - SmartScreen warning must NOT appear for established GA releases

---

## Budget

| Item | Cost |
|---|---|
| Azure Trusted Signing (Basic) | $9.99/mo |
| Apple Developer Program | $99/yr |
| **Total GA signing (annualized)** | **~$219/yr** |

Budget approved by Ray Hamilton on 2026-05-10.

---

## References

- ADR-011: docs/architecture/adr/0011-daemon-distribution-strategy.md
- GoReleaser config signing section: services/daemon/.goreleaser.yml (header comments)
- microsoft/trusted-signing-action: https://github.com/microsoft/trusted-signing-action
- Azure Trusted Signing docs: https://learn.microsoft.com/azure/trusted-signing/
- SmartScreen reputation note: https://learn.microsoft.com/windows/security/threat-protection/microsoft-defender-smartscreen/microsoft-defender-smartscreen-overview
